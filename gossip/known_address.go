package gossip

import (
	"time"
)

// knownAddress 跟踪有关已知网络地址的信息，该地址用于确定一个地址的可行性
type knownAddress struct {
	Addr            *NetAddress `json:"addr"`
	Attempts        int32       `json:"attempts"` // Attempts 表示尝试连接到该地址的次数
	LastAttemptTime time.Time       `json:"last_attempt_time"`
}

// newKnownAddress 刚生成的 knownAddress 里的 Clusters 是 nil，
// 然后其所属地址簇是普通地址簇
func newKnownAddress(addr *NetAddress) *knownAddress {
	return &knownAddress{
		Addr:            addr,
		Attempts:        0,
		LastAttemptTime: time.Now(),
	}
}

// ID 返回 knownAddress.addr.ID
func (ka *knownAddress) ID() ID {
	return ka.Addr.ID
}
