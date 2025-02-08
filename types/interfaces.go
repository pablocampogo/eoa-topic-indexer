package types

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	// ErrReqTooBig is returned by the Balancer when an RPC request fails due to the input or response exceeding the allowed size limit.
	ErrReqTooBig = errors.New("request was too big")
	// ErrMaxRounds is returned by the Balancer when the maximum number of rounds of retries has been exhausted.
	ErrMaxRounds = errors.New("max rounds done")
)

// Synchronizer is responsible for synchronizing and sending to the persistor blockchain data.
// It maintains a queue for incoming data, tracks the current block and index,
// and periodically saves accumulated data to persistent storage.
type Synchronizer interface {
	// Add sends a new SyncStruct item to the data channel for processing.
	// This function is non-blocking.
	Add(item *SyncStruct) error
	// GetCurrBlock returns the current block number being processed.
	// It safely loads the value from `currBlock` using atomic operations.
	GetCurrBlock() int
	// Start begins the synchronization process from a given start block.
	// It continuously listens for new data in the channel and periodically saves data to the persistor.
	// This function runs indefinitely and should be called in a separate goroutine.
	Start(startBlock int)
}

// Persistor handles database interactions for storing and retrieving indexed blockchain data.
type Persistor interface {
	// Close shuts down the database connection.
	Close()
	// StartDB initializes the database buckets based on the indexing mode.
	// If the mode is "range", it resets the database by deleting existing buckets.
	// If the mode is "full" and the previous mode was "range", it deletes the existing buckets.
	// If the mode is "full" and the previous mode was also "full", it keeps existing data.
	StartDB(mode Mode) error
	// GetLastBlockIndex retrieves the last indexed block number from the database.
	// Returns -1 if no block index is found.
	GetLastBlockIndex() (int, error)
	// SaveData persists indexed blockchain events and updates the last indexed block.
	SaveData(registers []Register, lastBlock int) error
	// GetPaginated retrieves a paginated list of indexed data.
	// It fetches `limit` number of items starting from `startIndex` and returns the next index.
	GetPaginated(startIndex int, limit int) ([]*IndexedDataJSON, int, error)
}

// Fetcher manages the retrieval of blockchain logs and transaction data.
type Fetcher interface {
	// RequestRange initiates an asynchronous request to fetch data for the specified block range.
	// If the error types.ErrReqTooBig is encountered, the block range is split into two halves and the fetch is retried for each half.
	// If any other error is returned, the function will panic to alert for potential issues with the code or RPC endpoints.
	// This function should be called in a separate goroutine.
	RequestRange(startBlock, endBlock int)
}

// Balancer manages multiple Ethereum RPC clients and handles retries and balancing of requests.
type Balancer interface {
	// GetLogs retrieves logs for a specified block range and topic from available RPC clients.
	// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
	// If all retry rounds are used, types.ErrMaxRounds is returned.
	// If the request is too large, types.ErrReqTooBig is returned.
	// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
	GetLogs(startBlock, endBlock int, topic string) ([]gethTypes.Log, error)
	// GetBlockFromTag retrieves the block number corresponding to a specified block tag.
	// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
	// If all retry rounds are used, types.ErrMaxRounds is returned.
	// If the request is too large, types.ErrReqTooBig is returned.
	// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
	GetBlockFromTag(tag string) (int, error)
	// GetBlocksAndTxs retrieves blocks and transactions for a specified list of block numbers and transaction hashes.
	// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
	// If all retry rounds are used, types.ErrMaxRounds is returned.
	// If the request is too large, types.ErrReqTooBig is returned.
	// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
	GetBlocksAndTxs(blockNumbers []uint64, txHashes []common.Hash) ([]*gethTypes.Header, []*gethTypes.Transaction, error)
}
