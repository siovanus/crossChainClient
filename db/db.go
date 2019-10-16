package db

import (
	"path"
	"strings"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/ontio/multi-chain/common"
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

func (w *WaitingDB) Put(k []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTWaiting)
		err := bucket.Put(k, []byte{0x00})
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *WaitingDB) Delete(k []byte) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.db.Update(func(btx *bolt.Tx) error {
		bucket := btx.Bucket(BKTWaiting)
		err := bucket.Delete(k)
		if err != nil {
			return err
		}

		return nil
	})
}

func (w *WaitingDB) GetWaitingAndDelete(h uint32) ([]*Waiting, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	deleteList := make([][]byte, 0)
	waitingList := make([]*Waiting, 0)
	err := w.db.Update(func(tx *bolt.Tx) error {
		bw := tx.Bucket(BKTWaiting)
		err := bw.ForEach(func(k, _ []byte) error {
			waiting := new(Waiting)
			err := waiting.Deserialization(common.NewZeroCopySource(k))
			if err != nil {
				return err
			}
			waitingList = append(waitingList, waiting)
			if waiting.AliaChainHeight <= h-50 {
				deleteList = append(deleteList, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return waitingList, nil
}

func (w *WaitingDB) Close() {
	w.lock.Lock()
	w.db.Close()
	w.lock.Unlock()
}
