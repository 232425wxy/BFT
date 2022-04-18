package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	srjson "github.com/232425wxy/BFT/libs/json"
	"github.com/232425wxy/BFT/libs/log"
	"github.com/232425wxy/BFT/libs/pubsub"
	"github.com/232425wxy/BFT/libs/service"
	rpcclient "github.com/232425wxy/BFT/rpc/client"
	coretypes "github.com/232425wxy/BFT/rpc/core/types"
	jsonrpcclient "github.com/232425wxy/BFT/rpc/jsonrpc/client"
	"github.com/232425wxy/BFT/types"
)

type HTTP struct {
	remote string
	rpc    *jsonrpcclient.Client

	*baseRPCClient
	*WSEvents
}


// rpcClient is an internal interface to which our RPC clients (batch and
// non-batch) must conform. Acts as an additional code-level sanity check to
// make sure the implementations stay coherent.
type rpcClient interface {
	rpcclient.ABCIClient
	rpcclient.HistoryClient
	rpcclient.NetworkClient
	rpcclient.SignClient
	rpcclient.StatusClient
}

// baseRPCClient implements the basic RPC method logic without the actual
// underlying RPC call functionality, which is provided by `caller`.
type baseRPCClient struct {
	caller jsonrpcclient.Caller
}

var _ rpcClient = (*HTTP)(nil)
var _ rpcClient = (*baseRPCClient)(nil)

//-----------------------------------------------------------------------------
// HTTP

// New takes a remote endpoint in the form <protocol>://<host>:<port> and
// the websocket path (which always seems to be "/websocket")
// An error is returned on invalid remote. The function panics when remote is nil.
func New(remote, wsEndpoint string) (*HTTP, error) {
	httpClient, err := jsonrpcclient.DefaultHTTPClient(remote)
	if err != nil {
		return nil, err
	}
	return NewWithClient(remote, wsEndpoint, httpClient)
}

// NewWithClient allows for setting a custom http client (See New).
// An error is returned on invalid remote. The function panics when remote is nil.
func NewWithClient(remote, wsEndpoint string, client *http.Client) (*HTTP, error) {
	if client == nil {
		panic("nil http.Client provided")
	}

	rc, err := jsonrpcclient.NewWithHTTPClient(remote, client)
	if err != nil {
		return nil, err
	}

	wsEvents, err := newWSEvents(remote, wsEndpoint)
	if err != nil {
		return nil, err
	}

	httpClient := &HTTP{
		rpc:           rc,
		remote:        remote,
		baseRPCClient: &baseRPCClient{caller: rc},
		WSEvents:      wsEvents,
	}

	return httpClient, nil
}

var _ rpcclient.Client = (*HTTP)(nil)

// SetLogger sets a logger.
func (c *HTTP) SetLogger(l log.CRLogger) {
	c.WSEvents.SetLogger(l)
}

// Remote returns the remote network address in a string form.
func (c *HTTP) Remote() string {
	return c.remote
}

//-----------------------------------------------------------------------------
// baseRPCClient

