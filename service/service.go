package service

import (
	"encoding/json"
	"os"

	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/db"
	"github.com/ontio/crossChainClient/log"
	asdk "github.com/ontio/multi-chain-go-sdk"
	"github.com/ontio/multi-chain/common"
	vconfig "github.com/ontio/multi-chain/consensus/vbft/config"
	autils "github.com/ontio/multi-chain/native/service/utils"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

type SyncService struct {
	aliaAccount    *asdk.Account
	aliaSdk        *asdk.MultiChainSdk
	aliaSyncHeight uint32
	sideAccount    *sdk.Account
	sideSdk        *sdk.OntologySdk
	sideSyncHeight uint32
	db             *db.WaitingDB
	config         *config.Config
}

func NewSyncService(aliaAccount *asdk.Account, sideAccount *sdk.Account, aliaSdk *asdk.MultiChainSdk, sideSdk *sdk.OntologySdk) *SyncService {
	boltDB, err := db.NewWaitingDB("boltdb")
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
	go this.ProcessToAllianceWaiting()
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
		for i := this.sideSyncHeight; i < currentAliaChainHeight; i++ {
			log.Infof("[AllianceToSide] start parse block %d", i)
			//sync key header
			block, err := this.aliaSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[AllianceToSide] this.aliaSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[AllianceToSide] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToSide(i)
				if err != nil {
					log.Errorf("[AllianceToSide] this.syncHeaderToSide error:%s", err)
				}
			}

			//sync cross chain info
			events, err := this.aliaSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[AllianceToSide] this.aliaSdk.GetSmartContractEventByBlock error:%s", err)
				break
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
							log.Errorf("[AllianceToSide] this.syncHeaderToSide error:%s", err)
						}
						err := this.syncProofToSide(key, i)
						if err != nil {
							log.Errorf("[AllianceToSide] this.syncProofToSide error:%s", err)
						}
					}
				}
			}
			this.sideSyncHeight++
		}
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
		for i := this.aliaSyncHeight; i < currentSideChainHeight; i++ {
			log.Infof("[SideToAlliance] start parse block %d", i)
			//sync key header
			block, err := this.sideSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[SideToAlliance] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[SideToAlliance] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToAlia(i)
				if err != nil {
					log.Errorf("[SideToAlliance] this.syncHeaderToMain error:%s", err)
				}
			}

			//sync cross chain info
			events, err := this.sideSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[SideToAlliance] this.sideSdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				txHash, err := common.Uint256FromHexString(event.TxHash)
				if err != nil {
					log.Errorf("[SideToAlliance] common.Uint256FromHexString error:%s", err)
					break
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
							log.Errorf("[SideToAlliance] this.syncHeaderToAlia error:%s", err)
						}
						err := this.syncProofToAlia(txHash[:], key, i)
						if err != nil {
							log.Errorf("[SideToAlliance] this.syncProofToAlia error:%s", err)
						}
					}
				}
			}
			this.aliaSyncHeight++
		}
	}
}

func (this *SyncService) ProcessToAllianceWaiting() {
	for {
		currentAliaChainHeight, err := this.aliaSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[ProcessToAllianceWaiting] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		if currentAliaChainHeight%10 == 0 {
			waitingList, err := this.db.GetWaitingAndDelete(currentAliaChainHeight)
			if err != nil {
				log.Errorf("[ProcessToAllianceWaiting] this.db.GetWaitingAndDelete error:%s", err)
			}
			for _, waiting := range waitingList {
				ok, err := this.retrySyncProofToAlia(waiting.TxHash, waiting.Key, waiting.Height)
				if err != nil {
					log.Errorf("[ProcessToAllianceWaiting] this.retrySyncProofToAlia error:%s", err)
				}
				sink := common.NewZeroCopySink(nil)
				waiting.Serialization(sink)
				if ok {
					err := this.db.Delete(sink.Bytes())
					if err != nil {
						log.Errorf("[ProcessToAllianceWaiting] this.db.Delete error:%s", err)
					}
				}
			}
		}
	}
}
