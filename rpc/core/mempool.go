package core

import (
	protoabci "github.com/232425wxy/BFT/proto/abci"
	"context"
	"errors"
	"fmt"
	"time"

	mempl "github.com/232425wxy/BFT/mempool"
	ctypes "github.com/232425wxy/BFT/rpc/core/types"
	rpctypes "github.com/232425wxy/BFT/rpc/jsonrpc/types"
	"github.com/232425wxy/BFT/types"
)

//-----------------------------------------------------------------------------
// NOTE: tx should be signed, but this is only checked at the app level (not by SRBFT!)

// BroadcastTxAsync returns right away, with no response. Does not wait for
// CheckTx nor DeliverTx results.
func BroadcastTxAsync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	err := env.Mempool.CheckTx(tx, nil, mempl.TxInfo{})

	if err != nil {
		return nil, err
	}
	return &ctypes.ResultBroadcastTx{Hash: tx.Hash()}, nil
}

// BroadcastTxSync returns with the response from CheckTx. Does not wait for
// DeliverTx result.
func BroadcastTxSync(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	resCh := make(chan *protoabci.Response, 1)
	err := env.Mempool.CheckTx(tx, func(res *protoabci.Response) {
		resCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		return nil, err
	}
	res := <-resCh
	r := res.GetCheckTx()
	return &ctypes.ResultBroadcastTx{
		Code:      r.Code,
		Data:      r.Data,
		Log:       r.Log,
		Codespace: r.Codespace,
		Hash:      tx.Hash(),
	}, nil
}

// BroadcastTxCommit returns with the responses from CheckTx and DeliverTx.
func BroadcastTxCommit(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	subscriber := ctx.RemoteAddr() // 如果 ctx 的 HTTPReq 不为 nil，则返回的地址是浏览器的地址

	if env.EventBus.NumClients() >= env.Config.MaxSubscriptionClients {
		return nil, fmt.Errorf("max_subscription_clients %d reached", env.Config.MaxSubscriptionClients)
	} else if env.EventBus.NumClientSubscriptions(subscriber) >= env.Config.MaxSubscriptionsPerClient {
		return nil, fmt.Errorf("max_subscriptions_per_client %d reached", env.Config.MaxSubscriptionsPerClient)
	}

	// 订阅在块中提交的 tx
	subCtx, cancel := context.WithTimeout(ctx.Context(), SubscribeTimeout)
	defer cancel()
	q := types.EventQueryTxFor(tx)
	// 创建一个订阅
	deliverTxSub, err := env.EventBus.Subscribe(subCtx, subscriber, q)
	if err != nil {
		err = fmt.Errorf("failed to subscribe to tx: %w", err)
		env.Logger.Errorw("Error on broadcast_tx_commit", "err", err)
		return nil, err
	}
	defer func() {
		// BroadcastTxCommit 执行完以后取消该订阅
		if err := env.EventBus.Unsubscribe(context.Background(), subscriber, q); err != nil {
			env.Logger.Errorw("Error unsubscribing from eventBus", "err", err)
		}
	}()

	// 广播 tx 然后等待 CheckTx 结果
	checkTxResCh := make(chan *protoabci.Response, 1)
	err = env.Mempool.CheckTx(tx, func(res *protoabci.Response) {
		// 如果 CheckTx 通过了，则会将 res 塞入到 checkTxResCh 通道里
		checkTxResCh <- res
	}, mempl.TxInfo{})
	if err != nil {
		env.Logger.Errorw("Error on broadcastTxCommit", "err", err)
		return nil, fmt.Errorf("error on broadcastTxCommit: %v", err)
	}
	checkTxResMsg := <-checkTxResCh
	checkTxRes := checkTxResMsg.GetCheckTx() // 转换为 *ResponseCheckTx 类型
	if checkTxRes.Code != protoabci.CodeTypeOK {
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: protoabci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, nil
	}

	// Wait for the tx to be included in a block or timeout.
	select {
	case msg := <-deliverTxSub.Out(): // The tx was included in a block.
		deliverTxRes := msg.Data().(types.EventDataTx)
		//time.Sleep(100 * time.Millisecond)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: deliverTxRes.Result,
			Hash:      tx.Hash(),
			Height:    deliverTxRes.Height,
		}, nil
	case <-deliverTxSub.Cancelled():
		var reason string
		if deliverTxSub.Err() == nil {
			reason = "BFT exited"
		} else {
			reason = deliverTxSub.Err().Error()
		}
		err = fmt.Errorf("deliverTxSub was cancelled (reason: %s)", reason)
		env.Logger.Errorw("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: protoabci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	case <-time.After(env.Config.TimeoutBroadcastTxCommit):
		err = errors.New("timed out waiting for tx to be included in a block")
		env.Logger.Errorw("Error on broadcastTxCommit", "err", err)
		return &ctypes.ResultBroadcastTxCommit{
			CheckTx:   *checkTxRes,
			DeliverTx: protoabci.ResponseDeliverTx{},
			Hash:      tx.Hash(),
		}, err
	}
}

// CheckTx checks the transaction without executing it. The transaction won't
// be added to the mempool either.
func CheckTx(ctx *rpctypes.Context, tx types.Tx) (*ctypes.ResultCheckTx, error) {
	res, err := env.ProxyAppMempool.CheckTxSync(protoabci.RequestCheckTx{Tx: tx})
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultCheckTx{ResponseCheckTx: *res}, nil
}
