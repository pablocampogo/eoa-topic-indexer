package balancer

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/types"
)

var (
	errRateLimit     = errors.New("request was rate limited")
	errClientBlocked = errors.New("client is blocked")
	errClientTimeout = errors.New("client timed out")
	errMaxRetries    = errors.New("max retries reached")

	// blockTime is the duration to block a client after a rate limit or timeout error.
	// It is configurable via the environment variable "BLOCK_TIME".
	blockTime = time.Duration(environment.GetInt64("BLOCK_TIME", 20)) * time.Second
	// rpcTimeout is the maximum time to wait for a response from the RPC endpoint.
	// It is configurable via the environment variable "RPC_TIMEOUT".
	rpcTimeout = time.Duration(environment.GetInt64("RPC_TIMEOUT", 20)) * time.Second
	// maxRounds is the maximum number of retry rounds allowed.
	// It is configurable via the environment variable "MAX_ROUNDS".
	maxRounds = int(environment.GetInt64("MAX_ROUNDS", 3))
	// rpcEndpoints is a list of RPC endpoints to be used for making requests.
	// It is configurable via the environment variable "ENDPOINTS" with the endpoints separated by a comma.
	rpcEndpoints = environment.MustGetStringSlice("ENDPOINTS", ",")
)

type client struct {
	ethClient    *ethclient.Client
	url          string
	blockedUntil time.Time
}

// Balancer manages multiple Ethereum RPC clients and handles retries and balancing of requests.
type Balancer struct {
	clients    []*client
	rwMutex    sync.RWMutex
	counter    atomic.Uint64
	lenClients int
}

// NewBalancer initializes and returns a new Balancer instance by connecting to RPC endpoints.
func NewBalancer() (*Balancer, error) {
	b := &Balancer{}

	for _, url := range rpcEndpoints {
		eClient, err := ethclient.Dial(url)
		if err != nil {
			return nil, err
		}
		b.clients = append(b.clients, &client{
			ethClient:    eClient,
			url:          url,
			blockedUntil: time.Now().Add(-time.Second), // this is necessary bc a 0 time is considered indefinite block
		})
	}

	b.lenClients = len(b.clients)

	return b, nil
}

func (b *Balancer) getEthClient(index int) *ethclient.Client {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	return b.clients[index].ethClient
}

func (b *Balancer) getURL(index int) string {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	return b.clients[index].url
}

func (b *Balancer) temporaryBlock(index int) {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	b.clients[index].blockedUntil = time.Now().Add(blockTime)
}

func (b *Balancer) indefiniteBlock(index int) {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	b.clients[index].blockedUntil = time.Time{}
}

func (b *Balancer) isBlocked(index int) bool {
	b.rwMutex.Lock()
	defer b.rwMutex.Unlock()

	return b.clients[index].blockedUntil.IsZero() || time.Now().Before(b.clients[index].blockedUntil)
}

// GetLogs retrieves logs for a specified block range and topic from available RPC clients.
// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
// If all retry rounds are used, types.ErrMaxRounds is returned.
// If the request is too large, types.ErrReqTooBig is returned.
// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
func (b *Balancer) GetLogs(startBlock, endBlock int, topic string) ([]gethTypes.Log, error) {
	input := &genericInput{
		logs: &logReqInput{
			startBlock: startBlock,
			endBlock:   endBlock,
			topic:      topic,
		},
	}

	output, err := b.handleRounds(input)
	if err != nil {
		return nil, err
	}

	return output.logs.logs, nil
}

// GetBlockFromTag retrieves the block number corresponding to a specified block tag.
// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
// If all retry rounds are used, types.ErrMaxRounds is returned.
// If the request is too large, types.ErrReqTooBig is returned.
// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
func (b *Balancer) GetBlockFromTag(tag string) (int, error) {
	input := &genericInput{
		blockTag: &blockTagInput{
			tag: tag,
		},
	}

	output, err := b.handleRounds(input)
	if err != nil {
		return 0, err
	}

	return output.blockTag.block, nil
}

