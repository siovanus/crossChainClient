package service

import (
	"os"

	"encoding/json"
	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	vconfig "github.com/ontio/multi-chain/consensus/vbft/config"
	"github.com/ontio/multi-chain/smartcontract/service/native/cross_chain_manager/ont"
	sdk "github.com/ontio/ontology-go-sdk"
)

type SyncService struct {
	account        *sdk.Account
	aliaSdk        *sdk.OntologySdk
	aliaSyncHeight uint32
	sideSdk        *sdk.OntologySdk
	sideSyncHeight uint32
	config         *config.Config
}

func NewSyncService(acct *sdk.Account, aliaSdk *sdk.OntologySdk, sideSdk *sdk.OntologySdk) *SyncService {
	syncSvr := &SyncService{
		account: acct,
		aliaSdk: aliaSdk,
		sideSdk: sideSdk,
		config:  config.DefConfig,
	}
	return syncSvr
}

func (this *SyncService) Run() {
	go this.SideToAlliance()
	go this.AllianceToSide()
}

func (this *SyncService) AllianceToSide() {
	currentSideChainSyncHeight, err := this.GetCurrentSideChainSyncHeight(this.GetAliaChainID())
	if err != nil {
		log.Errorf("[AllianceToSide] this.GetCurrentSideChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.sideSyncHeight = currentSideChainSyncHeight
	for {
		currentMainChainHeight, err := this.aliaSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[AllianceToSide] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.sideSyncHeight; i < currentMainChainHeight; i++ {
			log.Infof("[AllianceToSide] start parse block %d", i)
			//sync key header
			block, err := this.aliaSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[AllianceToSide] this.mainSdk.GetBlockByHeight error:", err)
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
					name := states[0].(string)
					if name == ont.MAKE_TO_ONT_PROOF {
						err = this.syncHeaderToSide(i + 1)
						if err != nil {
							log.Errorf("[AllianceToSide] this.syncHeaderToSide error:%s", err)
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
		log.Errorf("[SideToAlliance] this.GetCurrentMainChainSyncHeight error:", err)
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
				for _, notify := range event.Notify {
					states, ok := notify.States.([]interface{})
					if !ok {
						continue
					}
					name := states[0].(string)
					if name == ont.MAKE_FROM_ONT_PROOF {
						err = this.syncHeaderToAlia(i + 1)
						if err != nil {
							log.Errorf("[SideToAlliance] this.syncHeaderToMain error:%s", err)
						}
					}
				}
			}
			this.aliaSyncHeight++
		}
	}

}
