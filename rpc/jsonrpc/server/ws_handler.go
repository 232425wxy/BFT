package server

import (
	"github.com/232425wxy/BFT/libs/log"
	"github.com/232425wxy/BFT/libs/service"
	"github.com/232425wxy/BFT/rpc/jsonrpc/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"reflect"
	"runtime/debug"
	"time"
)

// WebSocket handler

const (
	defaultWSWriteChanCapacity = 100
	defaultWSWriteWait         = 30 * time.Second
	defaultWSReadWait          = 30 * time.Second
	defaultWSPingPeriod        = (defaultWSReadWait * 9) / 10
)

// WebsocketManager 为传入的连接提供了一个 WS 处理程序，并将函数映射和任何附加参数传递给新连接，
// 注意:websocket 路径在外部定义，例如在 node/node.go 中
type WebsocketManager struct {
	// Upgrader 指定将 HTTP 连接升级到 WebSocket 连接的参数。
	websocket.Upgrader

	funcMap       map[string]*RPCFunc
	logger        log.CRLogger
	wsConnOptions []func(*wsConnection)
}

// NewWebsocketManager 传入的参数 funcMap 实际上就是 /rpc/core/types.Routes
func NewWebsocketManager(
	funcMap map[string]*RPCFunc,
	wsConnOptions ...func(*wsConnection),
) *WebsocketManager {
	return &WebsocketManager{
		funcMap: funcMap,
		Upgrader: websocket.Upgrader{
			// 如果请求的 Origin 报头是可接受的，CheckOrigin 返回 true，如果 CheckOrigin 为 nil，则使用安全的默认值：
			// 如果 Origin 请求头存在并且源主机不等于请求主机头，则返回 false
			// CheckOrigin 函数应该仔细地验证请求源，以防止跨站点的伪造请求
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		logger:        log.NewCRLogger("INFO"),
		wsConnOptions: wsConnOptions,
	}
}

// SetLogger sets the logger.
func (wm *WebsocketManager) SetLogger(l log.CRLogger) {
	wm.logger = l
}

// WebsocketHandler upgrades the request/response (via http.Hijack) and starts
// the wsConnection.
func (wm *WebsocketManager) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wm.Upgrade(w, r, nil)
	if err != nil {
		wm.logger.Errorw("Failed to upgrade connection", "err", err)
		return
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			wm.logger.Errorw("Failed to close connection", "err", err)
		}
	}()

	// register connection
	con := newWSConnection(wsConn, wm.funcMap, wm.wsConnOptions...)
	con.SetLogger(wm.logger.With("remote", wsConn.RemoteAddr()))
	wm.logger.Infow("New websocket connection", "remote", con.remoteAddr)
	err = con.Start() // BLOCKING
	if err != nil {
		wm.logger.Errorw("Failed to start connection", "err", err)
		return
	}
	if err := con.Stop(); err != nil {
		wm.logger.Errorw("error while stopping connection", "error", err)
	}
}

// WebSocket connection

// A single websocket connection contains listener id, underlying ws
// connection, and the event switch for subscribing to events.
//
// In case of an error, the connection is stopped.
type wsConnection struct {
	service.BaseService

	remoteAddr string
	baseConn   *websocket.Conn
	// writeChan is never closed, to allow WriteRPCResponse() to fail.
	writeChan chan types.RPCResponse

	// chan, which is closed when/if readRoutine errors
	// used to abort writeRoutine
	readRoutineQuit chan struct{}

	funcMap map[string]*RPCFunc

	// write channel capacity
	writeChanCapacity int

	// each write times out after this.
	writeWait time.Duration

	// Connection times out if we haven't received *anything* in this long, not even pings.
	readWait time.Duration

	// Send pings to server with this period. Must be less than readWait, but greater than zero.
	pingPeriod time.Duration

	// Maximum message size.
	readLimit int64

	// callback which is called upon disconnect
	onDisconnect func(remoteAddr string)

	ctx    context.Context
	cancel context.CancelFunc
}

// NewWSConnection wraps websocket.Conn.
//
// See the commentary on the func(*wsConnection) functions for a detailed
// description of how to configure ping period and pong wait time. NOTE: if the
// write buffer is full, pongs may be dropped, which may cause clients to
// disconnect. see https://github.com/gorilla/websocket/issues/97
func newWSConnection(
	baseConn *websocket.Conn,
	funcMap map[string]*RPCFunc,
	options ...func(*wsConnection),
) *wsConnection {
	wsc := &wsConnection{
		remoteAddr:        baseConn.RemoteAddr().String(),
		baseConn:          baseConn,
		funcMap:           funcMap,
		writeWait:         defaultWSWriteWait,
		writeChanCapacity: defaultWSWriteChanCapacity,
		readWait:          defaultWSReadWait,
		pingPeriod:        defaultWSPingPeriod,
		readRoutineQuit:   make(chan struct{}),
	}
	for _, option := range options {
		option(wsc)
	}
	wsc.baseConn.SetReadLimit(wsc.readLimit)
	wsc.BaseService = *service.NewBaseService(nil, "wsConnection", wsc)
	return wsc
}

