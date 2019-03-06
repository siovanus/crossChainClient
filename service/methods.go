package service

import (
	"fmt"

	"github.com/ontio/crossChainClient/log"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
	"github.com/ontio/ontology/smartcontract/service/native/header_sync"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
	"hash/fnv"
	"time"
	"github.com/ontio/crossChainClient/common"
)

var codeVersion = byte(0)

func (this *SyncService) GetMainChainID() uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(this.config.MainChainID))
	return hash.Sum32()
}

func (this *SyncService) GetSideChainID() uint32 {
	hash := fnv.New32a()
	hash.Write([]byte(this.config.SideChainID))
	return hash.Sum32()
}

func (this *SyncService) GetGasPrice() uint64 {
	return this.config.GasPrice
}

func (this *SyncService) GetGasLimit() uint64 {
	return this.config.GasLimit
}

func (this *SyncService) GetCurrentSideChainSyncHeight(maiChainID uint32) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress
	maiChainIDBytes, err := utils.GetUint32Bytes(maiChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), maiChainIDBytes)
	value, err := this.sideSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := utils.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

func (this *SyncService) GetCurrentMainChainSyncHeight(sideChainID uint32) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress
	sideChainIDBytes, err := utils.GetUint32Bytes(sideChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), sideChainIDBytes)
	value, err := this.mainSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := utils.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

func (this *SyncService) syncHeaderToMain(height uint32) error {
	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_BLOCK_HEADER
	block, err := this.sideSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToMain] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{block.Header.ToArray()},
	}
	txHash, err := this.mainSdk.Native.InvokeNativeContract(this.GetMainChainID(), this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncHeaderToMain] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToMain] syncHeaderToMain txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) sendProofToMain(requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	sideChainIDBytes, err := utils.GetUint32Bytes(this.GetSideChainID())
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] GetUint32Bytes error:%s", err)
	//}
	prefix, err := utils.GetUint64Bytes(requestID)
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] GetUint64Bytes error:%s", err)
	//}
	//key := utils.ConcatKey(utils.CrossChainContractAddress, []byte(cross_chain.REQUEST), sideChainIDBytes, prefix)
	//mptProof, err := this.sideSdk.GetMptProof(key, height)
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] this.sideSdk.GetMptProof error: %s", err)
	//}
	var proof [][]byte
	//for _, v := range mptProof.MPTProof {
	//	proof = append(proof, v)
	//}

	key := common.ConcatKey([]byte(cross_chain.REQUEST), sideChainIDBytes, prefix)
	value, err := this.sideSdk.GetStorage(utils.CrossChainContractAddress.ToHexString(), key)
	if err != nil {
		return fmt.Errorf("[sendProofToSide] this.sideSdk.GetStorage error: %s", err)
	}

	contractAddress := utils.CrossChainContractAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		SideChainID: this.GetMainChainID(),
		ID:          requestID,
		Height:      height,
		Proof:       proof,
		Value:       value,
	}
	txHash, err := this.mainSdk.Native.InvokeNativeContract(this.GetSideChainID(), this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[sendProofToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[sendProofToSide] sendProofToSide txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) syncHeaderToSide(height uint32) error {
	chainIDBytes, err := utils.GetUint32Bytes(this.GetMainChainID())
	if err != nil {
		return fmt.Errorf("[syncHeaderToSide] chainIDBytes, getUint32Bytes error: %v", err)
	}
	heightBytes, err := utils.GetUint32Bytes(height)
	if err != nil {
		return fmt.Errorf("[syncHeaderToSide] heightBytes, getUint32Bytes error: %v", err)
	}
	v, err := this.sideSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if len(v) != 0 {
		return nil
	}
	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_BLOCK_HEADER
	block, err := this.mainSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToSide] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{block.Header.ToArray()},
	}
	txHash, err := this.sideSdk.Native.InvokeNativeContract(this.GetSideChainID(), this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncHeaderToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToSide] syncHeaderToSide txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) sendProofToSide(requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	sideChainIDBytes, err := utils.GetUint32Bytes(this.GetSideChainID())
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] GetUint32Bytes error:%s", err)
	//}
	prefix, err := utils.GetUint64Bytes(requestID)
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] GetUint64Bytes error:%s", err)
	//}
	//key := utils.ConcatKey(utils.CrossChainContractAddress, []byte(cross_chain.REQUEST), sideChainIDBytes, prefix)
	//mptProof, err := this.mainSdk.GetMptProof(key, height)
	//if err != nil {
	//	return fmt.Errorf("[sendProofToSide] this.mainSdk.GetMptProof error: %s", err)
	//}
	var proof [][]byte
	//for _, v := range mptProof.MPTProof {
	//	proof = append(proof, v)
	//}

	key := common.ConcatKey([]byte(cross_chain.REQUEST), sideChainIDBytes, prefix)
	value, err := this.mainSdk.GetStorage(utils.CrossChainContractAddress.ToHexString(), key)
	if err != nil {
		return fmt.Errorf("[sendProofToSide] this.mainSdk.GetStorage error: %s", err)
	}

	contractAddress := utils.CrossChainContractAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		SideChainID: this.GetMainChainID(),
		ID:          requestID,
		Height:      height,
		Proof:       proof,
		Value:       value,
	}
	txHash, err := this.sideSdk.Native.InvokeNativeContract(this.GetSideChainID(), this.GetGasPrice(), this.GetGasLimit(), this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[sendProofToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[sendProofToSide] sendProofToSide txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) waitForMainBlock() {
	_, err := this.mainSdk.WaitForGenerateBlock(30*time.Second, 1)
	if err != nil {
		log.Errorf("waitForMainBlock error:%s", err)
	}
}

func (this *SyncService) waitForSideBlock() {
	_, err := this.sideSdk.WaitForGenerateBlock(30*time.Second, 1)
	if err != nil {
		log.Errorf("waitForSideBlock error:%s", err)
	}
}
