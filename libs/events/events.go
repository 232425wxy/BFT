package events

import (
	srlog "BFT/libs/log"
	"BFT/libs/service"
	"fmt"
	"sync"
)

// ErrListenerWasRemoved 如果 listener 被移除了，会报该错误
type ErrListenerWasRemoved struct {
	listenerID string
}

// Error 实现 error 接口
func (e ErrListenerWasRemoved) Error() string {
	return fmt.Sprintf("listener #%s was removed", e.listenerID)
}

// EventData 是一个通用的事件数据
type EventData interface{}

// Eventable 是接口反应器和其他模块必须导出成为 Eventable
type Eventable interface {
	SetEventSwitch(evsw EventSwitch)
}

// Fireable 是包装 FireEvent 方法的接口。
// FireEvent 用给定的名称和数据触发事件。
type Fireable interface {
	FireEvent(event string, data EventData)
}

// EventSwitch 是同步 pubsub 的接口，其中监听器订阅特定的事件，当事件被触发(参见Fireable)时，通过回调函数通知。
// 监听器是通过调用 AddListenerForEvent 函数添加的。
// 它们可以通过调用 RemoveListenerForEvent 或 RemoveListener(for all events) 来删除。
type EventSwitch interface {
	service.Service
	Fireable

	AddListenerForEvent(listenerID, event string, cb EventCallback) error
	RemoveListenerForEvent(event string, listenerID string)
	RemoveListener(listenerID string)
}

type eventSwitch struct {
	*service.BaseService

	mtx sync.RWMutex
	// eventCells：
	//	event_0 |=> listenerID_0 => EventCallback_0
	//	event_1 |=> listenerID_1 => EventCallback_1
	//	event_2 |=> listenerID_2 => EventCallback_2
	//	      |...
	eventCells map[string]*eventCell     // eventCells: event => *eventCell
	listeners  map[string]*eventListener // listeners: listenerID => *eventListener
}

func NewEventSwitch() EventSwitch {
	evsw := &eventSwitch{
		eventCells: make(map[string]*eventCell),
		listeners:  make(map[string]*eventListener),
	}
	evsw.BaseService = service.NewBaseService(srlog.NewCRLogger("info").With("module", "event-switch"), "EventSwitch", evsw)
	return evsw
}

func (evsw *eventSwitch) OnStart() error {
	return nil
}

func (evsw *eventSwitch) OnStop() {}

// 为特定的 event 添加 listener
func (evsw *eventSwitch) AddListenerForEvent(listenerID, event string, cb EventCallback) error {
	// 获取或者创建 ec 和 listener
	evsw.mtx.Lock()
	ec := evsw.eventCells[event]
	if ec == nil { // 该 event 不存在于 eventSwitch 中，因此也就不存在对应的 ec
		ec = newEventCell()
		evsw.eventCells[event] = ec
	}
	listener := evsw.listeners[listenerID]
	if listener == nil { // 该 listenerID 不存在于 eventSwitch 中，因此也就不存在对应的 listener
		listener = newEventListener(listenerID)
		evsw.listeners[listenerID] = listener
	}
	evsw.mtx.Unlock()

	// 往 eventSwitch 里添加 listener 和 event
	if err := listener.AddEvent(event); err != nil { // 该监听器为自己添加 event
		return err
	}
	ec.AddListener(listenerID, cb) // 该 event 为自己添加一个监听器和对应的回调函数

	return nil
}

func (evsw *eventSwitch) RemoveListener(listenerID string) {
	// 获取然后删除对应的 listener
	evsw.mtx.RLock()
	listener := evsw.listeners[listenerID]
	evsw.mtx.RUnlock()
	if listener == nil {
		return
	}

	evsw.mtx.Lock()
	delete(evsw.listeners, listenerID)
	evsw.mtx.Unlock()

	listener.SetRemoved()
	for _, event := range listener.GetEvents() {
		evsw.RemoveListenerForEvent(event, listenerID) // 将 eventSwitch 里与该 listener 有关的 event 的回调函数删除掉
	}
}