// GetBlocksAndTxs retrieves blocks and transactions for a specified list of block numbers and transaction hashes.
// It retries the request for a number of rounds, handling rate limits by blocking temporary clients that return rate-limiting errors.
// If all retry rounds are used, types.ErrMaxRounds is returned.
// If the request is too large, types.ErrReqTooBig is returned.
// It internally handles rate limits and blocks indefinitely clients that return errors other than these.
func (b *Balancer) GetBlocksAndTxs(blockNumbers []uint64, txHashes []common.Hash) ([]*gethTypes.Header, []*gethTypes.Transaction, error) {
	input := &genericInput{
		blockAndTxs: &blocksAndTxsReqInput{
			blockNumbers: blockNumbers,
			txHashes:     txHashes,
		},
	}

	output, err := b.handleRounds(input)
	if err != nil {
		return nil, nil, err
	}

	return output.blockAndTxs.blocks, output.blockAndTxs.txs, nil
}

func handleRPCError(err error) error {
	errCode, inErr := getErrCode(err)
	if inErr != nil {
		fmt.Println("Unexpected error: ", err.Error())
		return err
	}

	for _, tooBigErrorCode := range tooBigErrorCodes {
		if errCode == tooBigErrorCode {
			return types.ErrReqTooBig
		}
	}

	if errCode == rateLimitErrorCode {
		return errRateLimit
	}

	fmt.Println("Unexpected error: ", err.Error())

	return err
}

func (b *Balancer) doRetry(input *genericInput, index int) (*genericOutput, error) {
	eClient := b.getEthClient(index)
	defer b.counter.Add(1)

	if b.isBlocked(index) {
		return nil, errClientBlocked
	}

	outputChan := make(chan *genericOutput, 1)
	errorChan := make(chan error, 1)

	go func() {
		output, err := doGenericReq(input, eClient)
		if err != nil {
			errorChan <- err
			return
		}
		outputChan <- output
	}()

	select {
	case <-time.After(rpcTimeout):
		return nil, errClientTimeout
	case err := <-errorChan:
		return nil, err
	case output := <-outputChan:
		return output, nil
	}
}

func (b *Balancer) handleRetries(input *genericInput) (*genericOutput, error) {
	var output *genericOutput
	var err error

	for retry := 0; retry < b.lenClients; retry++ {
		curr := int(b.counter.Load())
		index := curr % b.lenClients

		output, err = b.doRetry(input, index)
		if err != nil {
			if err == errClientTimeout {
				fmt.Println("rpc endoint timed out: ", b.getURL(index))
				b.temporaryBlock(index)
				continue
			}
			if err == errClientBlocked {
				fmt.Println("rpc endpoint is blocked: ", b.getURL(index))
				continue
			}
			err = handleRPCError(err)
			if err == errRateLimit {
				fmt.Println("rpc endpoint rate limited: ", b.getURL(index))
				b.temporaryBlock(index)
				continue
			}
			if err == types.ErrReqTooBig {
				return nil, err
			}
			fmt.Printf("rpc endpoint blocked indefinitely: %s, with err %s\n", b.getURL(index), err.Error())
			b.indefiniteBlock(index)
			continue
		}

		return output, nil
	}

	return nil, errMaxRetries
}

func (b *Balancer) handleRounds(input *genericInput) (*genericOutput, error) {
	var output *genericOutput
	var err error
	var rounds int

	for {
		if rounds == maxRounds {
			fmt.Println("Max rounds reached")
			return nil, types.ErrMaxRounds
		}

		output, err = b.handleRetries(input)
		if err == errMaxRetries {
			fmt.Println("Max retries reached, doing another round")
			time.Sleep(blockTime)
			rounds++
			continue
		}

		break
	}

	return output, err
}
