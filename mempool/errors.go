package mempool

import (
	"errors"
	"fmt"
)

var (
	// ErrTxInCache 如果 tx 已经存在于 mempool 里，就返回该错误
	ErrTxInCache = errors.New("tx already exists in cache")
)

// ErrTxTooLarge 表示 tx 太大，不能在消息中发送给其他 peers
type ErrTxTooLarge struct {
	max    int
	actual int
}

func (e ErrTxTooLarge) Error() string {
	return fmt.Sprintf("Tx too large. Max size is %d, but got %d", e.max, e.actual)
}

// ErrMempoolIsFull 在 mempool 满的时候，会返回该错误
type ErrMempoolIsFull struct {
	numTxs int
	maxTxs int

	txsBytes    int64
	maxTxsBytes int64
}

func (e ErrMempoolIsFull) Error() string {
	return fmt.Sprintf(
		"mempool is full: number of txs %d (max: %d), total txs bytes %d (max: %d)",
		e.numTxs, e.maxTxs,
		e.txsBytes, e.maxTxsBytes)
}

// ErrPreCheck 当 tx 太大时返回该错误
type ErrPreCheck struct {
	Reason error
}

func (e ErrPreCheck) Error() string {
	return e.Reason.Error()
}
