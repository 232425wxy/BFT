package pubsub

import (
	"BFT/libs/log"
	"BFT/libs/service"
	"context"
	"errors"
	"fmt"
	"sync"
)

type operation int

const (
	sub operation = iota
	pub
	unsub
	shutdown
)

var (
	// ErrSubscriptionNotFound 当客户端试图取消一个订阅时，却发现该订阅不存在，则此错误会被触发
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrAlreadySubscribed 当客户端试图订阅一个已经被订阅过的订阅时，此错误会被触发
	ErrAlreadySubscribed = errors.New("already subscribed")
)

type Query interface {
	Matches(events map[string][]string) (bool, error)
	String() string
}

type cmd struct {
	// 订阅/发布/取消订阅/关闭
	op operation

	// 订阅，不订阅
	query        Query
	subscription *Subscription
	clientID     string

	// 发布
	msg    interface{}
	events map[string][]string
}

type Server struct {
	service.BaseService

	cmds    chan cmd
	cmdsCap int // cmds 通道的容量

	// 在订阅与取消订阅之前，应当先检查是否有该订阅
	mtx sync.RWMutex
	// clientID -> query.str ->
	subscriptions map[string]map[string]struct{} // subscriber -> query (string) -> empty struct  {key1:{key:value}, key2:{key:value}}
}

type Option func(*Server)

func NewServer(options ...Option) *Server {
	s := &Server{
		subscriptions: make(map[string]map[string]struct{}),
	}
	s.BaseService = *service.NewBaseService(log.NewCRLogger("info").With("module", "pubsub"), "PubSub", s)

	for _, option := range options {
		option(s)
	}

	// if BufferCapacity option was not set, the channel is unbuffered
	s.cmds = make(chan cmd, s.cmdsCap)

	return s
}

func BufferCapacity(cap int) Option {
	return func(s *Server) {
		if cap > 0 {
			s.cmdsCap = cap
		}
	}
}

func (s *Server) BufferCapacity() int {
	return s.cmdsCap
}

// Subscribe 为给定的客户端创建一个订阅，
// 如果 context 已经取消了，或者clientID已经创建了该订阅，则会返回错误，
// outCapacity 被用作初始化 Subscription#out 通道的容量，默认情况下 为 1，
// 如果 outCapacity 小于或等于 0，会 panic，如果想创建无缓冲通道，则使用 SubscribeUnbuffered 方法
// outCapacity 是接受消息的通道的容量
func (s *Server) Subscribe(ctx context.Context, clientID string, query Query, outCapacity ...int) (*Subscription, error) {
	outCap := 1
	if len(outCapacity) > 0 {
		if outCapacity[0] <= 0 {
			panic("Negative or zero capacity. Use SubscribeUnbuffered if you want an unbuffered channel")
		}
		outCap = outCapacity[0]
	}

	return s.subscribe(ctx, clientID, query, outCap)
}

func (s *Server) SubscribeUnbuffered(ctx context.Context, clientID string, query Query) (*Subscription, error) {
	return s.subscribe(ctx, clientID, query, 0)
}

