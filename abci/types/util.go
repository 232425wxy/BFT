package types

import (
	protoabci "github.com/232425wxy/BFT/proto/abci"
	"sort"
)

// ValidatorUpdates 是 validators 的切片类型，并且实现了 Sort 接口
type ValidatorUpdates []protoabci.ValidatorUpdate

var _ sort.Interface = (ValidatorUpdates)(nil)

func (v ValidatorUpdates) Len() int {
	return len(v)
}

// Less 按照两个 validator 之间的公钥大小进行排序
func (v ValidatorUpdates) Less(i, j int) bool {
	return v[i].PubKey.Compare(v[j].PubKey) <= 0
}

func (v ValidatorUpdates) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}
