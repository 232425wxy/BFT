package consensus

import (
	abci "github.com/232425wxy/BFT/abci/types"
	"github.com/232425wxy/BFT/libs/clist"
	mempl "github.com/232425wxy/BFT/mempool"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	protostate "github.com/232425wxy/BFT/proto/state"
	"github.com/232425wxy/BFT/proxy"
	"github.com/232425wxy/BFT/types"
)

type emptyMempool struct{}

var _ mempl.Mempool = emptyMempool{}

func (emptyMempool) Lock()     {}
func (emptyMempool) Unlock()   {}
func (emptyMempool) Size() int { return 0 }
func (emptyMempool) CheckTx(_ types.Tx, _ func(*protoabci.Response), _ mempl.TxInfo) error {
	return nil
}
func (emptyMempool) ReapMaxBytes(_ int64) types.Txs { return types.Txs{} }
func (emptyMempool) ReapMaxTxs(n int) types.Txs     { return types.Txs{} }
func (emptyMempool) Update(
	_ int64,
	_ types.Txs,
	_ []*protoabci.ResponseDeliverTx,
	_ mempl.PreCheckFunc,
) error {
	return nil
}
func (emptyMempool) Flush()                        {}
func (emptyMempool) FlushAppConn() error           { return nil }
func (emptyMempool) TxsAvailable() <-chan struct{} { return make(chan struct{}) }
func (emptyMempool) EnableTxsAvailable()           {}
func (emptyMempool) TxsBytes() int64               { return 0 }

func (emptyMempool) TxsFront() *clist.CElement    { return nil }
func (emptyMempool) TxsWaitChan() <-chan struct{} { return nil }

func (emptyMempool) InitWAL() error { return nil }
func (emptyMempool) CloseWAL()      {}

//-----------------------------------------------------------------------------
// mockProxyApp uses ABCIResponses to give the right results.
//
// Useful because we don't want to call Reply() twice for the same block on
// the real app.

func newMockProxyApp(appHash []byte, abciResponses *protostate.ABCIResponses) *proxy.AppConnConsensus {
	clientCreator := proxy.NewLocalClientCreator(&mockProxyApp{
		appHash:       appHash,
		abciResponses: abciResponses,
	})
	cli, _ := clientCreator.NewABCIClient()
	err := cli.Start()
	if err != nil {
		panic(err)
	}
	return proxy.NewAppConnConsensus(cli)
}

type mockProxyApp struct {
	abci.BaseApplication

	appHash       []byte
	txCount       int
	abciResponses *protostate.ABCIResponses
}

func (mock *mockProxyApp) DeliverTx(req protoabci.RequestDeliverTx) protoabci.ResponseDeliverTx {
	r := mock.abciResponses.DeliverTxs[mock.txCount]
	mock.txCount++
	if r == nil {
		return protoabci.ResponseDeliverTx{}
	}
	return *r
}

func (mock *mockProxyApp) EndBlock(req protoabci.RequestEndBlock) protoabci.ResponseEndBlock {
	mock.txCount = 0
	return *mock.abciResponses.EndBlock
}

func (mock *mockProxyApp) Commit() protoabci.ResponseCommit {
	return protoabci.ResponseCommit{Data: mock.appHash}
}
