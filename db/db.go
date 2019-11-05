package db

import (
	"encoding/hex"
	"path"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
)

var (
	BKTCheck = []byte("Check")
	BKTRetry = []byte("Retry")
)

type BoltDB struct {
	lock     *sync.Mutex
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
	w.lock = new(sync.Mutex)
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
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTCheck)
		k, err := hex.DecodeString(txHash)
		if err != nil {
			return err
		}
		err = bucket.Put(k, v)
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *BoltDB) PutRetry(k []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTRetry)
		err := bucket.Put(k, []byte{0x00})
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *BoltDB) GetAllCheck() (map[string][]byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	checkMap := make(map[string][]byte)
	removeList := make([][]byte, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTCheck)
		err := bw.ForEach(func(k, v []byte) error {
			checkMap[hex.EncodeToString(k)] = v
			removeList = append(removeList, k)
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range removeList {
			err = bw.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return checkMap, nil
}

func (w *BoltDB) GetAllRetry() ([][]byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	retryList := make([][]byte, 0)
	removeList := make([][]byte, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTRetry)
		err := bw.ForEach(func(k, _ []byte) error {
			retryList = append(retryList, k)
			removeList = append(removeList, k)
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range removeList {
			err = bw.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return retryList, nil
}

func (w *BoltDB) Close() {
	w.lock.Lock()
	w.db.Close()
	w.lock.Unlock()
}
