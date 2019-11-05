package db

import (
	"encoding/hex"
	"testing"

	acommon "github.com/ontio/multi-chain/common"
	"github.com/stretchr/testify/assert"
)

func TestRetryDB(t *testing.T) {
	db, err := NewBoltDB("../testdb")
	assert.NoError(t, err)
	txHash, err := hex.DecodeString("253488b641eb25509bbd6bf7a744d130d2e7be24016144ae3a7049a9d2760cf0")
	assert.NoError(t, err)
	retry1 := &Retry{
		TxHash: txHash,
		Height: 14427,
		Key:    "0000000000000000000000000000000000000009726571756573740000000000000000253488b641eb25509bbd6bf7a744d130d2e7be24016144ae3a7049a9d2760cf0",
	}
	sink := acommon.NewZeroCopySink(nil)
	retry1.Serialization(sink)
	err = db.PutRetry(sink.Bytes())
	assert.NoError(t, err)

	retryList, err := db.GetAllRetry()
	assert.NoError(t, err)
	retry2 := new(Retry)
	err = retry2.Deserialization(acommon.NewZeroCopySource(retryList[0]))
	assert.NoError(t, err)
	assert.Equal(t, retry1.Key, retry2.Key)
}
