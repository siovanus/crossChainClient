package db

import (
	"path"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/ontio/crossChainClient/common"
)

var (
	BKTWaiting = []byte("Waiting")
)

type WaitingDB struct {
	lock     *sync.Mutex
	db       *bolt.DB
	filePath string
}

func NewWaitingDB(filePath string) (*WaitingDB, error) {
	if !strings.Contains(filePath, ".bin") {
		filePath = path.Join(filePath, "waiting.bin")
	}
	w := new(WaitingDB)
	db, err := bolt.Open(filePath, 0644, &bolt.Options{InitialMmapSize: 500000})
	if err != nil {
		return nil, err
	}
	w.db = db
	w.lock = new(sync.Mutex)
	w.filePath = filePath

	if err = db.Update(func(btx *bolt.Tx) error {
		_, err := btx.CreateBucketIfNotExists(BKTWaiting)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return w, nil
}

func (w *WaitingDB) Put(height uint32, v string) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTWaiting)
		heightBytes := common.GetUint32Bytes(height)
		err := bucket.Put(heightBytes, []byte(v))
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *WaitingDB) GetWaitingAndDelete(h uint32) (map[uint32]string, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	deletList := make([][]byte, 0)
	keyList := make(map[uint32]string, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTWaiting)
		err := bw.ForEach(func(k, v []byte) error {
			height := common.GetBytesUint32(k)
			keyList[height] = string(v)
			if height <= h-50 {
				deletList = append(deletList, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range deletList {
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
	return keyList, nil
}

func (w *WaitingDB) Close() {
	w.lock.Lock()
	w.db.Close()
	w.lock.Unlock()
}
