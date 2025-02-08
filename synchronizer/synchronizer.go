package synchronizer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
)

var (
	// syncDuration defines the maximum duration for the synchronization ticker interval.
	// It is configurable via the environment variable "SYNC_DURATION" (default: 5000ms).
	syncDuration = time.Duration(environment.GetInt64("SYNC_DURATION", 5000)) * time.Millisecond
)

// Synchronizer is responsible for synchronizing and sending to the persistor blockchain data.
// It maintains a queue for incoming data, tracks the current block and index,
// and periodically saves accumulated data to persistent storage.
type Synchronizer struct {
	datachan   chan *types.SyncStruct
	rwMutex    sync.RWMutex
	currBlock  atomic.Uint64
	currIndex  atomic.Uint64
	persistor  types.Persistor
	dataToSave map[int][]*types.IndexedDataGob
}

// NewSynchronizer initializes and returns a new Synchronizer instance.
// It takes a `Persistor` as an argument, which is responsible for saving data.
func NewSynchronizer(p types.Persistor) *Synchronizer {
	s := &Synchronizer{
		datachan:   make(chan *types.SyncStruct, 200),
		dataToSave: make(map[int][]*types.IndexedDataGob),
		persistor:  p,
	}

	return s
}

// Add sends a new SyncStruct item to the data channel for processing.
// This function is non-blocking.
func (s *Synchronizer) Add(item *types.SyncStruct) error {
	s.datachan <- item

	return nil
}

// GetCurrBlock returns the current block number being processed.
// It safely loads the value from `currBlock` using atomic operations.
func (s *Synchronizer) GetCurrBlock() int {
	return int(s.currBlock.Load())
}

func (s *Synchronizer) add(item *types.SyncStruct) {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	for i := item.StartBlock; i <= item.EndBlock; i++ {
		data, ok := item.DataToSave[i]
		if !ok {
			s.dataToSave[i] = nil
			continue
		}

		s.dataToSave[i] = data
	}
}

func (s *Synchronizer) save() error {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	startBlock := int(s.currBlock.Load())
	_, ok := s.dataToSave[startBlock]
	if !ok {
		return nil
	}

	startKey := int(s.currIndex.Load())

	var registers []types.Register
	var blockIterator int
	keyIterator := startKey
	for blockIterator = startBlock; ; blockIterator++ {
		data, ok := s.dataToSave[blockIterator]
		if !ok {
			break
		}

		for _, d := range data {
			registers = append(registers, types.Register{
				Key:  keyIterator,
				Data: d,
			})
			keyIterator++
		}
	}

	endBlock := blockIterator - 1

	err := s.persistor.SaveData(registers, endBlock)
	if err != nil {
		return err
	}

	s.currBlock.Add(uint64(blockIterator - startBlock))
	s.currIndex.Add(uint64(keyIterator - startKey))

	for i := startBlock; i < blockIterator; i++ {
		delete(s.dataToSave, i)
	}

	fmt.Printf("Succesfully saved to db from: %d to %d\n", startBlock, endBlock)

	return nil
}

// Start begins the synchronization process from a given start block.
// It continuously listens for new data in the channel and periodically saves data to the persistor.
// This function runs indefinitely and should be called in a separate goroutine.
func (s *Synchronizer) Start(startBlock int) {
	fmt.Println("Sync started with block: ", startBlock)
	s.currBlock.Add(uint64(startBlock))
	ticker := time.NewTicker(syncDuration)
	defer ticker.Stop()

	for {
		select {
		case item := <-s.datachan:
			fmt.Printf("Fetched blocks in receiver %d-%d, Data Size: %d\n", item.StartBlock, item.EndBlock, len(item.DataToSave))
			s.add(item)
		case <-ticker.C:
			fmt.Println("ARRIVED TO TIC OF SYNC")
			err := s.save()
			if err != nil {
				fmt.Println("Error saving on persistor: ", err)
			}
		}
	}
}
