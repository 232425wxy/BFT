package indexer

import (
	"BFT/libs/pubsub/query"
	"BFT/types"
	"context"
)

// BlockIndexer BlockIndexer定义了一个接口来索引区块事件。
type BlockIndexer interface {
	// Has 如果给定高度已被索引，则返回true，数据库查询失败时返回错误。
	Has(height int64) (bool, error)

	// Index 索引一个给定区块高度的 BeginBlock 和 EndBlock 事件。
	Index(types.EventDataNewBlockHeader) error

	// Search 执行一个查询区块高度，匹配给定的 BeginBlock 和 Endblock 事件搜索条件。
	Search(ctx context.Context, q *query.Query) ([]int64, error)
}
