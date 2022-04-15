package main

import (
	"BFT/crypto/srhash"
	srjson "BFT/libs/json"
	srrand "BFT/libs/rand"
	srtime "BFT/libs/time"
	ctypes "BFT/rpc/core/types"
	rpctypes "BFT/rpc/jsonrpc/types"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"net/url"
	"strings"
	"time"
)

type bomber struct {
	host              string
	broadcastTxMethod string
	c                 *websocket.Conn
	quit              chan struct{}
	recv              chan []byte
	duration          int
	hasSentTxNum      int
}

func newBomber(host, broadcastTxMethod string, duration int) *bomber {
	return &bomber{
		host:              host,
		broadcastTxMethod: broadcastTxMethod,
		quit:              make(chan struct{}),
		recv:              make(chan []byte),
		duration:          duration,
		hasSentTxNum:      0,
	}
}

func (bom *bomber) start(start *time.Time) {
	u := url.URL{Scheme: "ws", Host: bom.host, Path: "/websocket"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		printErrorAndExit(fmt.Sprintf("dial to %q failed", u.String()), err)
	}
	bom.c = c

	//param, _ := json.Marshal(map[string]interface{}{"data": "A6902E6538260DDF725E112DC7C9DF8C5A7EED8B325496AF2B5A398B0BCD9BC3"})
	//err = bom.c.WriteJSON(rpctypes.RPCRequest{
	//	JSONRPC: "2.0",
	//	Method:  "abci_query",
	//	Params:  param,
	//	ID: rpctypes.JSONRPCStringID("BFT"),
	//})

	*start = time.Now()
	go bom.sendRoutine()
	go bom.receiveRoutine()


	<-bom.quit

	_ = bom.c.Close()
}

func (bom *bomber) stop() {
	close(bom.quit)
}

func (bom *bomber) sendRoutine() {
	random := srrand.NewRand()
	random.Seed(time.Now().UnixNano())
	clock := time.NewTimer(time.Duration(bom.duration) * time.Second)
	for {
		select {
		case <-bom.quit:
			return
		case <-clock.C:
			bom.stop()
		default:
			tx, raw := generateTx(bom.hasSentTxNum+1, random)
			err := bom.c.SetWriteDeadline(srtime.Now().Add(time.Second * 30))
			if err != nil {
				printErrorAndExit("set write deadline failed", err)
			}
			err = bom.c.WriteJSON(rpctypes.RPCRequest{
				JSONRPC: "2.0",
				Method:  bom.broadcastTxMethod,
				Params:  tx,
				ID: rpctypes.JSONRPCStringID("BFT"),
			})
			if err != nil {
				printErrorAndExit("send tx to websocket failed", err)
			}
			recv := <-bom.recv
			//time.Sleep(100 * time.Millisecond)
			switch broadcastTxMethod {
			case "broadcast_tx_async":
				if bytes.Equal(recv, srhash.Sum(raw)) {
					bom.hasSentTxNum++
				} else {
					logger.Errorw("receive wrong feedback", "expected", srhash.Sum(raw), "actual", string(recv))
				}
			case "broadcast_tx_sync":
				if bytes.Equal(recv, srhash.Sum(raw)) {
					bom.hasSentTxNum++
				} else {
					logger.Errorw("receive wrong feedback", "expected", srhash.Sum(raw), "actual", string(recv))
				}
			case "broadcast_tx_commit":
				if bytes.Equal(recv, raw) {
					bom.hasSentTxNum++
					logger.Infow("new block was committed successfully")
				} else {
					logger.Errorw("receive wrong feedback", "expected", srhash.Sum(raw), "actual", srhash.Sum(recv))
				}
			}

		}
	}
}

func (bom *bomber) receiveRoutine() {
	for {
		select {
		case <-bom.quit:
			return
		default:
			err := bom.c.SetReadDeadline(srtime.Now().Add(time.Second * 30))
			if err != nil {
				printErrorAndExit("set read deadline failed", err)
			}
			mType, p, err := bom.c.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				printErrorAndExit("receive message failed", err)
			}
			switch mType {
			case websocket.TextMessage:
				var rpcRes rpctypes.RPCResponse
				err = json.Unmarshal(p, &rpcRes)
				fmt.Println(rpcRes)
				if err != nil {
					printErrorAndExit("unmarshal RPCResponse failed", err)
				}
				switch broadcastTxMethod {
				case "broadcast_tx_async", "broadcast_tx_sync":
					var broadcastTx ctypes.ResultBroadcastTx
					err = srjson.Unmarshal(rpcRes.Result, &broadcastTx)
					if err != nil {
						printErrorAndExit("unmarshal RPCResponse.Result failed", err)
					}
					bom.recv <- broadcastTx.Hash

				case "broadcast_tx_commit":
					var broadcastTxCommit ctypes.ResultBroadcastTxCommit
					err = srjson.Unmarshal(rpcRes.Result, &broadcastTxCommit)
					if err != nil {
						printErrorAndExit("unmarshal RPCResponse.Result failed", err)
					}
					bom.recv <- broadcastTxCommit.DeliverTx.Events[0].Attributes[1].Value
				case "abci_query":
					var res ctypes.ResultABCIQuery
					err = srjson.Unmarshal(rpcRes.Result, &res)
					if err != nil {
						printErrorAndExit("unmarshal RPCResponse.Result failed", err)
					}
					fmt.Println(res.Response)
				}
			}
		}
	}
}

func generateTx(txNumber int, random *srrand.Rand) (json.RawMessage, []byte) {
	tx := make([]byte, tx_size)
	binary.PutUvarint(tx[:8], uint64(txNumber))
	binary.PutUvarint(tx[8:16], uint64(srtime.Now().Unix()))
	txHex := make([]byte, len(tx)*2)
	hex.Encode(txHex, tx)
	paramsJSON, err := json.Marshal(map[string]interface{}{"tx": txHex})
	if err != nil {
		printErrorAndExit("marshal tx failed", err)
	}
	return paramsJSON, txHex
}