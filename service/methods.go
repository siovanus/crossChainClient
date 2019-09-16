package service

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/log"
	ocommon "github.com/ontio/ontology/common"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
	"github.com/ontio/ontology/smartcontract/service/native/header_sync"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

var codeVersion = byte(0)

func (this *SyncService) GetAliaChainID() uint64 {
	return this.config.AliaChainID
}

func (this *SyncService) GetSideChainID() uint64 {
	return this.config.SideChainID
}

func (this *SyncService) GetGasPrice() uint64 {
	return this.config.GasPrice
}

func (this *SyncService) GetGasLimit() uint64 {
	return this.config.GasLimit
}

func (this *SyncService) GetCurrentSideChainSyncHeight(aliaChainID uint64) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress
	aliaChainIDBytes, err := utils.GetUint64Bytes(aliaChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), aliaChainIDBytes)
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

func (this *SyncService) GetCurrentAliaChainSyncHeight(sideChainID uint64) (uint32, error) {
	contractAddress := utils.HeaderSyncContractAddress
	sideChainIDBytes, err := utils.GetUint64Bytes(sideChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), sideChainIDBytes)
	value, err := this.aliaSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := utils.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

func (this *SyncService) syncHeaderToAlia(height uint32) error {
	chainIDBytes, err := utils.GetUint64Bytes(this.GetSideChainID())
	if err != nil {
		return fmt.Errorf("[syncHeaderToAlia] chainIDBytes, getUint32Bytes error: %v", err)
	}
	heightBytes, err := utils.GetUint32Bytes(height)
	if err != nil {
		return fmt.Errorf("[syncHeaderToAlia] heightBytes, getUint32Bytes error: %v", err)
	}
	v, err := this.aliaSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if len(v) != 0 {
		return nil
	}
	block, err := this.sideSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToAlia] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	txHash, err := this.aliaSdk.Native.Hs.SyncBlockHeader(this.aliaAccount.Address, [][]byte{block.Header.ToArray()},
		this.aliaAccount)
	if err != nil {
		return fmt.Errorf("[syncHeaderToAlia] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToAlia] syncHeaderToAlia txHash is :", txHash.ToHexString())
	this.waitForAliaBlock()
	return nil
}

func (this *SyncService) syncProofToAlia(key string, height uint32) error {
	//TODO: filter if tx is done

	k, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] hex.DecodeString error: %s", err)
	}
	proof, err := this.sideSdk.GetCrossStatesProof(height, k)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] this.sideSdk.GetCrossStatesProof error: %s", err)
	}

	txHash, err := this.aliaSdk.Native.Ccm.ImportOuterTransfer(this.GetSideChainID(), "", height + 1, proof.AuditPath,
		this.aliaAccount.Address.ToBase58(), this.GetAliaChainID(), "", this.aliaAccount)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncProofToAlia] syncProofToAlia txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) syncHeaderToSide(height uint32) error {
	chainIDBytes, err := utils.GetUint64Bytes(this.GetAliaChainID())
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
	block, err := this.aliaSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToSide] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	param := &header_sync.SyncBlockHeaderParam{
		Headers: [][]byte{block.Header.ToArray()},
	}
	txHash, err := this.sideSdk.Native.InvokeNativeContract(this.GetGasPrice(), this.GetGasLimit(), this.sideAccount, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncHeaderToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToSide] syncHeaderToSide txHash is :", txHash.ToHexString())
	this.waitForSideBlock()
	return nil
}

func (this *SyncService) syncProofToSide(key string, height uint32) error {
	//TODO: filter if tx is done

	proof, err := this.aliaSdk.ClientMgr.GetCrossStatesProof(height, key)
	if err != nil {
		return fmt.Errorf("[syncProofToSide] this.sideSdk.GetMptProof error: %s", err)
	}

	crossChainAddress, _ := ocommon.AddressParseFromBytes([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08})
	contractAddress := crossChainAddress
	method := cross_chain.PROCESS_CROSS_CHAIN_TX
	param := &cross_chain.ProcessCrossChainTxParam{
		Address:     this.sideAccount.Address,
		FromChainID: this.GetAliaChainID(),
		Height:      height + 1,
		Proof:       proof.AuditPath,
	}
	txHash, err := this.sideSdk.Native.InvokeNativeContract(this.GetGasPrice(), this.GetGasLimit(), this.sideAccount, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncProofToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncProofToSide] syncProofToSide txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) waitForAliaBlock() {
	_, err := this.aliaSdk.WaitForGenerateBlock(90*time.Second, 3)
	if err != nil {
		log.Errorf("waitForAliaBlock error:%s", err)
	}
}

func (this *SyncService) waitForSideBlock() {
	_, err := this.sideSdk.WaitForGenerateBlock(90*time.Second, 3)
	if err != nil {
		log.Errorf("waitForSideBlock error:%s", err)
	}
}
