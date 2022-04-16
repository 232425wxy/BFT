package types

import (
	protoabci "github.com/232425wxy/BFT/proto/abci"
)

// Application 是一个接口，允许任何有限的、确定性的状态机由基于区块链的复制引擎通过 ABCI 驱动。
// 所有方法都接受 RequestXxx 参数并返回 ResponseXxx 参数，除了 CheckTx/DeliverTx，它接受
// ' tx []byte ' 和 ' Commit '，其他的都不接受。
type Application interface {
	// Info/Query Connection
	Info(protoabci.RequestInfo) protoabci.ResponseInfo                // Return application info
	SetOption(protoabci.RequestSetOption) protoabci.ResponseSetOption // Set application option
	Query(protoabci.RequestQuery) protoabci.ResponseQuery             // Query for state

	// Mempool Connection
	CheckTx(protoabci.RequestCheckTx) protoabci.ResponseCheckTx // Validate a tx for the mempool

	// Consensus Connection
	InitChain(protoabci.RequestInitChain) protoabci.ResponseInitChain    // 初始化区块链
	BeginBlock(protoabci.RequestBeginBlock) protoabci.ResponseBeginBlock // Signals the beginning of a block
	DeliverTx(protoabci.RequestDeliverTx) protoabci.ResponseDeliverTx    // Deliver a tx for full processing
	EndBlock(protoabci.RequestEndBlock) protoabci.ResponseEndBlock       // Signals the end of a block, returns changes to the validator set
	Commit() protoabci.ResponseCommit                                    // Commit the state and return the application Merkle root hash
}

//-------------------------------------------------------
// BaseApplication is a base form of Application

var _ Application = (*BaseApplication)(nil)

type BaseApplication struct {
}

func NewBaseApplication() *BaseApplication {
	return &BaseApplication{}
}

func (BaseApplication) Info(req protoabci.RequestInfo) protoabci.ResponseInfo {
	return protoabci.ResponseInfo{}
}

func (BaseApplication) SetOption(req protoabci.RequestSetOption) protoabci.ResponseSetOption {
	return protoabci.ResponseSetOption{}
}

func (BaseApplication) DeliverTx(req protoabci.RequestDeliverTx) protoabci.ResponseDeliverTx {
	return protoabci.ResponseDeliverTx{Code: protoabci.CodeTypeOK}
}

func (BaseApplication) CheckTx(req protoabci.RequestCheckTx) protoabci.ResponseCheckTx {
	return protoabci.ResponseCheckTx{Code: protoabci.CodeTypeOK}
}

func (BaseApplication) Commit() protoabci.ResponseCommit {
	return protoabci.ResponseCommit{}
}

func (BaseApplication) Query(req protoabci.RequestQuery) protoabci.ResponseQuery {
	return protoabci.ResponseQuery{}
}

func (BaseApplication) InitChain(req protoabci.RequestInitChain) protoabci.ResponseInitChain {
	return protoabci.ResponseInitChain{}
}

func (BaseApplication) BeginBlock(req protoabci.RequestBeginBlock) protoabci.ResponseBeginBlock {
	return protoabci.ResponseBeginBlock{}
}

func (BaseApplication) EndBlock(req protoabci.RequestEndBlock) protoabci.ResponseEndBlock {
	return protoabci.ResponseEndBlock{}
}
