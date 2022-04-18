package core

import (
	"errors"
	"fmt"
	"sort"

	srmath "github.com/232425wxy/BFT/libs/math"
	srquery "github.com/232425wxy/BFT/libs/pubsub/query"
	coretypes "github.com/232425wxy/BFT/rpc/core/types"
	rpctypes "github.com/232425wxy/BFT/rpc/jsonrpc/types"
	"github.com/232425wxy/BFT/types"
)

// 读取介于 minHeight 和 maxHeight 高度之间的区块信息
func BlockchainInfo(ctx *rpctypes.Context, minHeight, maxHeight int64) (*coretypes.ResultBlockchainInfo, error) {
	// maximum 100 block metas
	const limit int64 = 100
	var err error
	minHeight, maxHeight, err = filterMinMax(
		env.BlockStore.Base(),
		env.BlockStore.Height(),
		minHeight,
		maxHeight,
		limit)
	if err != nil {
		return nil, err
	}
	env.Logger.Debugw("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := env.BlockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	return &coretypes.ResultBlockchainInfo{
		LastHeight: env.BlockStore.Height(),
		BlockMetas: blockMetas}, nil
}

// 如果 min 或者 max 小于 0，则返回错误，如果 min 等于 0，则修改 min，让其等于 1，如果 max 等于 0，则让其等于最新的区块高度
func filterMinMax(base, height, min, max, limit int64) (int64, int64, error) {
	// filter negatives
	if min < 0 || max < 0 {
		return min, max, fmt.Errorf("heights must be non-negative")
	}

	// adjust for default values
	if min == 0 {
		min = 1
	}
	if max == 0 {
		max = height - 1
	}

	// limit max to the height
	max = srmath.MinInt64(height, max)

	// limit min to the base
	min = srmath.MaxInt64(base, min)

	// limit min to within `limit` of max
	// so the total number of blocks returned will be `limit`
	min = srmath.MaxInt64(min, max-limit+1)

	if min > max {
		return min, max, fmt.Errorf("min height %d can't be greater than max height %d", min, max)
	}
	return min, max, nil
}

// Block gets block at a given height.
// If no height is provided, it will fetch the latest block.
func Block(ctx *rpctypes.Context, heightPtr *int64) (*coretypes.ResultBlock, error) {
	height, err := getHeight(env.BlockStore.Height(), heightPtr)
	if err != nil {
		return nil, err
	}

	block := env.BlockStore.LoadBlock(height)
	blockMeta := env.BlockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return &coretypes.ResultBlock{BlockID: types.BlockID{}, Block: block}, nil
	}
	return &coretypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

// BlockByHash gets block by hash.
func BlockByHash(ctx *rpctypes.Context, hash []byte) (*coretypes.ResultBlock, error) {
	block := env.BlockStore.LoadBlockByHash(hash)
	if block == nil {
		return &coretypes.ResultBlock{BlockID: types.BlockID{}, Block: nil}, nil
	}
	// If block is not nil, then blockMeta can't be nil.
	blockMeta := env.BlockStore.LoadBlockMeta(block.Height)
	return &coretypes.ResultBlock{BlockID: blockMeta.BlockID, Block: block}, nil
}

// Commit gets block commit at a given height.
// If no height is provided, it will fetch the commit for the latest block.
func Commit(ctx *rpctypes.Context, heightPtr *int64) (*coretypes.ResultCommit, error) {
	height, err := getHeight(env.BlockStore.Height(), heightPtr)
	if err != nil {
		return nil, err
	}

	blockMeta := env.BlockStore.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil, nil
	}
	header := blockMeta.Header

	// If the next block has not been committed yet,
	// use a non-canonical commit
	if height == env.BlockStore.Height() {
		commit := env.BlockStore.LoadSeenReply(height)
		return coretypes.NewResultCommit(&header, commit, false), nil
	}

	// Return the canonical commit (comes from the block at height+1)
	commit := env.BlockStore.LoadBlockReply(height)
	return coretypes.NewResultCommit(&header, commit, true), nil
}

// BlockResults gets ABCIResults at a given height.
// If no height is provided, it will fetch results for the latest block.
//
// Results are for the height of the block containing the txs.
// Thus response.results.deliver_tx[5] is the results of executing
// getBlock(h).Txs[5]
func BlockResults(ctx *rpctypes.Context, heightPtr *int64) (*coretypes.ResultBlockResults, error) {
	height, err := getHeight(env.BlockStore.Height(), heightPtr)
	if err != nil {
		return nil, err
	}

	results, err := env.StateStore.LoadABCIResponses(height)
	if err != nil {
		return nil, err
	}

	return &coretypes.ResultBlockResults{
		Height:                height,
		TxsResults:            results.DeliverTxs,
		BeginBlockEvents:      results.BeginBlock.Events,
		EndBlockEvents:        results.EndBlock.Events,
		ValidatorUpdates:      results.EndBlock.ValidatorUpdates,
	}, nil
}

// BlockSearch searches for a paginated set of blocks matching BeginBlock and
// EndBlock event search criteria.
func BlockSearch(
	ctx *rpctypes.Context,
	query string,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {
	q, err := srquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := env.BlockIndexer.Search(ctx.Context(), q)
	if err != nil {
		return nil, err
	}

	// sort results (must be done before pagination)
	switch orderBy {
	case "desc", "":
		sort.Slice(results, func(i, j int) bool { return results[i] > results[j] })

	case "asc":
		sort.Slice(results, func(i, j int) bool { return results[i] < results[j] })

	default:
		return nil, errors.New("expected order_by to be either `asc` or `desc` or empty")
	}

	// paginate results
	totalCount := len(results)
	perPage := validatePerPage(perPagePtr)

	page, err := validatePage(pagePtr, perPage, totalCount)
	if err != nil {
		return nil, err
	}

	skipCount := validateSkipCount(page, perPage)
	pageSize := srmath.MinInt(perPage, totalCount-skipCount)

	apiResults := make([]*coretypes.ResultBlock, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		block := env.BlockStore.LoadBlock(results[i])
		if block != nil {
			blockMeta := env.BlockStore.LoadBlockMeta(block.Height)
			if blockMeta != nil {
				apiResults = append(apiResults, &coretypes.ResultBlock{
					Block:   block,
					BlockID: blockMeta.BlockID,
				})
			}
		}
	}

	return &coretypes.ResultBlockSearch{Blocks: apiResults, TotalCount: totalCount}, nil
}
