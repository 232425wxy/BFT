package txindex

import (
	"BFT/libs/pubsub/query"
	protoabci "BFT/proto/abci"
	"context"
	"errors"
)

// TxIndexer interface defines methods to index and search transactions.
type TxIndexer interface {
	// AddBatch 分析、索引和存储一批 tx
	AddBatch(b *Batch) error

	// Index 分析、索引和存储一个 tx
	Index(result *protoabci.TxResult) error

	// Get 返回由哈希指定的 tx，如果 tx 没有被索引或存储，则返回 nil。
	Get(hash []byte) (*protoabci.TxResult, error)

	// Search allows you to query for transactions.
	Search(ctx context.Context, q *query.Query) ([]*protoabci.TxResult, error)
}

// Batch 将同时执行的多个 Index 操作组合在一起。
// 注意:Batch 不是线程安全的，在开始执行后不能修改。
type Batch struct {
	Ops []*protoabci.TxResult
}

// NewBatch 创建一个长度为 n 的 Batch
func NewBatch(n int64) *Batch {
	return &Batch{
		Ops: make([]*protoabci.TxResult, n),
	}
}

// Add 根据 result.Index 在 Batch 中指定位置添加或更新一个 TxResult 条目
func (b *Batch) Add(result *protoabci.TxResult) error {
	b.Ops[result.Index] = result
	return nil
}

// Size 返回 Batch 中 Ops 的长度
func (b *Batch) Size() int {
	return len(b.Ops)
}

// ErrorEmptyHash 在 tx 的哈希值为空的时候报该错误
var ErrorEmptyHash = errors.New("transaction hash cannot be empty")
