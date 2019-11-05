package db

import (
	"fmt"
	"github.com/ontio/multi-chain/common"
)

type Retry struct {
	TxHash []byte
	Height uint32
	Key    string
}

func (this *Retry) Serialization(sink *common.ZeroCopySink) {
	sink.WriteVarBytes(this.TxHash)
	sink.WriteUint32(this.Height)
	sink.WriteString(this.Key)
}

func (this *Retry) Deserialization(source *common.ZeroCopySource) error {
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

	this.TxHash = txHash
	this.Height = height
	this.Key = key
	return nil
}
