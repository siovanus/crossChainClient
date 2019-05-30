package service

import (
	"fmt"

	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/log"
	ocommon "github.com/ontio/ontology/common"
	"github.com/ontio/ontology/core/types"
	"github.com/ontio/ontology/smartcontract/service/native/chain_manager"
	"github.com/ontio/ontology/smartcontract/service/native/cross_chain"
	"github.com/ontio/ontology/smartcontract/service/native/header_sync"
	"github.com/ontio/ontology/smartcontract/service/native/utils"
)

func (this *SyncService) syncHeaderToMain(chainID uint64, header *types.Header) {
	chainIDBytes, err := utils.GetUint64Bytes(header.ShardID)
	if err != nil {
		log.Errorf("[syncHeaderToMain] side chain %d, GetUint64Bytes error: %s", chainID, err)
		return
	}
	heightBytes, err := utils.GetUint32Bytes(header.Height)
	if err != nil {
		log.Errorf("[syncHeaderToMain] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	v, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if err != nil {
		log.Errorf("[syncHeaderToMain] side chain %d, sdk.GetStorage error: %s", chainID, err)
		return
	}
	if len(v) != 0 {
		log.Infof("[syncHeaderToMain] side chain %d, syncSideKeyHeader already done", chainID)
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
		log.Errorf("[syncHeaderToMain] side chain %d, invokeNativeContract error: %s", chainID, err)
		return
	}
	log.Infof("[syncHeaderToMain] side chain %d, syncSideKeyHeader txHash is :%s", chainID, txHash.ToHexString())
	this.waitForMainHeaderSync(chainID, heightBytes)
}

func (this *SyncService) sendSideProofToMain(chainID uint64, requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	chainIDBytes, err := utils.GetUint64Bytes(this.GetMainChain())
	if err != nil {
		return fmt.Errorf("[sendSideProofToMain] side chain %d, GetUint64Bytes error:%s", chainID, err)
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
		return fmt.Errorf("[sendSideProofToSide] side chain %d to side chain %d, GetUint64Bytes error:%s",
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
		log.Errorf("[syncHeaderToSide] side chain %d, GetUint64Bytes error: %s", chainID, err)
		return
	}
	heightBytes, err := utils.GetUint32Bytes(header.Height)
	if err != nil {
		log.Errorf("[syncHeaderToSide] side chain %d, getUint32Bytes error: %s", chainID, err)
		return
	}
	v, err := this.getSideSdk(chainID).GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.HEADER_INDEX), chainIDBytes, heightBytes))
	if err != nil {
		log.Errorf("[syncHeaderToSide] side chain %d, sdk.GetStorage error: %s", chainID, err)
		return
	}
	if len(v) != 0 {
		log.Infof("[syncHeaderToSide] side chain %d, syncMainKeyHeader already done", chainID)
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
		log.Errorf("[syncHeaderToSide] side chain %d, invokeNativeContract error: %s", chainID, err)
		return
	}
	log.Infof("[syncHeaderToSide] side chain %d, syncMainKeyHeader txHash is :%s", chainID, txHash.ToHexString())
	this.waitForSideHeaderSync(header.ShardID, chainID, heightBytes)
}

func (this *SyncService) sendMainProofToSide(chainID uint64, requestID uint64, height uint32) error {
	//TODO: filter if tx is done

	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		return fmt.Errorf("[sendMainProofToSide] main chain to side chain %d, GetUint64Bytes error:%s", chainID, err)
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

func (this *SyncService) getSideGovernanceEpoch(chainID uint64) (*chain_manager.GovernanceEpoch, error) {
	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		return nil, fmt.Errorf("[getSideGovernanceEpoch] getUint64Bytes error: %v", err)
	}
	governanceEpochStore, err := this.mainSdk.GetStorage(utils.ChainManagerContractAddress.ToHexString(),
		common.ConcatKey([]byte(chain_manager.GOVERNANCE_EPOCH), chainIDBytes))
	if err != nil {
		return nil, fmt.Errorf("[getSideGovernanceEpoch] get governanceEpochStore error: %v", err)
	}
	governanceEpoch := &chain_manager.GovernanceEpoch{
		ChainID: chainID,
		Epoch:   120000,
	}
	if len(governanceEpochStore) != 0 {
		if err := governanceEpoch.Deserialization(ocommon.NewZeroCopySource(governanceEpochStore)); err != nil {
			return nil, fmt.Errorf("[getSideGovernanceEpoch] deserialize governanceEpoch error: %v", err)
		}
	}
	return governanceEpoch, nil
}

