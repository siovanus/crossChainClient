package service

import (
	"time"

	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/smartcontract/service/native/header_sync"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

var codeVersion = byte(0)

func (this *SyncService) GetMainChain() uint64 {
	return this.config.MainChainID
}

func (this *SyncService) GetGasPrice() uint64 {
	return this.config.GasPrice
}

func (this *SyncService) GetGasLimit() uint64 {
	return this.config.GasLimit
}

func (this *SyncService) getSideSdk(chainID uint64) *sdk.OntologySdk {
	this.Lock()
	defer this.Unlock()
	return this.sideChainMap[chainID].sdk
}

func (this *SyncService) getSideSyncHeight(chainID uint64) uint32 {
	this.Lock()
	defer this.Unlock()
	return this.sideChainMap[chainID].syncHeight
}

func (this *SyncService) getSideChainMap() map[uint64]*SideChain {
	this.Lock()
	defer this.Unlock()
	return this.sideChainMap
}

func (this *SyncService) waitForMainHeaderSync(chainID uint64, heightBytes []byte) {
	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		log.Errorf("[waitForMainHeaderSync] side chain %d header to main chain, utils.GetUint64Bytes error: %s", chainID, err)
		return
	}
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		v, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
			common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
		if err != nil {
			log.Errorf("[waitForMainHeaderSync] side chain %d header to main chain, sdk.GetStorage error: %s", chainID, err)
			return
		}
		if len(v) != 0 {
			return
		}
	}
	log.Errorf("[waitForMainHeaderSync] main chain 60s timeout")
}

func (this *SyncService) waitForSideHeaderSync(fromChain, toChainID uint64, heightBytes []byte) {
	chainIDBytes, err := utils.GetUint64Bytes(fromChain)
	if err != nil {
		log.Errorf("[waitForSideHeaderSync] side chain %d header to side chain %d, utils.GetUint64Bytes error: %s", fromChain, toChainID, err)
		return
	}
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		v, err := this.getSideSdk(toChainID).GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
			common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
		if err != nil {
			log.Errorf("[waitForSideHeaderSync] side chain %d header to side chain %d, sdk.GetStorage error: %s", fromChain, toChainID, err)
			return
		}
		if len(v) != 0 {
			return
		}
	}
	log.Errorf("[waitForSideHeaderSync] side chain %d header to side chain %d, 60s timeout", fromChain, toChainID)
}

func (this *SyncService) waitForSideConsensusPeersSync(fromChain, toChainID uint64, height uint32) {
	chainIDBytes, err := utils.GetUint64Bytes(fromChain)
	if err != nil {
		log.Errorf("[waitForSideConsensusPeersSync] side chain %d consensus peers to side chain %d, utils.GetUint64Bytes error: %s",
			fromChain, toChainID, err)
		return
	}
	heightBytes, err := utils.GetUint32Bytes(height)
	if err != nil {
		log.Errorf("[waitForSideConsensusPeersSync] getUint32Bytes error: %v", err)
		return
	}
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		v, err := this.getSideSdk(toChainID).GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
			common.ConcatKey([]byte(header_sync.CONSENSUS_PEER), chainIDBytes, heightBytes))
		if err != nil {
			log.Errorf("[waitForSideConsensusPeersSync] side chain %d consensus peers to side chain %d, sdk.GetStorage error: %s",
				fromChain, toChainID, err)
			return
		}
		if len(v) != 0 {
			return
		}
	}
	log.Errorf("[waitForSideConsensusPeersSync] side chain %d consensus peers to side chain %d, 60s timeout", fromChain, toChainID)
}
