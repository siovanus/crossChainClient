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
		log.Errorf("[waitForHeaderSync] side chain %d, utils.GetUint64Bytes error: %s", chainID, err)
		return
	}
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		v, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
			common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
		if err != nil {
			log.Errorf("[waitForHeaderSync] side chain %d, sdk.GetStorage error: %s", chainID, err)
			return
		}
		if len(v) != 0 {
			return
		}
	}
}

func (this *SyncService) waitForSideHeaderSync(chainID uint64, heightBytes []byte) {
	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		log.Errorf("[waitForHeaderSync] side chain %d, utils.GetUint64Bytes error: %s", chainID, err)
		return
	}
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		v, err := this.sideChainMap[chainID].sdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
			common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
		if err != nil {
			log.Errorf("[waitForHeaderSync] side chain %d, sdk.GetStorage error: %s", chainID, err)
			return
		}
		if len(v) != 0 {
			return
		}
	}
}
