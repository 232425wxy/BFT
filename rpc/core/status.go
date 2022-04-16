package core

import (
	"time"

	srbytes "github.com/232425wxy/BFT/libs/bytes"
	coretypes "github.com/232425wxy/BFT/rpc/core/types"
	rpctypes "github.com/232425wxy/BFT/rpc/jsonrpc/types"
	"github.com/232425wxy/BFT/types"
)

// Status returns SRBFT status including node info, pubkey, latest block
// hash, app hash, block height and time.
func Status(ctx *rpctypes.Context) (*coretypes.ResultStatus, error) {
	var (
		earliestBlockHeight   int64
		earliestBlockHash     srbytes.HexBytes
		earliestAppHash       srbytes.HexBytes
		earliestBlockTimeNano int64
	)

	if earliestBlockMeta := env.BlockStore.LoadBaseMeta(); earliestBlockMeta != nil {
		earliestBlockHeight = earliestBlockMeta.Header.Height
		earliestAppHash = earliestBlockMeta.Header.AppHash
		earliestBlockHash = earliestBlockMeta.BlockID.Hash
		earliestBlockTimeNano = earliestBlockMeta.Header.Time.UnixNano()
	}

	var (
		latestBlockHash     srbytes.HexBytes
		latestAppHash       srbytes.HexBytes
		latestBlockTimeNano int64

		latestHeight = env.BlockStore.Height()
	)

	if latestHeight != 0 {
		if latestBlockMeta := env.BlockStore.LoadBlockMeta(latestHeight); latestBlockMeta != nil {
			latestBlockHash = latestBlockMeta.BlockID.Hash
			latestAppHash = latestBlockMeta.Header.AppHash
			latestBlockTimeNano = latestBlockMeta.Header.Time.UnixNano()
		}
	}

	// Return the very last voting power, not the voting power of this validator
	// during the last block.
	var votingPower float64
	if val := validatorAtHeight(latestUncommittedHeight()); val != nil {
		votingPower = val.VotingPower
	}

	result := &coretypes.ResultStatus{
		NodeInfo: env.P2PTransport.NodeInfo(),
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHash:     latestBlockHash,
			LatestAppHash:       latestAppHash,
			LatestBlockHeight:   latestHeight,
			LatestBlockTime:     time.Unix(0, latestBlockTimeNano),
			EarliestBlockHash:   earliestBlockHash,
			EarliestAppHash:     earliestAppHash,
			EarliestBlockHeight: earliestBlockHeight,
			EarliestBlockTime:   time.Unix(0, earliestBlockTimeNano),
			CatchingUp:          env.ConsensusReactor.WaitSync(),
		},
		ValidatorInfo: coretypes.ValidatorInfo{
			Address:     env.PubKey.Address(),
			PubKey:      env.PubKey,
			VotingPower: votingPower,
		},
	}

	return result, nil
}

func validatorAtHeight(h int64) *types.Validator {
	vals, err := env.StateStore.LoadValidators(h)
	if err != nil {
		return nil
	}
	privValAddress := env.PubKey.Address()
	_, val := vals.GetByAddress(privValAddress)
	return val
}
