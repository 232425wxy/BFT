package state

import (
	"BFT/mempool"
	"BFT/types"
)

// TxPreCheck 几乎就在套娃：
//	1. 第一层：先通过 state 里存储的 ConsensusParams.Block.MaxBytes 和
//	   Validators.Size() 等相关数据，计算区块中数据部分大小的上限 maxDataBytes
//	2. 第二层：根据计算得来的 maxDataBytes，调用 mempool.PreCheckFunc 有目的的
//	   去检查 mempool 里的 tx 是否大于 maxDataBytes，如果大于，那这个 tx 也太大了，就会报错
func TxPreCheck(state State) mempool.PreCheckFunc {
	maxDataBytes := types.MaxBlockDataBytes(
		state.ConsensusParams.Block.MaxBytes,
		state.Validators.Size(),
	)
	return mempool.PreCheckMaxBytes(maxDataBytes)
}