// OnDisconnect sets a callback which is used upon disconnect - not
// Goroutine-safe. Nop by default.
func OnDisconnect(onDisconnect func(remoteAddr string)) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.onDisconnect = onDisconnect
	}
}

// ReadLimit sets the maximum size for reading message.
// It should only be used in the constructor - not Goroutine-safe.
func ReadLimit(readLimit int64) func(*wsConnection) {
	return func(wsc *wsConnection) {
		wsc.readLimit = readLimit
	}
}

// OnStart implements service.Service by starting the read and write routines. It
// blocks until there's some error.
func (wsc *wsConnection) OnStart() error {
	wsc.writeChan = make(chan types.RPCResponse, wsc.writeChanCapacity)

	// Read subscriptions/unsubscriptions to events
	go wsc.readRoutine()
	// Write responses, BLOCKING.
	wsc.writeRoutine()

	return nil
}

// OnStop implements service.Service by unsubscribing remoteAddr from all
// subscriptions.
func (wsc *wsConnection) OnStop() {
	if wsc.onDisconnect != nil {
		wsc.onDisconnect(wsc.remoteAddr)
	}

	if wsc.ctx != nil {
		wsc.cancel()
	}
}

// GetRemoteAddr returns the remote address of the underlying connection.
// It implements WSRPCConnection
func (wsc *wsConnection) GetRemoteAddr() string {
	return wsc.remoteAddr
}

// WriteRPCResponse pushes a response to the writeChan, and blocks until it is
// accepted.
// It implements WSRPCConnection. It is Goroutine-safe.
func (wsc *wsConnection) WriteRPCResponse(ctx context.Context, resp types.RPCResponse) error {
	select {
	case <-wsc.Quit():
		return errors.New("connection was stopped")
	case <-ctx.Done():
		return ctx.Err()
	case wsc.writeChan <- resp:
		return nil
	}
}

// TryWriteRPCResponse attempts to push a response to the writeChan, but does
// not block.
// It implements WSRPCConnection. It is Goroutine-safe
func (wsc *wsConnection) TryWriteRPCResponse(resp types.RPCResponse) bool {
	select {
	case <-wsc.Quit():
		return false
	case wsc.writeChan <- resp:
		return true
	default:
		return false
	}
}

// Context returns the connection's context.
// The context is canceled when the client's connection closes.
func (wsc *wsConnection) Context() context.Context {
	if wsc.ctx != nil {
		return wsc.ctx
	}
	wsc.ctx, wsc.cancel = context.WithCancel(context.Background())
	return wsc.ctx
}

