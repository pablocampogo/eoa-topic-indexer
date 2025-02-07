package balancer

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type logReqInput struct {
	startBlock int
	endBlock   int
	topic      string
}

type logReqOutput struct {
	logs []gethTypes.Log
}

type blocksAndTxsReqInput struct {
	blockNumbers []uint64
	txHashes     []common.Hash
}

type blocksAndTxsReqOutput struct {
	blocks []*gethTypes.Header
	txs    []*gethTypes.Transaction
}

type blockTagInput struct {
	tag string
}

type blockTagOutput struct {
	block int
}

type genericInput struct {
	logs        *logReqInput
	blockAndTxs *blocksAndTxsReqInput
	blockTag    *blockTagInput
}

type genericOutput struct {
	logs        *logReqOutput
	blockAndTxs *blocksAndTxsReqOutput
	blockTag    *blockTagOutput
}

func getBlockFromTag(input *blockTagInput, eClient *ethclient.Client) (*genericOutput, error) {
	var gethInput int64
	if input.tag == "latest" {
		gethInput = rpc.LatestBlockNumber.Int64()
	}
	if input.tag == "safe" {
		gethInput = rpc.SafeBlockNumber.Int64()
	}
	if input.tag == "finalized" {
		gethInput = rpc.FinalizedBlockNumber.Int64()
	}
	block, err := eClient.BlockByNumber(context.Background(), big.NewInt(gethInput))
	if err != nil {
		return nil, err
	}

	return &genericOutput{
		blockTag: &blockTagOutput{
			block: int(block.NumberU64()),
		},
	}, nil
}

func getLogs(input *logReqInput, eClient *ethclient.Client) (*genericOutput, error) {
	logQuery := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(input.startBlock)),
		ToBlock:   big.NewInt(int64(input.endBlock)),
		Topics:    [][]common.Hash{{common.HexToHash(input.topic)}},
	}

	logs, err := eClient.FilterLogs(context.Background(), logQuery)
	if err != nil {
		return nil, err
	}

	return &genericOutput{
		logs: &logReqOutput{
			logs: logs,
		},
	}, nil
}

func getBlocksAndTxs(input *blocksAndTxsReqInput, eClient *ethclient.Client) (*genericOutput, error) {
	var batchEls []rpc.BatchElem

	blocks := make([]*gethTypes.Header, len(input.blockNumbers))
	for i, bn := range input.blockNumbers {
		batchEls = append(batchEls, rpc.BatchElem{
			Method: "eth_getBlockByNumber",
			Args:   []any{toBlockNumArg(big.NewInt(int64(bn))), false},
			Result: &blocks[i],
		})
	}

	txs := make([]*gethTypes.Transaction, len(input.txHashes))
	for i, hash := range input.txHashes {
		batchEls = append(batchEls, rpc.BatchElem{
			Method: "eth_getTransactionByHash",
			Args:   []any{hash},
			Result: &txs[i],
		})
	}

	err := eClient.Client().BatchCall(batchEls)
	if err != nil {
		return nil, err
	}

	for _, el := range batchEls {
		if el.Error != nil {
			return nil, el.Error
		}
	}

	return &genericOutput{
		blockAndTxs: &blocksAndTxsReqOutput{
			blocks: blocks,
			txs:    txs,
		},
	}, nil
}

func doGenericReq(input *genericInput, eClient *ethclient.Client) (*genericOutput, error) {
	if input.logs != nil {
		return getLogs(input.logs, eClient)
	}

	if input.blockAndTxs != nil {
		return getBlocksAndTxs(input.blockAndTxs, eClient)
	}

	if input.blockTag != nil {
		return getBlockFromTag(input.blockTag, eClient)
	}

	return nil, errors.New("impossible code")
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	if number.Sign() >= 0 {
		return hexutil.EncodeBig(number)
	}
	// It's negative.
	if number.IsInt64() {
		return rpc.BlockNumber(number.Int64()).String()
	}
	// It's negative and large, which is invalid.
	return fmt.Sprintf("<invalid %d>", number)
}
