package db

import (
	"encoding/hex"
	"fmt"
	"github.com/ontio/crossChainClient/log"
	"path"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
)

const MAX_NUM = 1000

var (
	BKTCheck = []byte("Check")
	BKTRetry = []byte("Retry")
)

type BoltDB struct {
	rwlock   *sync.RWMutex
	db       *bolt.DB
	filePath string
}

func NewBoltDB(filePath string) (*BoltDB, error) {
	if !strings.Contains(filePath, ".bin") {
		filePath = path.Join(filePath, "bolt.bin")
	}
	w := new(BoltDB)
	db, err := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 500000})
	if err != nil {
		return nil, err
	}
	w.db = db
	w.rwlock = new(sync.RWMutex)
	w.filePath = filePath

	if err = db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTCheck)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err = db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTRetry)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *BoltDB) PutCheck(txHash string, v []byte) error {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	k, err := hex.DecodeString(txHash)
	if err != nil {
		return err
	}
	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTCheck)
		err := bucket.Put(k, v)
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *BoltDB) DeleteCheck(txHash string) error {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	k, err := hex.DecodeString(txHash)
	if err != nil {
		return err
	}
	return w.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BKTRetry)
		err := bucket.Delete(k)
		if err != nil {
			return err
		}
		return nil
	})
}

func (w *BoltDB) PutRetry(k []byte) error {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTRetry)
		err := bucket.Put(k, []byte{0x00})
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *BoltDB) DeleteRetry(k []byte) error {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	return w.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(BKTRetry)
		err := bucket.Delete(k)
		if err != nil {
			return err
		}
		return nil
	})
}

func (w *BoltDB) GetAllCheck() (map[string][]byte, error) {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	checkMap := make(map[string][]byte)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTCheck)
		err := bw.ForEach(func(k, v []byte) error {
			_k := make([]byte, len(k))
			_v := make([]byte, len(v))
			copy(_k, k)
			copy(_v, v)
			checkMap[hex.EncodeToString(_k)] = _v
			if len(checkMap) >= MAX_NUM {
				return fmt.Errorf("max num")
			}
			return nil
		})
		if err != nil {
			log.Errorf("GetAllCheck err: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return checkMap, nil
}

func (w *BoltDB) GetAllRetry() ([][]byte, error) {
	w.rwlock.Lock()
	defer w.rwlock.Unlock()

	retryList := make([][]byte, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTRetry)
		err := bw.ForEach(func(k, _ []byte) error {
			_k := make([]byte, len(k))
			copy(_k, k)
			retryList = append(retryList, _k)
			if len(retryList) >= MAX_NUM {
				return fmt.Errorf("max num")
			}
			return nil
		})
		if err != nil {
			log.Errorf("GetAllRetry err: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return retryList, nil
}

func (w *BoltDB) Close() {
	w.rwlock.Lock()
	w.db.Close()
	w.rwlock.Unlock()
}
