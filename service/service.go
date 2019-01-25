package service

import (
	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/smartcontract/service/native/side_chain"
	"hash/fnv"
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
		hash := fnv.New32a()
		hash.Write([]byte(this.config.SideChainID))
		mainHeight, err := this.getMainCurrentHeaderHeight(hash.Sum32())
		if err != nil {
			log.Errorf("this.getMainCurrentHeaderHeight error: %s", err)
			return
		}

		//get current block header height of side chain
		sideHeight, err := this.sideSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("this.sideSdk.GetCurrentBlockHeight error: %s", err)
		}

		gap := sideHeight - mainHeight
		if gap > 0 {
			log.Infof("main chain height is %d and side chain height is %d, begin to sync header", mainHeight, sideHeight)
			//get header from side chain
			param := new(side_chain.SyncBlockHeaderParam)
			for i := mainHeight + 1; i <= sideHeight; i++ {
				log.Infof("fetching block %d", i)
				block, err := this.sideSdk.GetSideChainBlockByHeight(i)
				if err != nil {
					log.Errorf("this.sideSdk.GetSideChainBlockByHeight error: %s", err)
				}
				header := block.Header.ToArray()
				param.Headers = append(param.Headers, header)
			}
			//sync block header
			err = this.syncBlockHeaderToMain(param)
			if err != nil {
				log.Errorf("syncBlockHeaderToMain error: %s", err)
			}
			this.waitForMainBlock()
		}
		this.waitForSideBlock()
	}
}
