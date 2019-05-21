package service

import (
	"fmt"
	"time"

	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/log"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/core/types"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
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

func (this *SyncService) syncHeaderToMain(chainID uint64, header *types.Header) {
	chainIDBytes, err := utils.GetUint64Bytes(header.ShardID)
	if err != nil {
		log.Errorf("[syncSideKeyHeaderToMain] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	heightBytes, err := utils.GetUint32Bytes(header.Height)
	if err != nil {
		log.Errorf("[syncSideKeyHeaderToMain] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	v, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if err != nil {
		log.Errorf("[syncSideKeyHeaderToMain] side chain %d, sdk.GetStorage error: %s", chainID, err)
		return
	}
	if len(v) != 0 {
		log.Infof("[syncSideKeyHeaderToMain] side chain %d, syncSideKeyHeader already done", chainID)
		return
	}
	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_BLOCK_HEADER
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{header.ToArray()},
	}
	txHash, err := this.mainSdk.Native.InvokeNativeContract(chainID, this.GetGasPrice(),
		this.GetGasLimit(), this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		log.Errorf("[syncSideKeyHeaderToMain] side chain %d, invokeNativeContract error: %s", chainID, err)
		return
	}
	log.Infof("[syncSideKeyHeaderToMain] side chain %d, syncSideKeyHeader txHash is :%s", chainID, txHash.ToHexString())
	this.waitForMainHeaderSync(chainID, heightBytes)
}

func (this *SyncService) sendSideProofToMain(chainID uint64, requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	chainIDBytes, err := utils.GetUint64Bytes(this.GetMainChain())
	if err != nil {
		return fmt.Errorf("[sendSideProofToMain] side chain %d, GetUint32Bytes error:%s", chainID, err)
	}
	prefix, err := utils.GetUint64Bytes(requestID)
	if err != nil {
		return fmt.Errorf("[sendSideProofToMain] side chain %d, GetUint64Bytes error:%s", chainID, err)
	}
	key := utils.ConcatKey(utils.CrossChainContractAddress, []byte(cross_chain.REQUEST), chainIDBytes, prefix)
	crossStatesProof, err := this.getSideSdk(chainID).GetCrossStatesProof(height, key)
	if err != nil {
		return fmt.Errorf("[sendSideProofToMain] side chain %d, this.getSideSdk(chainID).GetCrossStatesProof error: %s",
			chainID, err)
	}

	contractAddress := utils.CrossChainContractAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		Address:     this.account.Address,
		FromChainID: chainID,
		Height:      height + 1,
		Proof:       crossStatesProof.AuditPath,
	}
	txHash, err := this.mainSdk.Native.InvokeNativeContract(chainID, this.GetGasPrice(), this.GetGasLimit(),
		this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[sendProofToMain] side chain %d, invokeNativeContract error: %s", chainID, err)
	}
	log.Infof("[sendProofToMain] side chain %d, sendProofToMain txHash is :%s", chainID, txHash.ToHexString())
	return nil
}

func (this *SyncService) sendSideProofToSide(fromChainID, toChainID uint64, requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	chainIDBytes, err := utils.GetUint64Bytes(toChainID)
	if err != nil {
		return fmt.Errorf("[sendSideProofToSide] side chain %d to side chain %d, GetUint32Bytes error:%s",
			fromChainID, toChainID, err)
	}
	prefix, err := utils.GetUint64Bytes(requestID)
	if err != nil {
		return fmt.Errorf("[sendSideProofToSide] side chain %d to side chain %d, GetUint64Bytes error:%s",
			fromChainID, toChainID, err)
	}
	key := utils.ConcatKey(utils.CrossChainContractAddress, []byte(cross_chain.REQUEST), chainIDBytes, prefix)
	crossStatesProof, err := this.getSideSdk(fromChainID).GetCrossStatesProof(height, key)
	if err != nil {
		return fmt.Errorf("[sendSideProofToSide] side chain %d to side chain %d, this.getSideSdk(chainID).GetCrossStatesProof error: %s",
			fromChainID, toChainID, err)
	}

	contractAddress := utils.CrossChainContractAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		Address:     this.account.Address,
		FromChainID: fromChainID,
		Height:      height + 1,
		Proof:       crossStatesProof.AuditPath,
	}
	txHash, err := this.getSideSdk(toChainID).Native.InvokeNativeContract(toChainID, this.GetGasPrice(), this.GetGasLimit(),
		this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[sendSideProofToSide] side chain %d to side chain %d, invokeNativeContract error: %s",
			fromChainID, toChainID, err)
	}
	log.Infof("[sendSideProofToSide] side chain %d to side chain %d, sendProofToMain txHash is :%s", fromChainID,
		toChainID, txHash.ToHexString())
	return nil
}

