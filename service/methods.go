package service

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/db"
	"github.com/ontio/crossChainClient/log"
	acommon "github.com/ontio/multi-chain/common"
	autils "github.com/ontio/multi-chain/native/service/utils"
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
	aliaChainIDBytes := common.GetUint64Bytes(aliaChainID)
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
	contractAddress := autils.HeaderSyncContractAddress
	sideChainIDBytes := common.GetUint64Bytes(sideChainID)
	key := common.ConcatKey([]byte(header_sync.CURRENT_HEIGHT), sideChainIDBytes)
	value, err := this.aliaSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height := autils.GetBytesUint32(value)
	return height, nil
}

func (this *SyncService) syncHeaderToAlia(height uint32) error {
	chainIDBytes := common.GetUint64Bytes(this.GetSideChainID())
	heightBytes := common.GetUint32Bytes(height)
	v, err := this.aliaSdk.GetStorage(autils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if len(v) != 0 {
		return nil
	}
	block, err := this.sideSdk.GetBlockByHeight(height)
	if err != nil {
		log.Errorf("[syncHeaderToAlia] this.mainSdk.GetBlockByHeight error:%s", err)
	}
	txHash, err := this.aliaSdk.Native.Hs.SyncBlockHeader(this.GetSideChainID(), this.aliaAccount.Address, [][]byte{block.Header.ToArray()},
		this.aliaAccount)
	if err != nil {
		return fmt.Errorf("[syncHeaderToAlia] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncHeaderToAlia] syncHeaderToAlia txHash is :", txHash.ToHexString())
	this.waitForAliaBlock()
	return nil
}

func (this *SyncService) syncProofToAlia(hash []byte, key string, height uint32) error {
	k, err := hex.DecodeString(key)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] hex.DecodeString error: %s", err)
	}
	proof, err := this.sideSdk.GetCrossStatesProof(height, k)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] this.sideSdk.GetCrossStatesProof error: %s", err)
	}
	auditPath, err := hex.DecodeString(proof.AuditPath)
	if err != nil {
		return fmt.Errorf("[syncProofToAlia] hex.DecodeString error: %s", err)
	}

	retry := &db.Retry{
		TxHash: hash,
		Height: height,
		Key:    key,
	}
	sink := acommon.NewZeroCopySink(nil)
	retry.Serialization(sink)

	txHash, err := this.aliaSdk.Native.Ccm.ImportOuterTransfer(this.GetSideChainID(), hash, nil, height+1, auditPath,
		this.aliaAccount.Address[:], this.aliaAccount)
	if err != nil {
		if strings.Contains(err.Error(), "chooseUtxos, current utxo is not enough") {
			log.Infof("[syncProofToAlia] invokeNativeContract error: %s", err)

			err = this.db.PutRetry(sink.Bytes())
			if err != nil {
				log.Errorf("[syncProofToAlia] this.db.PutRetry error: %s", err)
			}
			log.Infof("[syncProofToAlia] put tx into retry db, height %d, key %s, hash %x", height, key, hash)
			return nil
		} else {
			return fmt.Errorf("[syncProofToAlia] invokeNativeContract error: %s", err)
		}
	}

	err = this.db.PutCheck(txHash.ToHexString(), sink.Bytes())
	if err != nil {
		log.Errorf("[syncProofToAlia] this.db.PutCheck error: %s", err)
	}

	log.Infof("[syncProofToAlia] syncProofToAlia txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) retrySyncProofToAlia(v []byte) error {
	retry := new(db.Retry)
	err := retry.Deserialization(acommon.NewZeroCopySource(v))
	if err != nil {
		return fmt.Errorf("[retryTx] retry.Deserialization error: %s", err)
	}
	k, err := hex.DecodeString(retry.Key)
	if err != nil {
		return fmt.Errorf("[retrySyncProofToAlia] hex.DecodeString error: %s", err)
	}
	proof, err := this.sideSdk.GetCrossStatesProof(retry.Height, k)
	if err != nil {
		return fmt.Errorf("[retrySyncProofToAlia] this.sideSdk.GetCrossStatesProof error: %s", err)
	}
	auditPath, err := hex.DecodeString(proof.AuditPath)
	if err != nil {
		return fmt.Errorf("[retrySyncProofToAlia] hex.DecodeString error: %s", err)
	}

	txHash, err := this.aliaSdk.Native.Ccm.ImportOuterTransfer(this.GetSideChainID(), retry.TxHash,
		nil, retry.Height+1, auditPath, this.aliaAccount.Address[:], this.aliaAccount)
	if err != nil {
		if strings.Contains(err.Error(), "chooseUtxos, current utxo is not enough") {
			log.Infof("[retrySyncProofToAlia] invokeNativeContract error: %s", err)
			return nil
		} else {
			if err := this.db.DeleteRetry(v); err != nil {
				log.Errorf("[retrySyncProofToAlia] this.db.DeleteRetry error: %s", err)
			}
			return fmt.Errorf("[retrySyncProofToAlia] invokeNativeContract error: %s", err)
		}
	}

	err = this.db.PutCheck(txHash.ToHexString(), v)
	if err != nil {
		log.Errorf("[retrySyncProofToAlia] this.db.PutCheck error: %s", err)
	}
	err = this.db.DeleteRetry(v)
	if err != nil {
		log.Errorf("[retrySyncProofToAlia] this.db.PutCheck error: %s", err)
	}

	log.Infof("[retrySyncProofToAlia] syncProofToAlia txHash is :", txHash.ToHexString())
	return nil
}

func (this *SyncService) syncHeaderToSide(height uint32) error {
	chainIDBytes := common.GetUint64Bytes(this.GetAliaChainID())
	heightBytes := common.GetUint32Bytes(height)
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
	proof, err := this.aliaSdk.ClientMgr.GetCrossStatesProof(height, key)
	if err != nil {
		return fmt.Errorf("[syncProofToSide] this.sideSdk.GetMptProof error: %s", err)
	}

	contractAddress := utils.CrossChainContractAddress
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

func (this *SyncService) checkDoneTx() error {
	checkMap, err := this.db.GetAllCheck()
	if err != nil {
		return fmt.Errorf("[checkDoneTx] this.db.GetAllCheck error: %s", err)
	}
	for k, v := range checkMap {
		event, err := this.aliaSdk.GetSmartContractEvent(k)
		if err != nil {
			return fmt.Errorf("[checkDoneTx] this.aliaSdk.GetSmartContractEvent error: %s", err)
		}
		if event == nil {
			log.Infof("[checkDoneTx] can not find event of hash %s", k)
			continue
		}
		if event.State != 1 {
			log.Infof("[checkDoneTx] state of tx %s is not success", k)
			err := this.db.PutRetry(v)
			if err != nil {
				log.Errorf("[checkDoneTx] this.db.PutRetry error:%s", err)
			}
		} else {
			err := this.db.DeleteCheck(k)
			if err != nil {
				log.Errorf("[checkDoneTx] this.db.DeleteRetry error:%s", err)
			}
		}
	}

	return nil
}

func (this *SyncService) retryTx() error {
	retryList, err := this.db.GetAllRetry()
	if err != nil {
		return fmt.Errorf("[retryTx] this.db.GetAllRetry error: %s", err)
	}
	for _, v := range retryList {
		err = this.retrySyncProofToAlia(v)
		if err != nil {
			log.Errorf("[retryTx] this.retrySyncProofToAlia error:%s", err)
		}
		time.Sleep(time.Duration(this.config.RetryInterval) * time.Second)
	}

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
