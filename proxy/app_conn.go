package proxy

import (
	abcicli "BFT/abci/client"
	protoabci "BFT/proto/abci"
)

type AppConnConsensus struct {
	appConn abcicli.Client
}

func NewAppConnConsensus(appConn abcicli.Client) *AppConnConsensus {
	return &AppConnConsensus{
		appConn: appConn,
	}
}

func (app *AppConnConsensus) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *AppConnConsensus) Error() error {
	return app.appConn.Error()
}

func (app *AppConnConsensus) InitChainSync(req protoabci.RequestInitChain) (*protoabci.ResponseInitChain, error) {
	return app.appConn.InitChainSync(req)
}

func (app *AppConnConsensus) BeginBlockSync(req protoabci.RequestBeginBlock) (*protoabci.ResponseBeginBlock, error) {
	return app.appConn.BeginBlockSync(req)
}

func (app *AppConnConsensus) DeliverTxAsync(req protoabci.RequestDeliverTx) *abcicli.ReqRes {
	return app.appConn.DeliverTxAsync(req)
}

func (app *AppConnConsensus) EndBlockSync(req protoabci.RequestEndBlock) (*protoabci.ResponseEndBlock, error) {
	return app.appConn.EndBlockSync(req)
}

func (app *AppConnConsensus) CommitSync() (*protoabci.ResponseCommit, error) {
	return app.appConn.CommitSync()
}


type AppConnMempool struct {
	appConn abcicli.Client
}

func NewAppConnMempool(appConn abcicli.Client) *AppConnMempool {
	return &AppConnMempool{
		appConn: appConn,
	}
}

func (app *AppConnMempool) SetResponseCallback(cb abcicli.Callback) {
	app.appConn.SetResponseCallback(cb)
}

func (app *AppConnMempool) Error() error {
	return app.appConn.Error()
}

func (app *AppConnMempool) FlushAsync() *abcicli.ReqRes {
	return app.appConn.FlushAsync()
}

func (app *AppConnMempool) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *AppConnMempool) CheckTxAsync(req protoabci.RequestCheckTx) *abcicli.ReqRes {
	return app.appConn.CheckTxAsync(req)
}

func (app *AppConnMempool) CheckTxSync(req protoabci.RequestCheckTx) (*protoabci.ResponseCheckTx, error) {
	return app.appConn.CheckTxSync(req)
}


type AppConnQuery struct {
	appConn abcicli.Client
}

func NewAppConnQuery(appConn abcicli.Client) *AppConnQuery {
	return &AppConnQuery{
		appConn: appConn,
	}
}

func (app *AppConnQuery) Error() error {
	return app.appConn.Error()
}

func (app *AppConnQuery) EchoSync(msg string) (*protoabci.ResponseEcho, error) {
	return app.appConn.EchoSync(msg)
}

func (app *AppConnQuery) InfoSync(req protoabci.RequestInfo) (*protoabci.ResponseInfo, error) {
	return app.appConn.InfoSync(req)
}

func (app *AppConnQuery) QuerySync(reqQuery protoabci.RequestQuery) (*protoabci.ResponseQuery, error) {
	return app.appConn.QuerySync(reqQuery)
}