func (this *SyncService) syncMainHeader(block *types.Block) {
	for chainID := range this.getSideChainMap() {
		go this.syncHeaderToSide(chainID, block.Header)
	}
}

func (this *SyncService) syncHeaderToSide(chainID uint64, header *types.Header) {
	chainIDBytes, err := utils.GetUint64Bytes(header.ShardID)
	if err != nil {
		log.Errorf("[syncMainKeyHeaderToSide] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	heightBytes, err := utils.GetUint32Bytes(header.Height)
	if err != nil {
		log.Errorf("[syncMainKeyHeaderToSide] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	v, err := this.getSideSdk(chainID).GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if err != nil {
		log.Errorf("[syncMainKeyHeaderToSide] side chain %d, sdk.GetStorage error: %s", chainID, err)
		return
	}
	if len(v) != 0 {
		log.Infof("[syncMainKeyHeaderToSide] side chain %d, syncMainKeyHeader already done", chainID)
		return
	}
	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_BLOCK_HEADER
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{header.ToArray()},
	}
	txHash, err := this.getSideSdk(chainID).Native.InvokeNativeContract(chainID, this.GetGasPrice(),
		this.GetGasLimit(), this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		log.Errorf("[syncMainKeyHeaderToSide] side chain %d, invokeNativeContract error: %s", chainID, err)
		return
	}
	log.Infof("[syncMainKeyHeaderToSide] side chain %d, syncMainKeyHeader txHash is :%s", chainID, txHash.ToHexString())
	this.waitForSideHeaderSync(chainID, heightBytes)
}

func (this *SyncService) sendMainProofToSide(chainID uint64, requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		return fmt.Errorf("[sendMainProofToSide] main chain to side chain %d, GetUint32Bytes error:%s", chainID, err)
	}
	prefix, err := utils.GetUint64Bytes(requestID)
	if err != nil {
		return fmt.Errorf("[sendMainProofToSide] main chain to side chain %d, GetUint64Bytes error:%s", chainID, err)
	}
	key := utils.ConcatKey(utils.CrossChainContractAddress, []byte(cross_chain.REQUEST), chainIDBytes, prefix)
	crossStatesProof, err := this.mainSdk.GetCrossStatesProof(height, key)
	if err != nil {
		return fmt.Errorf("[sendMainProofToSide] main chain to side chain %d, this.mainSdk.GetCrossStatesProof error: %s", chainID, err)
	}

	contractAddress := utils.CrossChainContractAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		Address:     this.account.Address,
		FromChainID: this.GetMainChain(),
		Height:      height + 1,
		Proof:       crossStatesProof.AuditPath,
	}
	txHash, err := this.getSideSdk(chainID).Native.InvokeNativeContract(chainID, this.GetGasPrice(),
		this.GetGasLimit(), this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[sendMainProofToSide] main chain to side chain %d, invokeNativeContract error: %s", chainID, err)
	}
	log.Infof("[sendMainProofToSide] main chain to side chain %d, sendProofToSide txHash is :%s", chainID, txHash.ToHexString())
	return nil
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