func (this *SyncService) getSideKeyHeightsFromMain(chainID uint64) (*header_sync.KeyHeights, error) {
	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromMain] GetUint64Bytes error:%s", err)
	}
	value, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.KEY_HEIGHTS), chainIDBytes))
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromMain] sdk.GetStorage error: %s", err)
	}
	keyHeights := &header_sync.KeyHeights{
		HeightList: make([]uint32, 0),
	}
	if len(value) == 0 {
		return nil, fmt.Errorf("[getSideKeyHeightsFromMain] key heights is empty")
	}
	err = keyHeights.Deserialization(ocommon.NewZeroCopySource(value))
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromMain] deserialize keyHeights err:%s", err)
	}
	return keyHeights, nil
}

func (this *SyncService) getSideKeyHeightsFromSide(fromChainID, toChainID uint64) (*header_sync.KeyHeights, error) {
	chainIDBytes, err := utils.GetUint64Bytes(fromChainID)
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromSide] GetUint64Bytes error:%s", err)
	}
	value, err := this.getSideSdk(toChainID).GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.KEY_HEIGHTS), chainIDBytes))
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromSide] sdk.GetStorage error: %s", err)
	}
	keyHeights := &header_sync.KeyHeights{
		HeightList: make([]uint32, 0),
	}
	err = keyHeights.Deserialization(ocommon.NewZeroCopySource(value))
	if err != nil {
		return nil, fmt.Errorf("[getSideKeyHeightsFromSide] deserialize keyHeights err:%s", err)
	}
	return keyHeights, nil
}

func (this *SyncService) getMerkleHeight(chainID uint64, keyHeight uint32) (uint32, error) {
	chainIDBytes, err := utils.GetUint64Bytes(chainID)
	if err != nil {
		return 0, fmt.Errorf("[getMerkleHeight] getUint64Bytes error: %v", err)
	}
	heightBytes, err := utils.GetUint32Bytes(keyHeight)
	if err != nil {
		return 0, fmt.Errorf("[getMerkleHeight] getUint32Bytes error: %v", err)
	}
	merkleHeightStore, err := this.mainSdk.GetStorage(utils.HeaderSyncContractAddress.ToHexString(),
		common.ConcatKey([]byte(header_sync.CONSENSUS_PEER_BLOCK_HEIGHT), chainIDBytes, heightBytes))
	if err != nil {
		return 0, fmt.Errorf("[getMerkleHeight] get merkleHeightStore error: %v", err)
	}
	if len(merkleHeightStore) == 0 {
		return 0, fmt.Errorf("[getMerkleHeight] merkleHeightStore is empty")
	}
	merkleHeight, err := utils.GetBytesUint32(merkleHeightStore)
	if err != nil {
		return 0, fmt.Errorf("[getMerkleHeight] utils.GetBytesUint32 err:%v", err)
	}
	return merkleHeight, nil
}

