package db

import (
	"fmt"
	"github.com/ontio/multi-chain/common"
)

type Waiting struct {
	AliaChainHeight uint32
	TxHash          []byte
	Height          uint32
	Key             string
}

func (this *Waiting) Serialization(sink *common.ZeroCopySink) {
	sink.WriteUint32(this.AliaChainHeight)
	sink.WriteVarBytes(this.TxHash)
	sink.WriteUint32(this.Height)
	sink.WriteString(this.Key)
}

func (this *Waiting) Deserialization(source *common.ZeroCopySource) error {
	aliaChainHeight, eof := source.NextUint32()
	if eof {
		return fmt.Errorf("Waiting deserialize aliaChainHeight error")
	}
	txHash, eof := source.NextVarBytes()
	if eof {
		return fmt.Errorf("Waiting deserialize txHash error")
	}
	height, eof := source.NextUint32()
	if eof {
		return fmt.Errorf("Waiting deserialize height error")
	}
	key, eof := source.NextString()
	if eof {
		return fmt.Errorf("Waiting deserialize key error")
	}

	this.AliaChainHeight = aliaChainHeight
	this.TxHash = txHash
	this.Height = height
	this.Key = key
	return nil
}
