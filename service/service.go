package service

import (
	"os"

	"encoding/json"
	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/consensus/vbft/config"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
	"sync"
)

type SyncService struct {
	sync.RWMutex
	account      *sdk.Account
	mainSdk      *sdk.OntologySdk
	syncHeight   uint32
	sideChainMap map[uint64]*SideChain
	config       *config.Config
}

type SideChain struct {
	sdk        *sdk.OntologySdk
	syncHeight uint32
}

func NewSyncService(acct *sdk.Account, mainSdk *sdk.OntologySdk, config *config.Config) *SyncService {
	syncSvr := &SyncService{
		account: acct,
		mainSdk: mainSdk,
		config:  config,
	}
	sideChainMap := make(map[uint64]*SideChain)
	for rpcAddress, chainID := range config.SideChainMap {
		sdk := sdk.NewOntologySdk()
		sdk.NewRpcClient().SetAddress(rpcAddress)
		sideChainMap[chainID] = &SideChain{
			sdk: sdk,
		}
	}
	syncSvr.sideChainMap = sideChainMap
	return syncSvr
}

func (this *SyncService) Run() {
	go this.MainMonitor()
	for chainID := range this.sideChainMap {
		go this.SideMonitor(chainID)
	}
}

func (this *SyncService) MainMonitor() {
	mainChainHeight, err := this.mainSdk.GetCurrentBlockHeight()
	if err != nil {
		log.Errorf("[MainMonitor] this.mainSdk.GetCurrentBlockHeight error:", err)
	}
	this.syncHeight = mainChainHeight

	for {
		currentMainChainHeight, err := this.mainSdk.GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[MainMonitor] this.mainSdk.GetCurrentBlockHeight error:", err)
		}
		for i := this.syncHeight; i < currentMainChainHeight; i++ {
			log.Infof("[MainMonitor] start parse block %d", i)
			//sync key header
			block, err := this.mainSdk.GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[MainMonitor] this.mainSdk.GetBlockByHeight error:", err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[MainMonitor] unmarshal blockInfo error: %s", err)
			}
			if blkInfo.NewChainConfig != nil {
				this.syncMainHeader(block)
			}

			//sync cross chain info
			events, err := this.mainSdk.GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[MainMonitor] this.mainSdk.GetSmartContractEventByBlock error:%s", err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states, ok := notify.States.([]interface{})
					if !ok {
						continue
					}
					name := states[0].(string)
					if name == cross_chain.CREATE_CROSS_CHAIN_TX {
						toChainID := uint64(states[1].(float64))
						requestID := uint64(states[2].(float64))
						block, err := this.mainSdk.GetBlockByHeight(i + 1)
						if err != nil {
							log.Errorf("[MainMonitor] this.mainSdk.GetBlockByHeight error:%s", err)
						}
						this.syncHeaderToSide(toChainID, block.Header)
						err = this.sendMainProofToSide(toChainID, requestID, i)
						if err != nil {
							log.Errorf("[MainMonitor] this.sendProofToSide error:%s", err)
						}
					}
				}
			}
			this.syncHeight++
		}
	}
}

func (this *SyncService) SideMonitor(chainID uint64) {
	sideChainHeight, err := this.getSideSdk(chainID).GetCurrentBlockHeight()
	if err != nil {
		log.Errorf("[SideMonitor] side chain %d, this.GetCurrentMainChainSyncHeight error:%s", chainID, err)
		os.Exit(1)
	}
	this.Lock()
	this.sideChainMap[chainID].syncHeight = sideChainHeight
	this.Unlock()
	for {
		currentSideChainHeight, err := this.getSideSdk(chainID).GetCurrentBlockHeight()
		if err != nil {
			log.Errorf("[SideMonitor] side chain %d, this.sideSdk.GetCurrentBlockHeight error:", chainID, err)
		}
		for i := this.getSideSyncHeight(chainID); i < currentSideChainHeight; i++ {
			log.Infof("[SideMonitor] side chain %d, start parse block %d", i)
			//sync key header
			block, err := this.getSideSdk(chainID).GetBlockByHeight(i)
			if err != nil {
				log.Errorf("[SideMonitor] side chain %d, this.getSideSdk(chainID).GetBlockByHeight error:", chainID, err)
			}
			blkInfo := &vconfig.VbftBlockInfo{}
			if err := json.Unmarshal(block.Header.ConsensusPayload, blkInfo); err != nil {
				log.Errorf("[SideMonitor] side chain %d, unmarshal blockInfo error: %s", chainID, err)
			}
			if blkInfo.NewChainConfig != nil {
				this.syncHeaderToMain(chainID, block.Header)
			}

			//sync cross chain info
			events, err := this.getSideSdk(chainID).GetSmartContractEventByBlock(i)
			if err != nil {
				log.Errorf("[SideMonitor] side chain %d, this.sideSdk.GetSmartContractEventByBlock error:%s", chainID, err)
				break
			}
			for _, event := range events {
				for _, notify := range event.Notify {
					states, ok := notify.States.([]interface{})
					if !ok {
						continue
					}
					name := states[0].(string)
					if name == cross_chain.CREATE_CROSS_CHAIN_TX {
						toChainID := uint64(states[1].(float64))
						requestID := uint64(states[2].(float64))
						block, err := this.getSideSdk(chainID).GetBlockByHeight(i + 1)
						if err != nil {
							log.Errorf("[SideMonitor] side chain %d, this.getSideSdk(chainID).GetBlockByHeight error:%s",
								chainID, err)
						}
						if toChainID == 0 {
							this.syncHeaderToMain(toChainID, block.Header)
							err = this.sendSideProofToMain(toChainID, requestID, i)
							if err != nil {
								log.Errorf("[SideMonitor] side chain %d, this.sendProofToMain error:%s", chainID, err)
							}
						} else {
							//TODO
							this.syncHeaderToSide(toChainID, block.Header)
							err = this.sendSideProofToSide(chainID, toChainID, requestID, i)
							if err != nil {
								log.Errorf("[SideMonitor] side chain %d to side chain %d, this.sendSideProofToSide error:%s",
									chainID, toChainID, err)
							}
						}
					}
				}
			}
			this.Lock()
			this.sideChainMap[chainID].syncHeight++
			this.Unlock()
		}
	}
}