func (evsw *eventSwitch) RemoveListenerForEvent(event string, listenerID string) {
	// Get eventCell
	evsw.mtx.Lock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.Unlock()

	if eventCell == nil {
		return
	}

	// RemoveConn listenerID from eventCell
	numListeners := eventCell.RemoveListener(listenerID)

	// Maybe garbage collect eventCell.
	if numListeners == 0 {
		// Lock again and double check.
		evsw.mtx.Lock()      // OUTER LOCK
		eventCell.mtx.Lock() // INNER LOCK
		// 如果没有监听器在监听该 eventCell，那么就将其从 eventSwitch 中删除掉
		if len(eventCell.listeners) == 0 {
			delete(evsw.eventCells, event)
		}
		eventCell.mtx.Unlock() // INNER LOCK
		evsw.mtx.Unlock()      // OUTER LOCK
	}
}

func (evsw *eventSwitch) FireEvent(event string, data EventData) {
	// Get the eventCell
	evsw.mtx.RLock()
	eventCell := evsw.eventCells[event]
	evsw.mtx.RUnlock()

	if eventCell == nil {
		return
	}

	// Fire event for all listeners in eventCell
	eventCell.FireEvent(data)
}

//-----------------------------------------------------------------------------

// eventCell 具有 FireEvent 方法，处理相关回调函数
type eventCell struct {
	mtx       sync.RWMutex
	listeners map[string]EventCallback // listeners: listenerID => EventCallback
}

func newEventCell() *eventCell {
	return &eventCell{
		listeners: make(map[string]EventCallback),
	}
}

// 往 eventCell 的 listeners 中添加 listenerID:EventCallback
func (cell *eventCell) AddListener(listenerID string, cb EventCallback) {
	cell.mtx.Lock()
	cell.listeners[listenerID] = cb
	cell.mtx.Unlock()
}

// RemoveListener 删除指定的 listener，并返回剩下的 listener 个数
func (cell *eventCell) RemoveListener(listenerID string) int {
	cell.mtx.Lock()
	delete(cell.listeners, listenerID)
	numListeners := len(cell.listeners)
	cell.mtx.Unlock()
	return numListeners
}

// eventCell 的 listeners 字段存储了一些 EventCallback 回调函数，
// 依次调用这些回调函数，并往这些函数里传递 data 入参，执行回调函数
func (cell *eventCell) FireEvent(data EventData) {
	cell.mtx.RLock()
	eventCallbacks := make([]EventCallback, 0, len(cell.listeners))
	for _, cb := range cell.listeners {
		eventCallbacks = append(eventCallbacks, cb)
	}
	cell.mtx.RUnlock()

	for _, cb := range eventCallbacks {
		cb(data)
	}
}

//-----------------------------------------------------------------------------

type EventCallback func(data EventData)

type eventListener struct {
	listenerID string

	mtx     sync.RWMutex
	removed bool
	events  []string
}

func newEventListener(id string) *eventListener {
	return &eventListener{
		listenerID: id,
		removed:    false,
		events:     nil, // AddEvent 里的 append 可以为 nil 的 events 开辟空间
	}
}

// 为 eventListener 添加新的 event
func (evl *eventListener) AddEvent(event string) error {
	evl.mtx.Lock()

	if evl.removed {
		// 不能给已经移除的 listener 添加 event
		evl.mtx.Unlock()
		return ErrListenerWasRemoved{listenerID: evl.listenerID}
	}

	// 往 eventListener 的 events 中添加新的 event
	evl.events = append(evl.events, event)
	evl.mtx.Unlock()
	return nil
}

// events 是若干字符串
func (evl *eventListener) GetEvents() []string {
	evl.mtx.RLock()
	events := make([]string, len(evl.events))
	copy(events, evl.events)
	evl.mtx.RUnlock()
	return events
}

func (evl *eventListener) SetRemoved() {
	evl.mtx.Lock()
	evl.removed = true
	evl.mtx.Unlock()
}