func (this *SyncService) syncConsensusPeersToSide(fromChainID, toChainID uint64, merkleHeight, keyHeight uint32) error {
	block, err := this.mainSdk.GetBlockByHeight(merkleHeight + 1)
	if err != nil {
		log.Errorf("[syncConsensusPeersToSide] this.mainSdk.GetBlockByHeight %d error:%s", merkleHeight+1, err)
	}
	chainIDBytes, err := utils.GetUint64Bytes(fromChainID)
	if err != nil {
		return fmt.Errorf("[syncConsensusPeersToSide] GetUint64Bytes error:%s", err)
	}
	heightBytes, err := utils.GetUint32Bytes(keyHeight)
	if err != nil {
		return fmt.Errorf("[syncConsensusPeersToSide] getUint32Bytes error: %v", err)
	}
	key := utils.ConcatKey(utils.HeaderSyncContractAddress, []byte(header_sync.CONSENSUS_PEER), chainIDBytes, heightBytes)
	crossStatesProof, err := this.mainSdk.GetCrossStatesProof(merkleHeight, key)
	if err != nil {
		return fmt.Errorf("[syncConsensusPeersToSide] this.mainSdk.GetCrossStatesProof error: %s", err)
	}

	contractAddress := utils.HeaderSyncContractAddress
	method := header_sync.SYNC_CONSENSUS_PEERS
	param := &header_sync.SyncConsensusPeerParam{
		Header: block.Header.ToArray(),
		Proof:  crossStatesProof.AuditPath,
	}
	txHash, err := this.getSideSdk(toChainID).Native.InvokeNativeContract(toChainID, this.GetGasPrice(),
		this.GetGasLimit(), this.account, codeVersion, contractAddress, method, []interface{}{param})
	if err != nil {
		return fmt.Errorf("[syncConsensusPeersToSide] invokeNativeContract error: %s", err)
	}
	log.Infof("[syncConsensusPeersToSide] sendProofToSide txHash is :%s", txHash.ToHexString())
	return nil
}

func (this *SyncService) checkConsensusPeers(fromChainID, toChainID uint64, height uint32) (bool, error) {
	keyHeights, err := this.getSideKeyHeightsFromSide(fromChainID, toChainID)
	if err != nil {
		return false, fmt.Errorf("[checkConsensusPeers] this.getSideKeyHeightsFromSide error:%s", err)
	}
	var keyHeight uint32
	for _, v := range keyHeights.HeightList {
		if (height - v) > 0 {
			keyHeight = v
		}
	}
	governanceEpoch, err := this.getSideGovernanceEpoch(fromChainID)
	if err != nil {
		return false, fmt.Errorf("[checkConsensusPeers] this.getSideGovernanceEpoch error:%s", err)
	}
	if height-keyHeight > governanceEpoch.Epoch || len(keyHeights.HeightList) == 0 {
		log.Infof("[checkConsensusPeers] check false")
		return false, nil
	} else {
		log.Infof("[checkConsensusPeers] check true")
		return true, nil
	}
}

func (this *SyncService) syncConsensusPeersFromSideToSide(fromChainID, toChainID uint64, height uint32) error {
	keyHeights, err := this.getSideKeyHeightsFromMain(fromChainID)
	if err != nil {
		log.Errorf("[syncConsensusPeersFromSideToSide] this.getSideKeyHeights error:%s", err)
	}
	var keyHeight uint32
	for _, v := range keyHeights.HeightList {
		if (height - v) > 0 {
			keyHeight = v
		}
	}
	governanceEpoch, err := this.getSideGovernanceEpoch(fromChainID)
	if err != nil {
		log.Errorf("[syncConsensusPeersFromSideToSide] this.getSideGovernanceEpoch error:%s", err)
	}
	if height-keyHeight > governanceEpoch.Epoch {
		log.Errorf("[syncConsensusPeersFromSideToSide] some key height is missing in main chain")
	}
	//get merkle height according to key height
	merkleHeight, err := this.getMerkleHeight(fromChainID, keyHeight)
	if err != nil {
		log.Errorf("[syncConsensusPeersFromSideToSide] this.getMerkleHeight error: %s", err)
	}
	//sync main header of merkle height and merkle proof to destination side chain
	err = this.syncConsensusPeersToSide(fromChainID, toChainID, merkleHeight, keyHeight)
	if err != nil {
		log.Errorf("[syncConsensusPeersFromSideToSide] this.syncHeaderAndProofToSide error in height %d: %s", merkleHeight, err)
	}
	this.waitForSideConsensusPeersSync(fromChainID, toChainID, merkleHeight)
	return nil
}
