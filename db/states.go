package db

import (
	"fmt"
	"github.com/ontio/multi-chain/common"
)

type Waiting struct {
	AliaChainHeight uint32
	Height          uint32
	Key             string
}

func (this *Waiting) Serialization(sink *common.ZeroCopySink) {
	sink.WriteUint32(this.AliaChainHeight)
	sink.WriteUint32(this.Height)
	sink.WriteString(this.Key)
}

func (this *Waiting) Deserialization(source *common.ZeroCopySource) error {
	aliaChainHeight, eof := source.NextUint32()
	if eof {
		return fmt.Errorf("Waiting deserialize aliaChainHeight error")
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
	this.Height = height
	this.Key = key
	return nil
}
