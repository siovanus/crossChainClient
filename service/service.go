package service

import (
	"time"

	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/smartcontract/service/native/side_chain"
)

type SyncService struct {
	account *sdk.Account
	mainSdk *sdk.OntologySdk
	sideSdk *sdk.OntologySdk
	config  *config.Config
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
	for {
		//get current block header height of main chain
		mainHeight, err := this.getMainCurrentHeaderHeight(this.config.SideChainID)
		if err != nil {
			log.Errorf("this.getMainCurrentHeaderHeight error: %s", err)
		}

		//get current block header height of side chain
		sideHeight, err := this.sideSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("this.sideSdk.GetCurrentBlockHeight error: %s", err)
		}

		if mainHeight <= sideHeight {
			log.Infof("main chain height is %s and side chain height is %s, begin to sync header", mainHeight, sideHeight)
			//get header from side chain
			param := new(side_chain.SyncBlockHeaderParam)
			for i := mainHeight + 1; i <= sideHeight; i++ {
				log.Infof("fetching block %s", i)
				block, err := this.sideSdk.GetBlockByHeight(i)
				if err != nil {
					log.Errorf("this.sideSdk.GetBlockByHeight error: %s", err)
				}
				header := block.Header.ToArray()
				param.Headers = append(param.Headers, header)
			}
			//sync block header
			err = this.syncBlockHeaderToMain(param)
			if err != nil {
				log.Errorf("syncBlockHeaderToMain error: %s", err)
			}
		}
		time.Sleep(time.Duration(this.config.Interval) * time.Second)
	}
}
