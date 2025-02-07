package types

import (
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

type Synchronizer interface {
	Add(item *SyncStruct) error
	GetCurrBlock() int
	Start(startBlock int)
}

type Persistor interface {
	Close()
	StartDB(mode Mode) error
	GetLastBlockIndex() (int, error)
	SaveData(registers []Register, lastBlock int) error
	GetPaginated(startIndex int, limit int) ([]*IndexedDataJSON, int, error)
}

type Fetcher interface {
	RequestRange(startBlock, endBlock int)
}

type Balancer interface {
	GetLogs(startBlock, endBlock int, topic string) ([]gethTypes.Log, error)
	GetBlockFromTag(tag string) (int, error)
	GetBlocksAndTxs(blockNumbers []uint64, txHashes []common.Hash) ([]*gethTypes.Header, []*gethTypes.Transaction, error)
}
