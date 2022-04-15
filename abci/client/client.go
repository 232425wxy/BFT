package client

import (
	"BFT/libs/service"
	protoabci "BFT/proto/abci"
	"sync"
)

const (
	dialRetryIntervalSeconds = 3
	echoRetryIntervalSeconds = 1
)

// Client 接口里的方法覆盖了以下几个接口的方法：
//	- AppConnConsensus
//	- AppConnQuery
//	- AppConnSnapshot
//	- AppConnMempool
type Client interface {
	service.Service

	SetResponseCallback(Callback)
	Error() error

	FlushAsync() *ReqRes
	EchoAsync(msg string) *ReqRes
	InfoAsync(protoabci.RequestInfo) *ReqRes
	SetOptionAsync(protoabci.RequestSetOption) *ReqRes
	DeliverTxAsync(protoabci.RequestDeliverTx) *ReqRes
	CheckTxAsync(protoabci.RequestCheckTx) *ReqRes
	QueryAsync(protoabci.RequestQuery) *ReqRes
	CommitAsync() *ReqRes
	InitChainAsync(protoabci.RequestInitChain) *ReqRes
	BeginBlockAsync(protoabci.RequestBeginBlock) *ReqRes
	EndBlockAsync(protoabci.RequestEndBlock) *ReqRes

	FlushSync() error
	EchoSync(msg string) (*protoabci.ResponseEcho, error)
	InfoSync(protoabci.RequestInfo) (*protoabci.ResponseInfo, error)
	SetOptionSync(protoabci.RequestSetOption) (*protoabci.ResponseSetOption, error)
	DeliverTxSync(protoabci.RequestDeliverTx) (*protoabci.ResponseDeliverTx, error)
	CheckTxSync(protoabci.RequestCheckTx) (*protoabci.ResponseCheckTx, error)
	QuerySync(protoabci.RequestQuery) (*protoabci.ResponseQuery, error)
	CommitSync() (*protoabci.ResponseCommit, error)
	InitChainSync(protoabci.RequestInitChain) (*protoabci.ResponseInitChain, error)
	BeginBlockSync(protoabci.RequestBeginBlock) (*protoabci.ResponseBeginBlock, error)
	EndBlockSync(protoabci.RequestEndBlock) (*protoabci.ResponseEndBlock, error)
}

type Callback func(*protoabci.Request, *protoabci.Response)

type ReqRes struct {
	*protoabci.Request
	*sync.WaitGroup
	*protoabci.Response // 不是自动设置的，所以一定要使用WaitGroup。

	mtx  sync.Mutex
	done bool                      // 在 WaitGroup.Done() 之后设置为 true。
	cb   func(*protoabci.Response) // 一个可以被设置的回调函数。
}

func NewReqRes(req *protoabci.Request) *ReqRes {
	return &ReqRes{
		Request:   req,
		WaitGroup: waitGroup1(),
		Response:  nil,

		done: false,
		cb:   nil,
	}
}

// SetCallback 设置一个回调函数，注意：如果 .done 为 true，则立马调用该回调函数
// 且 ReqRes 只能有一个回调函数
func (r *ReqRes) SetCallback(cb func(res *protoabci.Response)) {
	r.mtx.Lock()

	if r.done {
		r.mtx.Unlock()
		cb(r.Response)
		return
	}

	r.cb = cb
	r.mtx.Unlock()
}

// InvokeCallback 如果 ReqRes 的回调函数不是空的，则调用其回调函数
func (r *ReqRes) InvokeCallback() {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.cb != nil {
		r.cb(r.Response)
	}
}

// GetCallback 获取 ReqRes 的回调函数
func (r *ReqRes) GetCallback() func(*protoabci.Response) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.cb
}

// SetDone 标记 ReqRes 已经 done
func (r *ReqRes) SetDone() {
	r.mtx.Lock()
	r.done = true
	r.mtx.Unlock()
}

// waitGroup1 返回一个 sync.WaitGroup 实例，且在该 WaitGroup 中添加一个 go-routine
func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
