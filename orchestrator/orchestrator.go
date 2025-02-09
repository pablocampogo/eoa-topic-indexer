package orchestrator

import (
	"errors"
	"fmt"
	"time"

	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
)

var (
	// startBlockEnv represents the starting block for range-based processing, if not provided, it defaults to 0.
	// It is configurable via the environment variable "START_BLOCK".
	startBlockEnv = int(environment.GetInt64("START_BLOCK", 0))
	// endBlockEnv represents the ending block for range-based processing, if not provided, it defaults to 0.
	// It is configurable via the environment variable "END_BLOCK".
	endBlockEnv = int(environment.GetInt64("END_BLOCK", 0))
	// blockTag represents the block tag ("latest", "safe" or "finalized") for full processing, defaults to "latest".
	// It is configurable via the environment variable "BLOCK_TAG".
	blockTag = environment.GetString("BLOCK_TAG", "latest")
	// cycleInterval defines the interval between checks for the orchestrator's operation, defaults to 5000 milliseconds (5 seconds).
	// It is configurable via the environment variable "CYCLE_INTERVAL".
	cycleInterval = time.Duration(environment.GetInt64("CYCLE_INTERVAL", 5000)) * time.Millisecond
)

var (
	validTags = map[string]bool{
		"latest":    true,
		"safe":      true,
		"finalized": true,
	}
)

// Orchestrator is the main struct responsible for managing the synchronization and fetching of blockchain data.
// It integrates Synchronizer, Fetcher, and Persistor to handle range and full-block processing modes.
type Orchestrator struct {
	synchronizer types.Synchronizer
	fetcher      types.Fetcher
	balancer     types.Balancer
	persistor    types.Persistor
	mode         types.Mode
}

// StartOrchestrator initializes the Orchestrator with the provided components and starts the appropriate processing mode based on environment variables.
// Returns an error if an invalid tag is provided or if the mode can't be identified.
func StartOrchestrator(s types.Synchronizer, f types.Fetcher, b types.Balancer, p types.Persistor) error {
	o := &Orchestrator{
		synchronizer: s,
		fetcher:      f,
		balancer:     b,
		persistor:    p,
	}

	// Check if both start and end block are set for range mode
	if startBlockEnv != 0 && endBlockEnv != 0 {
		return o.manageRange()
	}

	// Validate the provided block tag for full mode
	if !validTags[blockTag] {
		return errors.New("invalid tag")
	}

	// If a valid tag is provided, initiate full mode processing
	if blockTag != "" {
		return o.manageFull()
	}

	return errors.New("could not identify mode")
}

func (o *Orchestrator) manageRange() error {
	o.mode = types.ModeRange
	err := o.persistor.StartDB(types.ModeRange)
	if err != nil {
		return err
	}
	go o.synchronizer.Start(startBlockEnv)
	go o.fetcher.RequestRange(startBlockEnv, endBlockEnv)
	return nil
}

func (o *Orchestrator) manageFull() error {
	o.mode = types.ModeFull
	err := o.persistor.StartDB(types.ModeFull)
	if err != nil {
		return err
	}
	endBlock, err := o.balancer.GetBlockFromTag(blockTag)
	if err != nil {
		return err
	}
	lastIndex, err := o.persistor.GetLastBlockIndex()
	if err != nil {
		return err
	}

	startBlock := lastIndex + 1 // start block is the block after the one currently saved
	go o.synchronizer.Start(startBlock)
	go o.fetcher.RequestRange(startBlock, endBlock)
	go o.orchestrate(endBlock)

	return nil
}

// orchestrate manages the periodic orchestration of block fetching based on the current synchronization state and block tag.
// It periodically checks the synchronizer's current block and compares it with the end block to adjust the fetching range.
// It restarts the fetching cycle whenever the current block exceeds the expected end block.
func (o *Orchestrator) orchestrate(endBlock int) {
	currEndblock := endBlock
	ticker := time.NewTicker(cycleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("ARRIVED TO TIC OF ORCHESTRATOR")
			currBlock := o.synchronizer.GetCurrBlock()
			if currBlock > currEndblock {
				endBlock, err := o.balancer.GetBlockFromTag(blockTag)
				if err != nil {
					fmt.Println("error getting block tag")
				}
				if currBlock > endBlock {
					fmt.Println("Synchronizer is ahead of tag")
					continue
				}
				currEndblock = endBlock
				fmt.Printf("STARTED NEW CYCLE FROM %d to %d\n", currBlock, endBlock)
				go o.fetcher.RequestRange(currBlock, endBlock)
			}
		}
	}
}
