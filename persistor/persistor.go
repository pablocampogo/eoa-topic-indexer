package persistor

import (
	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
	bolt "go.etcd.io/bbolt"
)

const (
	dataBucketName  = "l1_data"
	adminBucketName = "admin"
	lastBlockIndex  = "last-block"
	modeIndex       = "mode"
)

var (
	dbPath = environment.GetString("DB_PATH", "l1_data.db")
)

type Persistor struct {
	db *bolt.DB
}

func NewPersistor() (*Persistor, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	return &Persistor{
		db: db,
	}, nil
}

func (p *Persistor) Close() {
	p.db.Close()
}

func (p *Persistor) StartDB(mode types.Mode) error {
	return p.db.Update(func(tx *bolt.Tx) error {
		if mode == types.ModeRange {
			if tx.Bucket([]byte(dataBucketName)) != nil {
				err := tx.DeleteBucket([]byte(dataBucketName))
				if err != nil {
					return err
				}
			}
			if tx.Bucket([]byte(adminBucketName)) != nil {
				err := tx.DeleteBucket([]byte(adminBucketName))
				if err != nil {
					return err
				}
			}
		}

		if mode == types.ModeFull {
			adminBucket := tx.Bucket([]byte(adminBucketName))
			if adminBucket != nil {
				val := adminBucket.Get([]byte(modeIndex))
				savedMode := string(val)
				if savedMode == string(types.ModeRange) {
					err := tx.DeleteBucket([]byte(adminBucketName))
					if err != nil {
						return err
					}
					if tx.Bucket([]byte(dataBucketName)) != nil {
						err := tx.DeleteBucket([]byte(dataBucketName))
						if err != nil {
							return err
						}
					}
				}
			}
		}

		_, err := tx.CreateBucketIfNotExists([]byte(dataBucketName))
		if err != nil {
			return err
		}
		adminBucket, err := tx.CreateBucketIfNotExists([]byte(adminBucketName))
		if err != nil {
			return err
		}

		err = adminBucket.Put([]byte(modeIndex), []byte(mode))
		if err != nil {
			return err
		}

		return nil
	})
}

func (p *Persistor) GetLastBlockIndex() (int, error) {
	var block int

	err := p.db.View(func(tx *bolt.Tx) error {
		adminBucket := tx.Bucket([]byte(adminBucketName))

		val := adminBucket.Get([]byte(lastBlockIndex))
		if len(val) == 0 {
			block = -1
			return nil
		}

		block = types.BytesToInt(val)

		return nil
	})
	if err != nil {
		return 0, err
	}

	return block, nil
}

func (p *Persistor) SaveData(registers []types.Register, lastBlock int) error {
	return p.db.Update(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		for _, register := range registers {
			key := types.IntToBytes(register.Key)
			val, _ := register.Data.Serialize()
			err := dataBucket.Put([]byte(key), val)
			if err != nil {
				return err
			}
		}

		adminBucket := tx.Bucket([]byte(adminBucketName))
		err := adminBucket.Put([]byte(lastBlockIndex), types.IntToBytes(lastBlock))
		if err != nil {
			return err
		}

		return nil
	})
}

func (p *Persistor) GetPaginated(startIndex int, limit int) ([]*types.IndexedDataJSON, int, error) {
	var results []*types.IndexedDataJSON
	nextIndex := startIndex

	err := p.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dataBucketName))

		c := b.Cursor()
		startKey := types.IntToBytes(startIndex)
		k, v := c.Seek([]byte(startKey))

		for i := 0; i < limit && k != nil; i++ {
			event, err := types.Deserialize(v)
			if err != nil {
				return err
			}
			results = append(results, event.ToJSON(k))
			nextIndex++
			k, v = c.Next()
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return results, nextIndex, nil
}
