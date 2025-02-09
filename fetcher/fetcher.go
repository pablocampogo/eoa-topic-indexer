package fetcher

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
	"golang.org/x/sync/semaphore"
)

var (
	parsedABIEvent *abi.Event
	// inputAddress specifies the Ethereum address to fetch data from.
	// It is configurable via the environment variable "ADDRESS"
	inputAddress = environment.MustGetString("ADDRESS")
	// inputTopic specifies the topic hash to fetch data from.
	// It is configurable via the environment variable "TOPIC"
	inputTopic = environment.MustGetString("TOPIC")
	// concurrency defines the number of concurrent fetch operations.
	// It is configurable via the environment variable "CONCURRENCY"
	concurrency = environment.GetInt64("CONCURRENCY", 100)
)

func init() {
	parsedABI, err := abi.JSON(strings.NewReader(rawABI))
	if err != nil {
		panic(err)
	}
	parsedABIEvent, err = parsedABI.EventByID(common.HexToHash(inputTopic))
	if err != nil {
		panic(err)
	}
}

// Fetcher manages the retrieval of blockchain logs and transaction data.
type Fetcher struct {
	balancer     types.Balancer
	synchronizer types.Synchronizer
	semaphore    *semaphore.Weighted
}

// NewFetcher creates and returns a new instance of Fetcher with the provided balancer and synchronizer.
func NewFetcher(b types.Balancer, s types.Synchronizer) *Fetcher {
	f := &Fetcher{
		balancer:     b,
		synchronizer: s,
		semaphore:    semaphore.NewWeighted(concurrency),
	}

	return f
}

// RequestRange initiates an asynchronous request to fetch data for the specified block range.
// If the error types.ErrReqTooBig is encountered, the block range is split into two halves and the fetch is retried for each half.
// If any other error is returned, the function will panic to alert for potential issues with the code or RPC endpoints.
// This function should be called in a separate goroutine.
func (f *Fetcher) RequestRange(startBlock, endBlock int) {
	go f.fetchWithRetry(startBlock, endBlock)
}

func (f *Fetcher) fetchData(startBlock, endBlock int) (map[int][]*types.IndexedDataGob, error) {
	logs, err := f.balancer.GetLogs(startBlock, endBlock, inputTopic)
	if err != nil {
		return nil, err
	}

	if len(logs) == 0 {
		return nil, nil
	}

	var bns []uint64
	bnsMap := make(map[uint64]bool, endBlock-startBlock+1)
	for _, log := range logs {
		ok := bnsMap[log.BlockNumber]
		if !ok {
			bnsMap[log.BlockNumber] = true
			bns = append(bns, log.BlockNumber)
		}
	}

	var txHashes []common.Hash
	txHashToLog := make(map[common.Hash]gethTypes.Log)
	for _, log := range logs {
		txHashToLog[log.TxHash] = log
		txHashes = append(txHashes, log.TxHash)
	}

	blocks, txs, err := f.balancer.GetBlocksAndTxs(bns, txHashes)
	if err != nil {
		return nil, err
	}

	blockToSlice := make(map[int]int, len(blocks))
	for i, block := range blocks {
		blockToSlice[int(block.Number.Int64())] = i
	}

	bnToLogIndexes := make(map[int][]int)
	bnToIndexToData := make(map[int]map[int]*types.IndexedDataGob)

	for _, tx := range txs {
		log := txHashToLog[tx.Hash()]
		bn := log.BlockNumber
		lIndex := log.Index
		block := blocks[blockToSlice[int(bn)]]
		signer := gethTypes.MakeSigner(params.SepoliaChainConfig, block.Number, block.Time)

		from, err := gethTypes.Sender(signer, tx)
		if err != nil {
			fmt.Println("Failed to get sender: ", err)
			return nil, err
		}

		eventData, err := parsedABIEvent.Inputs.Unpack(log.Data)
		if err != nil {
			fmt.Println("Failed to unpack data: ", err)
			return nil, err
		}
		l1InfoRoot, ok := eventData[0].([32]byte)
		if !ok {
			fmt.Println("Failed to cast info root")
			return nil, errors.New("wrong l1 info root type")
		}

		if strings.ToLower(tx.To().Hex()) == inputAddress || strings.ToLower(from.Hex()) == inputAddress {
			_, ok := bnToIndexToData[int(bn)]
			if !ok {
				bnToIndexToData[int(bn)] = make(map[int]*types.IndexedDataGob)
			}
			_, ok = bnToIndexToData[int(bn)][int(lIndex)]
			if !ok {
				bnToLogIndexes[int(bn)] = append(bnToLogIndexes[int(bn)], int(lIndex))
			}

			bnToIndexToData[int(bn)][int(lIndex)] = &types.IndexedDataGob{
				L1InfoRoot:      l1InfoRoot,
				BlockTime:       block.Time,
				BlockParentHash: block.ParentHash,
			}
		}
	}

	dataToSave := make(map[int][]*types.IndexedDataGob, len(bns))

	for _, bn := range bns {
		sort.Ints(bnToLogIndexes[int(bn)]) // other components assumes logs are ordered
		for _, lIndex := range bnToLogIndexes[int(bn)] {
			dataToSave[int(bn)] = append(dataToSave[int(bn)], bnToIndexToData[int(bn)][lIndex])
		}
	}

	fmt.Println("NO ERROR RETURNED ON FETCH DATA")

	return dataToSave, nil
}

func (f *Fetcher) concurrentFetchData(startBlock, endBlock int) (map[int][]*types.IndexedDataGob, error) {
	defer f.semaphore.Release(1)

	return f.fetchData(startBlock, endBlock)
}

func (f *Fetcher) fetchWithRetry(startBlock, endBlock int) {
	f.semaphore.Acquire(context.Background(), 1)

	result, err := f.concurrentFetchData(startBlock, endBlock)

	if err == nil {
		f.synchronizer.Add(&types.SyncStruct{
			StartBlock: startBlock,
			EndBlock:   endBlock,
			DataToSave: result,
		})
		return
	}

	if err == types.ErrReqTooBig {
		midBlock := (startBlock + endBlock) / 2

		leftStart, leftEnd := startBlock, midBlock
		rightStart, rightEnd := midBlock+1, endBlock

		go f.fetchWithRetry(leftStart, leftEnd)
		go f.fetchWithRetry(rightStart, rightEnd)
		return
	}

	fmt.Println("DATA WILL BE LOST IF NO PANIC IS RETURNED, PLEASE REVIEW CODE, CONFIGURATION AND/OR RPC ENDPOINTS")
	fmt.Println("Tips:")
	fmt.Println("	1. Most errors are rate limits: increase value in BLOCK_TIME env var")
	fmt.Println("	2. Most errors are timeouts: increase value in RPC_TIMEOUT env var")
	fmt.Println("	3. Errors are none of the above: looks at logs and see if a new error code parse case should be added to the balancer/error_code.go file")
	fmt.Println("In full mode, any data saved up until now will not be lost, and when you restart, it will begin from the block after the last saved one.")
	panic(err)
}