func (c *baseRPCClient) Status() (*coretypes.ResultStatus, error) {
	result := new(coretypes.ResultStatus)
	_, err := c.caller.Call("status", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ABCIInfo(ctx context.Context) (*coretypes.ResultABCIInfo, error) {
	result := new(coretypes.ResultABCIInfo)
	_, err := c.caller.Call("abci_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) ABCIQuery(
	ctx context.Context,
	data string,
) (*coretypes.ResultABCIQuery, error) {
	return c.ABCIQueryWithOptions(ctx, data, rpcclient.DefaultABCIQueryOptions)
}

func (c *baseRPCClient) ABCIQueryWithOptions(
	ctx context.Context,
	data string,
	opts rpcclient.ABCIQueryOptions) (*coretypes.ResultABCIQuery, error) {
	result := new(coretypes.ResultABCIQuery)
	_, err := c.caller.Call("abci_query",
		map[string]interface{}{"data": data, "height": opts.Height, "prove": opts.Prove},
		result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) BroadcastTxCommit(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTxCommit, error) {
	result := new(coretypes.ResultBroadcastTxCommit)
	_, err := c.caller.Call("broadcast_tx_commit", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BroadcastTxAsync(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_async", tx)
}

func (c *baseRPCClient) BroadcastTxSync(
	ctx context.Context,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	return c.broadcastTX(ctx, "broadcast_tx_sync", tx)
}

func (c *baseRPCClient) broadcastTX(
	ctx context.Context,
	route string,
	tx types.Tx,
) (*coretypes.ResultBroadcastTx, error) {
	result := new(coretypes.ResultBroadcastTx)
	_, err := c.caller.Call(route, map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) UnconfirmedTxs(
	ctx context.Context,
	limit *int,
) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)
	params := make(map[string]interface{})
	if limit != nil {
		params["limit"] = limit
	}
	_, err := c.caller.Call("unconfirmed_txs", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) NumUnconfirmedTxs(ctx context.Context) (*coretypes.ResultUnconfirmedTxs, error) {
	result := new(coretypes.ResultUnconfirmedTxs)
	_, err := c.caller.Call("num_unconfirmed_txs", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) CheckTx(ctx context.Context, tx types.Tx) (*coretypes.ResultCheckTx, error) {
	result := new(coretypes.ResultCheckTx)
	_, err := c.caller.Call("check_tx", map[string]interface{}{"tx": tx}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) NetInfo(ctx context.Context) (*coretypes.ResultNetInfo, error) {
	result := new(coretypes.ResultNetInfo)
	_, err := c.caller.Call("net_info", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) DumpConsensusState(ctx context.Context) (*coretypes.ResultDumpConsensusState, error) {
	result := new(coretypes.ResultDumpConsensusState)
	_, err := c.caller.Call("dump_consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusState(ctx context.Context) (*coretypes.ResultConsensusState, error) {
	result := new(coretypes.ResultConsensusState)
	_, err := c.caller.Call("consensus_state", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) ConsensusParams(
	ctx context.Context,
	height *int64,
) (*coretypes.ResultConsensusParams, error) {
	result := new(coretypes.ResultConsensusParams)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call("consensus_params", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Health(ctx context.Context) (*coretypes.ResultHealth, error) {
	result := new(coretypes.ResultHealth)
	_, err := c.caller.Call("health", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockchainInfo(
	ctx context.Context,
	minHeight,
	maxHeight int64,
) (*coretypes.ResultBlockchainInfo, error) {
	result := new(coretypes.ResultBlockchainInfo)
	_, err := c.caller.Call("blockchain", map[string]interface{}{"minHeight": minHeight, "maxHeight": maxHeight}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Genesis(ctx context.Context) (*coretypes.ResultGenesis, error) {
	result := new(coretypes.ResultGenesis)
	_, err := c.caller.Call("genesis", map[string]interface{}{}, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call("block", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockByHash(ctx context.Context, hash []byte) (*coretypes.ResultBlock, error) {
	result := new(coretypes.ResultBlock)
	params := map[string]interface{}{
		"hash": hash,
	}
	_, err := c.caller.Call("block_by_hash", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) BlockResults(
	ctx context.Context,
	height *int64,
) (*coretypes.ResultBlockResults, error) {
	result := new(coretypes.ResultBlockResults)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call("block_results", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	result := new(coretypes.ResultCommit)
	params := make(map[string]interface{})
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call("commit", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) Tx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	result := new(coretypes.ResultTx)
	params := map[string]interface{}{
		"hash":  hash,
		"prove": prove,
	}
	_, err := c.caller.Call("tx", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *baseRPCClient) TxSearch(
	ctx context.Context,
	query string,
	prove bool,
	page,
	perPage *int,
	orderBy string,
) (*coretypes.ResultTxSearch, error) {

	result := new(coretypes.ResultTxSearch)
	params := map[string]interface{}{
		"query":    query,
		"prove":    prove,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	_, err := c.caller.Call("tx_search", params, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) BlockSearch(
	ctx context.Context,
	query string,
	page, perPage *int,
	orderBy string,
) (*coretypes.ResultBlockSearch, error) {

	result := new(coretypes.ResultBlockSearch)
	params := map[string]interface{}{
		"query":    query,
		"order_by": orderBy,
	}

	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}

	_, err := c.caller.Call("block_search", params, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *baseRPCClient) Validators(
	ctx context.Context,
	height *int64,
	page,
	perPage *int,
) (*coretypes.ResultValidators, error) {
	result := new(coretypes.ResultValidators)
	params := make(map[string]interface{})
	if page != nil {
		params["page"] = page
	}
	if perPage != nil {
		params["per_page"] = perPage
	}
	if height != nil {
		params["height"] = height
	}
	_, err := c.caller.Call("validators", params, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

//-----------------------------------------------------------------------------
// WSEvents

var errNotRunning = errors.New("client is not running. Use .Start() method to start")

// WSEvents is a wrapper around WSClient, which implements EventsClient.
type WSEvents struct {
	service.BaseService
	remote   string
	endpoint string
	ws       *jsonrpcclient.WSClient

	mtx           sync.RWMutex
	subscriptions map[string]chan coretypes.ResultEvent // query -> chan
}

func newWSEvents(remote, endpoint string) (*WSEvents, error) {
	w := &WSEvents{
		endpoint:      endpoint,
		remote:        remote,
		subscriptions: make(map[string]chan coretypes.ResultEvent),
	}
	w.BaseService = *service.NewBaseService(nil, "WSEvents", w)

	var err error
	w.ws, err = jsonrpcclient.NewWS(w.remote, w.endpoint, jsonrpcclient.OnReconnect(func() {
		// resubscribe immediately
		w.redoSubscriptionsAfter(0 * time.Second)
	}))
	if err != nil {
		return nil, err
	}
	w.ws.SetLogger(w.Logger)

	return w, nil
}

// OnStart implements service.Service by starting WSClient and event loop.
func (w *WSEvents) OnStart() error {
	if err := w.ws.Start(); err != nil {
		return err
	}

	go w.eventListener()

	return nil
}

// OnStop implements service.Service by stopping WSClient.
func (w *WSEvents) OnStop() {
	if err := w.ws.Stop(); err != nil {
		w.Logger.Warnw("Can't stop ws client", "err", err)
	}
}

// Subscribe implements EventsClient by using WSClient to subscribe given
// subscriber to query. By default, returns a channel with cap=1. Error is
// returned if it fails to subscribe.
//
// Channel is never closed to prevent clients from seeing an erroneous event.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) Subscribe(ctx context.Context, subscriber, query string,
	outCapacity ...int) (out <-chan coretypes.ResultEvent, err error) {

	if !w.IsRunning() {
		return nil, errNotRunning
	}

	if err := w.ws.Subscribe(ctx, query); err != nil {
		return nil, err
	}

	outCap := 1
	if len(outCapacity) > 0 {
		outCap = outCapacity[0]
	}

	outc := make(chan coretypes.ResultEvent, outCap)
	w.mtx.Lock()
	// subscriber param is ignored because SRBFT will override it with
	// remote IP anyway.
	w.subscriptions[query] = outc
	w.mtx.Unlock()

	return outc, nil
}

// Unsubscribe implements EventsClient by using WSClient to unsubscribe given
// subscriber from query.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) Unsubscribe(ctx context.Context, subscriber, query string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.Unsubscribe(ctx, query); err != nil {
		return err
	}

	w.mtx.Lock()
	_, ok := w.subscriptions[query]
	if ok {
		delete(w.subscriptions, query)
	}
	w.mtx.Unlock()

	return nil
}

// UnsubscribeAll implements EventsClient by using WSClient to unsubscribe
// given subscriber from all the queries.
//
// It returns an error if WSEvents is not running.
func (w *WSEvents) UnsubscribeAll(ctx context.Context, subscriber string) error {
	if !w.IsRunning() {
		return errNotRunning
	}

	if err := w.ws.UnsubscribeAll(ctx); err != nil {
		return err
	}

	w.mtx.Lock()
	w.subscriptions = make(map[string]chan coretypes.ResultEvent)
	w.mtx.Unlock()

	return nil
}

// After being reconnected, it is necessary to redo subscription to server
// otherwise no data will be automatically received.
func (w *WSEvents) redoSubscriptionsAfter(d time.Duration) {
	time.Sleep(d)

	w.mtx.RLock()
	defer w.mtx.RUnlock()
	for q := range w.subscriptions {
		err := w.ws.Subscribe(context.Background(), q)
		if err != nil {
			w.Logger.Warnw("Failed to resubscribe", "err", err)
		}
	}
}

func isErrAlreadySubscribed(err error) bool {
	return strings.Contains(err.Error(), pubsub.ErrAlreadySubscribed.Error())
}

func (w *WSEvents) eventListener() {
	for {
		select {
		case resp, ok := <-w.ws.ResponsesCh:
			if !ok {
				return
			}

			if resp.Error != nil {
				w.Logger.Warnw("WS error", "err", resp.Error.Error())
				// Error can be ErrAlreadySubscribed or max client (subscriptions per
				// client) reached or SRBFT exited.
				// We can ignore ErrAlreadySubscribed, but need to retry in other
				// cases.
				if !isErrAlreadySubscribed(resp.Error) {
					// Resubscribe after 1 second to give SRBFT time to restart (if
					// crashed).
					w.redoSubscriptionsAfter(1 * time.Second)
				}
				continue
			}

			result := new(coretypes.ResultEvent)
			err := srjson.Unmarshal(resp.Result, result)
			if err != nil {
				w.Logger.Warnw("failed to unmarshal response", "err", err)
				continue
			}

			w.mtx.RLock()
			if out, ok := w.subscriptions[result.Query]; ok {
				if cap(out) == 0 {
					out <- *result
				} else {
					select {
					case out <- *result:
					default:
						w.Logger.Warnw("wanted to publish ResultEvent, but out channel is full", "result", result, "query", result.Query)
					}
				}
			}
			w.mtx.RUnlock()
		case <-w.Quit():
			return
		}
	}
}
