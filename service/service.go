package service

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/db"
	"github.com/ontio/crossChainClient/log"
	asdk "github.com/ontio/multi-chain-go-sdk"
	vconfig "github.com/ontio/multi-chain/consensus/vbft/config"
	autils "github.com/ontio/multi-chain/native/service/utils"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/common"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

type SyncService struct {
	aliaAccount    *asdk.Account
	aliaSdk        *asdk.MultiChainSdk
	aliaSyncHeight uint32
	sideAccount    *sdk.Account
	sideSdk        *sdk.OntologySdk
	sideSyncHeight uint32
	db             *db.BoltDB
	config         *config.Config
}

func NewSyncService(aliaAccount *asdk.Account, sideAccount *sdk.Account, aliaSdk *asdk.MultiChainSdk, sideSdk *sdk.OntologySdk) *SyncService {
	boltDB, err := db.NewBoltDB("boltdb")
	if err != nil {
		log.Errorf("db.NewWaitingDB error:%s", err)
		os.Exit(1)
	}
	syncSvr := &SyncService{
		aliaAccount: aliaAccount,
		aliaSdk:     aliaSdk,
		sideAccount: sideAccount,
		sideSdk:     sideSdk,
		db:          boltDB,
		config:      config.DefConfig,
	}
	return syncSvr
}

func (this *SyncService) Run() {
	go this.SideToAlliance()
	go this.AllianceToSide()
	go this.ProcessToAllianceCheckAndRetry()
}

func (this *SyncService) AllianceToSide() {
	currentSideChainSyncHeight, err := this.GetCurrentSideChainSyncHeight(this.GetAliaChainID())
	if err != nil {
		log.Errorf("[AllianceToSide] this.GetCurrentSideChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.sideSyncHeight = currentSideChainSyncHeight
	for {
		currentAliaChainHeight, err := this.aliaSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[AllianceToSide] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		err = this.allianceToSide(this.sideSyncHeight, currentAliaChainHeight)
		if err != nil {
			log.Errorf("[AllianceToSide] this.allianceToSide error:", err)
		}
		time.Sleep(time.Duration(this.config.ScanInterval) * time.Second)
	}
}

func (this *SyncService) SideToAlliance() {
	currentAliaChainSyncHeight, err := this.GetCurrentAliaChainSyncHeight(this.GetSideChainID())
	if err != nil {
		log.Errorf("[SideToAlliance] this.GetCurrentAliaChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.aliaSyncHeight = currentAliaChainSyncHeight
	for {
		currentSideChainHeight, err := this.sideSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[SideToAlliance] this.sideSdk.GetCurrentBlockHeight error:", err)
		}
		err = this.sideToAlliance(this.aliaSyncHeight, currentSideChainHeight)
		if err != nil {
			log.Errorf("[SideToAlliance] this.sideToAlliance error:", err)
		}
		time.Sleep(time.Duration(this.config.ScanInterval) * time.Second)
	}
}

func (this *SyncService) ProcessToAllianceCheckAndRetry() {
	for {
		err := this.checkDoneTx()
		if err != nil {
			log.Errorf("[ProcessToAllianceCheckAndRetry] this.checkDoneTx error:%s", err)
		}
		err = this.retryTx()
		if err != nil {
			log.Errorf("[ProcessToAllianceCheckAndRetry] this.retryTx error:%s", err)
		}
		time.Sleep(time.Duration(this.config.ScanInterval) * time.Second)
	}
}

func (this *SyncService) allianceToSide(m, n uint32) error {
	for i := m; i < n; i++ {
		log.Infof("[allianceToSide] start parse block %d", i)
		//sync key header
		block, err := this.aliaSdk.GetBlockByHeight(i)
		if err != nil {
			return fmt.Errorf("[allianceToSide] this.aliaSdk.GetBlockByHeight error: %s", err)
		}
		blkInfo := &vconfig.VbftBlockInfo{}
		if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
			return fmt.Errorf("[allianceToSide] unmarshal blockInfo error: %s", err)
		}
		if blkInfo.NewChainConfig != nil {
			err = this.syncHeaderToSide(i)
			if err != nil {
				return fmt.Errorf("[allianceToSide] this.syncHeaderToSide error:%s", err)
			}
		}

		//sync cross chain info
		events, err := this.aliaSdk.GetSmartContractEventByBlock(i)
		if err != nil {
			return fmt.Errorf("[allianceToSide] this.aliaSdk.GetSmartContractEventByBlock error:%s", err)
		}
		for _, event := range events {
			for _, notify := range event.Notify {
				states, ok := notify.States.([]interface{})
				if !ok {
					continue
				}
				if notify.ContractAddress != autils.CrossChainManagerContractAddress.ToHexString() {
					continue
				}
				name := states[0].(string)
				if name == "makeToOntProof" {
					key := states[4].(string)
					err = this.syncHeaderToSide(i + 1)
					if err != nil {
						return fmt.Errorf("[allianceToSide] this.syncHeaderToSide error:%s", err)
					}
					err := this.syncProofToSide(key, i)
					if err != nil {
						return fmt.Errorf("[allianceToSide] this.syncProofToSide error:%s", err)
					}
				}
			}
		}
		this.sideSyncHeight++
	}
	return nil
}

func (this *SyncService) sideToAlliance(m, n uint32) error {
	for i := m; i < n; i++ {
		log.Infof("[sideToAlliance] start parse block %d", i)
		//sync key header
		block, err := this.sideSdk.GetBlockByHeight(i)
		if err != nil {
			return fmt.Errorf("[sideToAlliance] this.mainSdk.GetBlockByHeight error:", err)
		}
		blkInfo := &vconfig.VbftBlockInfo{}
		if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
			return fmt.Errorf("[sideToAlliance] unmarshal blockInfo error: %s", err)
		}
		if blkInfo.NewChainConfig != nil {
			err = this.syncHeaderToAlia(i)
			if err != nil {
				return fmt.Errorf("[sideToAlliance] this.syncHeaderToMain error:%s", err)
			}
		}

		//sync cross chain info
		events, err := this.sideSdk.GetSmartContractEventByBlock(i)
		if err != nil {
			return fmt.Errorf("[sideToAlliance] this.sideSdk.GetSmartContractEventByBlock error:%s", err)
		}
		for _, event := range events {
			txHash, err := common.Uint256FromHexString(event.TxHash)
			if err != nil {
				return fmt.Errorf("[sideToAlliance] common.Uint256FromHexString error:%s", err)
			}
			for _, notify := range event.Notify {
				states, ok := notify.States.([]interface{})
				if !ok {
					continue
				}
				if notify.ContractAddress != utils.CrossChainContractAddress.ToHexString() {
					continue
				}
				name := states[0].(string)
				if name == "makeFromOntProof" {
					key := states[4].(string)
					err = this.syncHeaderToAlia(i + 1)
					if err != nil {
						return fmt.Errorf("[sideToAlliance] this.syncHeaderToAlia error:%s", err)
					}
					err := this.syncProofToAlia(txHash[:], key, i)
					if err != nil {
						return fmt.Errorf("[sideToAlliance] this.syncProofToAlia error:%s", err)
					}
				}
			}
		}
		this.aliaSyncHeight++
	}
	return nil
}