// Read from the socket and subscribe to or unsubscribe from events
func (wsc *wsConnection) readRoutine() {
	// readRoutine 将阻塞，直到写响应或 WS 连接关闭
	writeCtx := context.Background()

	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("WSJSONRPC: %v", r)
			}
			wsc.Logger.Errorw("Panic in WSJSONRPC handler", "err", err, "stack", string(debug.Stack()))
			if err := wsc.WriteRPCResponse(writeCtx, types.RPCInternalError(types.JSONRPCIntID(-1), err)); err != nil {
				wsc.Logger.Errorw("Error writing RPC response", "err", err)
			}
			go wsc.readRoutine()
		}
	}()

	// 当从 peer 收到 pong 消息后，设置一个 handler 来处理 pong 消息
	wsc.baseConn.SetPongHandler(func(m string) error {
		return wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait))
	})

	for {
		select {
		case <-wsc.Quit():
			return
		default:
			// reset deadline for every type of message (control or data)
			if err := wsc.baseConn.SetReadDeadline(time.Now().Add(wsc.readWait)); err != nil {
				wsc.Logger.Errorw("failed to set read deadline", "err", err)
			}

			// NextReader返回从对端收到的下一个数据消息。返回的messageType为TextMessage或BinaryMessage。
			// 在一个连接上最多只能有一个打开的阅读器。如果应用程序尚未使用前一条消息，则NextReader将丢弃该消息。
			// 当此方法返回非nil错误值时，应用程序必须跳出应用程序的读循环。从该方法返回的错误是永久的。一旦该方法
			// 返回一个非空错误，对该方法的所有后续调用都将返回相同的错误。
			_, r, err := wsc.baseConn.NextReader()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					wsc.Logger.Infow("Client closed the connection")
				} else {
					wsc.Logger.Errorw("Failed to read request", "err", err)
				}
				if err := wsc.Stop(); err != nil {
					wsc.Logger.Errorw("Error closing websocket connection", "err", err)
				}
				close(wsc.readRoutineQuit)
				return
			}

			dec := json.NewDecoder(r)
			var request types.RPCRequest
			err = dec.Decode(&request)
			if err != nil {
				if err := wsc.WriteRPCResponse(writeCtx,
					types.RPCParseError(fmt.Errorf("error unmarshaling request: %w", err))); err != nil {
					wsc.Logger.Errorw("Error writing RPC response", "err", err)
				}
				continue
			}

			// A Notification is a Request object without an "id" member.
			// The Server MUST NOT reply to a Notification, including those that are within a batch request.
			if request.ID == nil {
				wsc.Logger.Debugw(
					"WSJSONRPC received a notification, skipping... (please send a non-empty ID if you want to call a method)",
					"req", request,
				)
				continue
			}

			// Now, fetch the RPCFunc and execute it.
			rpcFunc := wsc.funcMap[request.Method]
			if rpcFunc == nil {
				if err := wsc.WriteRPCResponse(writeCtx, types.RPCMethodNotFoundError(request.ID)); err != nil {
					wsc.Logger.Errorw("Error writing RPC response", "err", err)
				}
				continue
			}

			ctx := &types.Context{JSONReq: &request, WSConn: wsc}
			args := []reflect.Value{reflect.ValueOf(ctx)}
			if len(request.Params) > 0 {
				fnArgs, err := jsonParamsToArgs(rpcFunc, request.Params)
				if err != nil {
					if err := wsc.WriteRPCResponse(writeCtx,
						types.RPCInternalError(request.ID, fmt.Errorf("error converting json params to arguments: %w", err)),
					); err != nil {
						wsc.Logger.Errorw("Error writing RPC response", "err", err)
					}
					continue
				}
				args = append(args, fnArgs...)
			}

			returns := rpcFunc.f.Call(args)

			//wsc.Logger.Info("WSJSONRPC", "method", request.Method)

			result, err := unreflectResult(returns)
			if err != nil {
				if err := wsc.WriteRPCResponse(writeCtx, types.RPCInternalError(request.ID, err)); err != nil {
					wsc.Logger.Errorw("Error writing RPC response", "err", err)
				}
				continue
			}

			if err := wsc.WriteRPCResponse(writeCtx, types.NewRPCSuccessResponse(request.ID, result)); err != nil {
				wsc.Logger.Errorw("Error writing RPC response", "err", err)
			}
		}
	}
}

// receives on a write channel and writes out on the socket
func (wsc *wsConnection) writeRoutine() {
	pingTicker := time.NewTicker(wsc.pingPeriod)
	defer pingTicker.Stop()

	pongs := make(chan string, 1)
	wsc.baseConn.SetPingHandler(func(m string) error {
		select {
		case pongs <- m:
		default:
		}
		return nil
	})

	for {
		select {
		case <-wsc.Quit():
			return
		case <-wsc.readRoutineQuit: // error in readRoutine
			return
		case m := <-pongs:
			err := wsc.writeMessageWithDeadline(websocket.PongMessage, []byte(m))
			if err != nil {
				wsc.Logger.Infow("Failed to write pong (client may disconnect)", "err", err)
			}
		case <-pingTicker.C:
			err := wsc.writeMessageWithDeadline(websocket.PingMessage, []byte{})
			if err != nil {
				wsc.Logger.Errorw("Failed to write ping", "err", err)
				return
			}
		case msg := <-wsc.writeChan:
			jsonBytes, err := json.MarshalIndent(msg, "", "  ")
			if err != nil {
				wsc.Logger.Errorw("Failed to marshal RPCResponse to JSON", "err", err)
				continue
			}
			if err = wsc.writeMessageWithDeadline(websocket.TextMessage, jsonBytes); err != nil {
				wsc.Logger.Errorw("Failed to write response", "err", err, "msg", msg)
				return
			}
		}
	}
}

// 所有写入 websocket 的操作都必须(重新)设置写入超时时间，如果一些写操作没有设置它，而另一些设置了，它们可能会超时
func (wsc *wsConnection) writeMessageWithDeadline(msgType int, msg []byte) error {
	// SetWriteDeadline 设置底层网络连接的写超时时间，当一次写操作超时后，websocket状态就会被破坏，
	// 以后所有的写操作都会返回一个错误。如果传入的超时时间 t 等于 0 则意味着写入不会超时
	if err := wsc.baseConn.SetWriteDeadline(time.Now().Add(wsc.writeWait)); err != nil {
		return err
	}
	return wsc.baseConn.WriteMessage(msgType, msg)
}
