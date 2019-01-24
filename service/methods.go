package service

import (
	"fmt"

	"github.com/ontio/crossChainClient/log"
	"github.com/ontio/ontology/smartcontract/service/native/governance"
	"github.com/ontio/ontology/smartcontract/service/native/side_chain"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

var codeVersion = byte(0)

func (this *SyncService) getMainCurrentHeaderHeight(sideChainID uint32) (uint32, error) {
	contractAddress := utils.SideChainGovernanceContractAddress
	sideChainIDBytes, err := governance.GetUint32Bytes(sideChainID)
	if err != nil {
		return 0, fmt.Errorf("GetUint32Bytes, get viewBytes error: %s", err)
	}
	key := ConcatKey([]byte(side_chain.CURRENT_HEIGHT), sideChainIDBytes)
	value, err := this.mainSdk.ClientMgr.GetStorage(contractAddress.ToHexString(), key)
	if err != nil {
		return 0, fmt.Errorf("getStorage error: %s", err)
	}
	height, err := governance.GetBytesUint32(value)
	if err != nil {
		return 0, fmt.Errorf("GetBytesUint32, get height error: %s", err)
	}
	return height, nil
}

func (this *SyncService) syncBlockHeaderToMain(param *side_chain.SyncBlockHeaderParam) error {
	contractAddress := utils.SideChainGovernanceContractAddress
	method := side_chain.SYNC_BLOCK_HEADER
	txHash, err := this.mainSdk.Native.InvokeNativeContract(this.config.GasPrice, this.config.GasLimit, this.account, codeVersion,
		contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("invokeNativeContract error: %s", err)
	}
	log.Infof("syncBlockHeaderToMain txHash is :", txHash.ToHexString())
	return nil
}

func ConcatKey(args ...[]byte) []byte {
	temp := []byte{}
	for _, arg := range args {
		temp = append(temp, arg...)
	}
	return temp
}
