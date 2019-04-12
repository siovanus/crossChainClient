package service

import (
	"os"

	"encoding/json"
	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/consensus/vbft/config"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
)

type SyncService struct {
	account        *sdk.Account
	mainSdk        *sdk.OntologySdk
	mainSyncHeight uint32
	sideSdk        *sdk.OntologySdk
	sideSyncHeight uint32
	config         *config.Config
}

func NewSyncService(acct *sdk.Account, mainSdk *sdk.OntologySdk, sideSdk *sdk.OntologySdk) *SyncService {
	syncSvr := &SyncService{
		account: acct,
		mainSdk: mainSdk,
		sideSdk: sideSdk,
		config:  config.DefConfig,
	}
	return syncSvr
}

func (this *SyncService) Run() {
	go this.MainToSide()
	go this.SideToMain()
}

func (this *SyncService) MainToSide() {
	currentSideChainSyncHeight, err := this.GetCurrentSideChainSyncHeight(this.GetMainChainID())
	if err != nil {
		log.Errorf("[MainToSide] this.GetCurrentSideChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.sideSyncHeight = currentSideChainSyncHeight
	for {
		currentMainChainHeight, err := this.mainSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[MainToSide] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.sideSyncHeight; i < currentMainChainHeight; i++ {
			log.Infof("[MainToSide] start parse block %d", i)
			//sync key header
			block, err := this.mainSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[MainToSide] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[MainToSide] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToSide(i)
				if err != nil {
					log.Errorf("[MainToSide] this.syncHeaderToSide error:%s", err)
				}
			}

			//sync cross chain info
			events, err := this.mainSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[MainToSide] this.mainSdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states := notify.States.([]interface{})
					name := states[0].(string)
					if name == cross_chain.CREATE_CROSS_CHAIN_TX {
						requestID := uint64(states[2].(float64))
						err = this.syncHeaderToSide(i + 1)
						if err != nil {
							log.Errorf("[MainToSide] this.syncHeaderToSide error:%s", err)
						}
						err = this.sendProofToSide(requestID, i)
						if err != nil {
							log.Errorf("[MainToSide] this.sendProofToSide error:%s", err)
						}
					}
				}
			}
			this.sideSyncHeight++
		}
	}
}

func (this *SyncService) SideToMain() {
	currentMainChainSyncHeight, err := this.GetCurrentMainChainSyncHeight(this.GetSideChainID())
	if err != nil {
		log.Errorf("[SideToMain] this.GetCurrentMainChainSyncHeight error:", err)
		os.Exit(1)
	}
	this.mainSyncHeight = currentMainChainSyncHeight
	for {
		currentSideChainHeight, err := this.sideSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[SideToMain] this.sideSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.mainSyncHeight; i < currentSideChainHeight; i++ {
			log.Infof("[SideToMain] start parse block %d", i)
			//sync key header
			block, err := this.sideSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[SideToMain] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[SideToMain] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				err = this.syncHeaderToMain(i)
				if err != nil {
					log.Errorf("[SideToMain] this.syncHeaderToMain error:%s", err)
				}
			}

			//sync cross chain info
			events, err := this.sideSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[SideToMain] this.sideSdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states := notify.States.([]interface{})
					name := states[0].(string)
					if name == cross_chain.CREATE_CROSS_CHAIN_TX {
						requestID := uint64(states[2].(float64))
						err = this.syncHeaderToMain(i + 1)
						if err != nil {
							log.Errorf("[SideToMain] this.syncHeaderToMain error:%s", err)
						}
						err = this.sendProofToMain(requestID, i)
						if err != nil {
							log.Errorf("[SideToMain] this.sendProofToMain error:%s", err)
						}
					}
				}
			}
			this.mainSyncHeight++
		}
	}

}
