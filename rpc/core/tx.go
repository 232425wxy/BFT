package core

import (
	"errors"
	"fmt"
	"sort"

	srmath "BFT/libs/math"
	srquery "BFT/libs/pubsub/query"
	coretypes "BFT/rpc/core/types"
	rpctypes "BFT/rpc/jsonrpc/types"
	"BFT/types"
)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place.
func Tx(ctx *rpctypes.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	r, err := env.TxIndexer.Get(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return nil, fmt.Errorf("tx (%X) not found", hash)
	}

	height := r.Height
	index := r.Index

	var proof types.TxProof
	if prove {
		block := env.BlockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(int(index)) // XXX: overflow on 32-bit machines
	}

	return &coretypes.ResultTx{
		Hash:     hash,
		Height:   height,
		Index:    index,
		TxResult: r.Result,
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}

// TxSearch allows you to query for multiple transactions results. It returns a
// list of transactions (maximum ?per_page entries) and the total count.
func TxSearch(
	ctx *rpctypes.Context,
	query string,
	prove bool,
	pagePtr, perPagePtr *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {
	q, err := srquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := env.TxIndexer.Search(ctx.Context(), q)
	if err != nil {
		return nil, err
	}

	// sort results (must be done before pagination)
	switch orderBy {
	case "desc":
		sort.Slice(results, func(i, j int) bool {
			if results[i].Height == results[j].Height {
				return results[i].Index > results[j].Index
			}
			return results[i].Height > results[j].Height
		})
	case "asc", "":
		sort.Slice(results, func(i, j int) bool {
			if results[i].Height == results[j].Height {
				return results[i].Index < results[j].Index
			}
			return results[i].Height < results[j].Height
		})
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

	apiResults := make([]*coretypes.ResultTx, 0, pageSize)
	for i := skipCount; i < skipCount+pageSize; i++ {
		r := results[i]

		var proof types.TxProof
		if prove {
			block := env.BlockStore.LoadBlock(r.Height)
			proof = block.Data.Txs.Proof(int(r.Index)) // XXX: overflow on 32-bit machines
		}

		apiResults = append(apiResults, &coretypes.ResultTx{
			Hash:     types.Tx(r.Tx).Hash(),
			Height:   r.Height,
			Index:    r.Index,
			TxResult: r.Result,
			Tx:       r.Tx,
			Proof:    proof,
		})
	}

	return &coretypes.ResultTxSearch{Txs: apiResults, TotalCount: totalCount}, nil
}
