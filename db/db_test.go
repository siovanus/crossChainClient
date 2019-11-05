package db

import (
	"encoding/hex"
	"fmt"
	"testing"

	acommon "github.com/ontio/multi-chain/common"
	"github.com/stretchr/testify/assert"
)

func TestRetryDB(t *testing.T) {
	db, err := NewBoltDB("../testdb")
	assert.NoError(t, err)
	txHash, err := hex.DecodeString("253488b641eb25509bbd6bf7a744d130d2e7be24016144ae3a7049a9d2760cf0")
	assert.NoError(t, err)
	for i := 0; i < 10; i++ {
		retry := &Retry{
			TxHash: txHash,
			Height: uint32(i),
			Key:    "0000000000000000000000000000000000000009726571756573740000000000000000253488b641eb25509bbd6bf7a744d130d2e7be24016144ae3a7049a9d2760cf0",
		}
		sink1 := acommon.NewZeroCopySink(nil)
		retry.Serialization(sink1)
		err = db.PutRetry(sink1.Bytes())
		assert.NoError(t, err)
	}

	retryList, err := db.GetAllRetry()
	assert.NoError(t, err)
	for _, v := range retryList {
		fmt.Printf("####: %x \n", v)
		err := db.PutCheck(hex.EncodeToString(v), v)
		assert.NoError(t, err)
	}
}
