package orchestrator

import (
	"errors"
	"fmt"
	"time"

	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
)

var (
	startBlockEnv = int(environment.GetInt64("START_BLOCK", 0))
	endBlockEnv   = int(environment.GetInt64("END_BLOCK", 0))
	blockTag      = environment.GetString("BLOCK_TAG", "latest")
	durationCheck = time.Duration(environment.GetInt64("DURATION_CHECK", 5000)) * time.Millisecond
)

var (
	validTags = map[string]bool{
		"latest":    true,
		"safe":      true,
		"finalized": true,
	}
)

type Orchestrator struct {
	synchronizer types.Synchronizer
	fetcher      types.Fetcher
	balancer     types.Balancer
	persistor    types.Persistor
	mode         types.Mode
}

func StartOrchestrator(s types.Synchronizer, f types.Fetcher,
	b types.Balancer, p types.Persistor) error {
	o := &Orchestrator{
		synchronizer: s,
		fetcher:      f,
		balancer:     b,
		persistor:    p,
	}

	if startBlockEnv != 0 && endBlockEnv != 0 {
		return o.manageRange()
	}

	if !validTags[blockTag] {
		return errors.New("invalid tag")
	}

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

	startBlock := lastIndex + 1
	go o.synchronizer.Start(startBlock)
	go o.fetcher.RequestRange(startBlock, endBlock)
	go o.orchestrate(endBlock)

	return nil
}

func (o *Orchestrator) orchestrate(endBlock int) {
	currEndblock := endBlock
	ticker := time.NewTicker(durationCheck)
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
