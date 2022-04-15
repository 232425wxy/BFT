package pubsub

import (
	"errors"
	"sync"
)

var (
	// ErrUnsubscribed 当客户端没有订阅的时候
	ErrUnsubscribed = errors.New("client unsubscribed")

	ErrOutOfCapacity = errors.New("client is not pulling messages fast enough")
)

// Message 将数据与事件绑定在一起
type Message struct {
	data   interface{}
	events map[string][]string
}

func NewMessage(data interface{}, events map[string][]string) Message {
	return Message{data, events}
}

// Data 返回发布的原始数据
func (msg Message) Data() interface{} {
	return msg.data
}

// Events 返回与客户端查询匹配的事件
func (msg Message) Events() map[string][]string {
	return msg.events
}

// A Subscription 表示一个特定查询的客户端订阅，包含以下三个内容
//	1. 将消息事件发布到其上的通道
//	2. 如果客户端太慢或选择取消订阅，指示通道将关闭的字段
//	3. 指示通道关闭的原因的字段
type Subscription struct {
	out       chan Message
	cancelled chan struct{}
	mtx       sync.RWMutex
	err       error
}

// NewSubscription 根据给定的缓冲值，创建一个带缓冲的消息通道，返回一个新的订阅
func NewSubscription(outCapacity int) *Subscription {
	return &Subscription{
		out:       make(chan Message, outCapacity),
		cancelled: make(chan struct{}),
	}
}

// Out 返回一个事件与消息都发布到其上的一个通道
func (s *Subscription) Out() <-chan Message {
	return s.out
}

// Cancelled 返回一个指示当订阅被停止通道是否关闭的通道
func (s *Subscription) Cancelled() <-chan struct{} {
	return s.cancelled
}

func (s *Subscription) Err() error {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.err
}

func (s *Subscription) cancel(err error) {
	s.mtx.Lock()
	s.err = err
	s.mtx.Unlock()
	close(s.cancelled)
}
