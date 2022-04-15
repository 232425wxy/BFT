package mempool

import (
	"BFT/gossip"
	protoabci "BFT/proto/abci"
	"BFT/types"
	"fmt"
)

// Mempool defines the mempool interface.
//
// 内存池的更新需要与 commit block 同步，这样 apps 可以在 commit 时重置它们的瞬态状态。
type Mempool interface {

	// CheckTx 如果我们调用了 Update 或者 Reap 方法，则 CheckTx 会被阻塞，它负责将新的交易提交给 APP，然后决定
	// 是否将新交易加入到内存池里
	// CheckTx 的执行流程如下：
	//	1. 获取待检查交易数据的大小，检查当前内存池中交易数据的大小加上该交易
	//	   数据的大小是否超过上限，并检查内存池中的交易数据数量是否超过上限，
	//	   最后检查该交易数据的大小是否超过单笔交易数据的大小上限
	//	2. 如果 CListMempool 的 .preCheck 不为 nil，则调用 .preCheck()
	//	   检查该笔交易
	//	3. 尝试将这笔 tx 放到缓冲区里
	//	4. 内存池将新的 tx 提交给 代理应用程序，代理应用程序执行 CheckTx 检查这个新的 tx是否可用，然后调用内
	//	   存池的 reqResCb 方法来检查代理应用程序返回的 Response，一般来说，如果应用程序认为 tx 可用，则会
	//	   将 Response 的 .Code 字段设置为 CodeTypeOK（0），这样内存池检查 Response 的 Code 字段是否等
	//	   于 CodeTypeOK，如果等于，则将其添加到内存池里
	CheckTx(tx types.Tx, callback func(*protoabci.Response), txInfo TxInfo) error

	// ReapMaxBytes 从内存池中获取交易数据，数据大小最大为 maxBytes 大小，
	// 如果 maxBytes 为负数，则返回内存池中所有的交易数据
	ReapMaxBytes(maxBytes int64) types.Txs

	// ReapMaxTxs 从内存池中获取最多 max 条交易数据，
	// 如果 max 小于 0，则返回内存池中所有交易数据
	ReapMaxTxs(max int) types.Txs

	// Lock 锁定内存池，共识模块必须能够持有锁定内存池的锁，这样才能安全更新内存池
	Lock()

	// Unlock unlocks the mempool.
	Unlock()

	// Update 通知内存池给定的 txs 已提交，可以被丢弃了
	// 注意:这应该在区块被 consensus 提交后再调用
	Update(
		blockHeight int64,
		blockTxs types.Txs,
		deliverTxResponses []*protoabci.ResponseDeliverTx,
		newPreFn PreCheckFunc,
	) error

	// FlushAppConn 刷新内存池连接，以确保完成异步 reqResCb 调用，例如 CheckTx
	FlushAppConn() error

	// Flush 从内存池中和缓存中删除所有交易数据
	Flush()

	// TxsAvailable 返回一个通道，该通道在每个区块高度触发一次，并且仅当交易数据在内存池中可用时触发
	// 注意:如果没有调用 EnableTxsAvailable，返回的通道可能是 nil 的
	TxsAvailable() <-chan struct{}

	// EnableTxsAvailable 初始化 TxsAvailable 通道，确保当内存池中的交易数据可用时将在每个区块高度触发一次
	EnableTxsAvailable()

	// Size 返回内存池中的交易数量
	Size() int

	// TxsBytes 返回内存池中所有交易数据加一起的大小，单位是：字节
	TxsBytes() int64

	// InitWAL creates a directory for the WAL file and opens a file itself. If
	// there is an error, it will be of type *PathError.
	InitWAL() error

	// CloseWAL closes and discards the underlying WAL file.
	// Any further writes will not be relayed to disk.
	CloseWAL()
}

//--------------------------------------------------------------------------------

// PreCheckFunc 是在 CheckTx 之前执行的一个可选过滤器，
// 如果返回 false 则拒绝交易，一个例子是确保交易数据不超过区块大小限制
type PreCheckFunc func(types.Tx) error

// TxInfo 是在试图将 tx 添加到内存池时传递的参数
type TxInfo struct {
	// SenderP2PID 是发送者实际的 gossip.ID，用于日志记录？
	SenderP2PID gossip.ID
}

//--------------------------------------------------------------------------------

// PreCheckMaxBytes 检查 tx 的大小是否小于或等于预期的 maxBytes，
// 如果不是则表明 tx 太大，返回一个错误
func PreCheckMaxBytes(maxBytes int64) PreCheckFunc {
	return func(tx types.Tx) error {
		txSize := types.ComputeProtoSizeForTxs([]types.Tx{tx})

		if txSize > maxBytes {
			return fmt.Errorf("tx size is too big: %d, max: %d",
				txSize, maxBytes)
		}
		return nil
	}
}