func (s *Server) subscribe(ctx context.Context, clientID string, query Query, outCapacity int) (*Subscription, error) {
	s.mtx.RLock()
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok { // clientID 在 server 处已经登记过了
		// 如果这里还 OK 的话，说明该订阅已经被 clientID 创建过了，后面就会返回已经被订阅过的错误
		_, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if ok {
		return nil, ErrAlreadySubscribed
	}

	// 创建一个新的订阅
	subscription := NewSubscription(outCapacity)
	select {
	case s.cmds <- cmd{op: sub, clientID: clientID, query: query, subscription: subscription}:
		s.mtx.Lock()
		if _, ok = s.subscriptions[clientID]; !ok {
			// 如果该 clientID 还未在 server 处登记过，则现在给它登记
			s.subscriptions[clientID] = make(map[string]struct{})
		}
		// 将创建的订阅交给 clientID
		s.subscriptions[clientID][query.String()] = struct{}{}
		s.mtx.Unlock()
		return subscription, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.Quit():
		return nil, nil
	}
}

// Unsubscribe 根据给定的 query 和 clientID，从 server 删除 clientID 过去订阅的 subscription，
// 如果对应的 subscription 不存在，或者 context 被取消，则会返回错误
func (s *Server) Unsubscribe(ctx context.Context, clientID string, query Query) error {
	s.mtx.RLock()
	// 从 server 处找到 clientID 的所有 subscription
	clientSubscriptions, ok := s.subscriptions[clientID]
	if ok {
		// 从 clientID 的所有 subscription 中找到 query 对应的那个 subscription
		_, ok = clientSubscriptions[query.String()]
	}
	s.mtx.RUnlock()
	if !ok {
		// 找不到对应的 subscription 就返回 “not found” 的错误
		return ErrSubscriptionNotFound
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID, query: query}:
		// 将取消 clientID 的 query 对应的 subscription 命令放入到 server 的命令集中
		s.mtx.Lock()
		delete(clientSubscriptions, query.String())
		if len(clientSubscriptions) == 0 {
			// 如果删除 subscription 后，clientID 在 server 处没有其他 subscription 了，则在 server 处删除该 clientID
			delete(s.subscriptions, clientID)
		}
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// UnsubscribeAll 取消 clientID 在 server 处的所有 subscription
func (s *Server) UnsubscribeAll(ctx context.Context, clientID string) error {
	s.mtx.RLock()
	_, ok := s.subscriptions[clientID]
	s.mtx.RUnlock()
	if !ok {
		return ErrSubscriptionNotFound
	}

	select {
	case s.cmds <- cmd{op: unsub, clientID: clientID}:
		s.mtx.Lock()
		delete(s.subscriptions, clientID)
		s.mtx.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// NumClients 返回在 server 处登记过得 client 个数
func (s *Server) NumClients() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return len(s.subscriptions)
}

// NumClientSubscriptions 给定一个 client 的 ID，查找它在 server 处有多少个 subscription
func (s *Server) NumClientSubscriptions(clientID string) int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return len(s.subscriptions[clientID])
}

// Publish 发布一个给定的消息，如果 context 被取消了，会返回一个错误
func (s *Server) Publish(ctx context.Context, msg interface{}) error {
	return s.PublishWithEvents(ctx, msg, make(map[string][]string))
}

// PublishWithEvents 将给定的消息和附带的 event 集合发布出去，event 集合用来和 client 的 query 进行匹配，
// 如果匹配上了，则该消息会被发送给指定的 client
func (s *Server) PublishWithEvents(ctx context.Context, msg interface{}, events map[string][]string) error {
	select {
	case s.cmds <- cmd{op: pub, msg: msg, events: events}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.Quit():
		return nil
	}
}

// OnStop 实现 service.Service 接口，通过向 server 的命令集中传入一个关闭的指令来关闭 server
func (s *Server) OnStop() {
	s.cmds <- cmd{op: shutdown}
}

// state 的 subscriptions 里存储了一个 query 对应多个 client 这样的数据结构
type state struct {
	// query string -> client -> subscription
	subscriptions map[string]map[string]*Subscription
	// query string -> queryPlusRefCount
	queries map[string]*queryPlusRefCount
}

// queryPlusRefCount 保存一个指向 query 和引用计数器的指针。当 refCount 为 0 时，query 将被删除。
type queryPlusRefCount struct {
	q        Query
	refCount int // refCount 引用计数器表示有多少个 client 引用了该 query，如果计数器为 0，说明没有 client 引用该 query
}

func (s *Server) OnStart() error {
	go s.loop(state{
		subscriptions: make(map[string]map[string]*Subscription),
		queries:       make(map[string]*queryPlusRefCount),
	})
	return nil
}

// OnReset 实现 service.Service 接口
func (s *Server) OnReset() error {
	return nil
}

func (s *Server) loop(state state) {
loop:
	for cmd := range s.cmds {
		switch cmd.op {
		case unsub:
			if cmd.query != nil {
				state.remove(cmd.clientID, cmd.query.String(), ErrUnsubscribed)
			} else {
				state.removeClient(cmd.clientID, ErrUnsubscribed)
			}
		case shutdown:
			state.removeAll(nil)
			break loop
		case sub:
			state.add(cmd.clientID, cmd.query, cmd.subscription)
		case pub:
			if err := state.send(cmd.msg, cmd.events); err != nil {
				s.Logger.Errorw("Error querying for events", "err", err)
			}
		}
	}
}

func (state *state) add(clientID string, q Query, subscription *Subscription) {
	qStr := q.String()

	if _, ok := state.subscriptions[qStr]; !ok {
		// 如果要添加的 query 还不存在，则为该 query 创建一个 map，用来存储 clientID 和其订阅的 subscriptions
		state.subscriptions[qStr] = make(map[string]*Subscription)
	}
	// 在 state 中为 query 添加一个 clientID 和其订阅的 subscriptions
	state.subscriptions[qStr][clientID] = subscription

	// 在 state 中为 query 初始化一个指向该 query 的 queryPlusRefCount
	if _, ok := state.queries[qStr]; !ok {
		state.queries[qStr] = &queryPlusRefCount{q: q, refCount: 0}
	}
	// 为该 query 的引用计数器加一
	state.queries[qStr].refCount++
}

func (state *state) remove(clientID string, qStr string, reason error) {
	clientSubscriptions, ok := state.subscriptions[qStr]
	if !ok {
		// state 中不存在该 query
		return
	}

	subscription, ok := clientSubscriptions[clientID]
	if !ok {
		// clientID 没有引用该 query
		return
	}

	subscription.cancel(reason)

	// 从 state 中将引用该 query 的指定 clientID 删除掉
	// 如果删除以后，没有 client 引用该 query，则从 state 中将该 query 删除掉
	delete(state.subscriptions[qStr], clientID)
	if len(state.subscriptions[qStr]) == 0 {
		delete(state.subscriptions, qStr)
	}

	// 对指向该 query 的引用计数器减一
	state.queries[qStr].refCount--
	// 如果引用计数器为 0，说明没有 client 引用该 query，则删除该 query
	if state.queries[qStr].refCount == 0 {
		delete(state.queries, qStr)
	}
}

// removeClient 从 state 那里删除所有 clientID 订阅的 subscription
func (state *state) removeClient(clientID string, reason error) {
	for qStr, clientSubscriptions := range state.subscriptions {
		if _, ok := clientSubscriptions[clientID]; ok {
			state.remove(clientID, qStr, reason)
		}
	}
}

// removeAll 清空 state
func (state *state) removeAll(reason error) {
	for qStr, clientSubscriptions := range state.subscriptions {
		for clientID := range clientSubscriptions {
			state.remove(clientID, qStr, reason)
		}
	}
}

func (state *state) send(msg interface{}, events map[string][]string) error {
	for qStr, clientSubscriptions := range state.subscriptions { // query -> client -> subscription
		// 遍历 state 里所有的 query，从 queries 里找到在 .subscriptions 处登记过得 query
		q := state.queries[qStr].q

		// 用 query 匹配以下 events 事件
		match, err := q.Matches(events)
		if err != nil {
			return fmt.Errorf("failed to match against query %s: %w", q.String(), err)
		}

		if match {
			// 如果匹配成功，则将 msg 消息发送给 query 下的所有客户端
			for clientID, subscription := range clientSubscriptions {
				if cap(subscription.out) == 0 {
					// 无缓冲的 Message#out 会堵塞住
					subscription.out <- NewMessage(msg, events)
				} else {
					// 非无缓冲的 Message#out，如果不能立刻往里面推送 消息 Message，则删除该订阅下的对应 client
					select {
					case subscription.out <- NewMessage(msg, events):
					default:
						state.remove(clientID, qStr, ErrOutOfCapacity)
					}
				}
			}
		}
	}
	return nil
}
