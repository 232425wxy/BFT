package events

// EventCache 为可触发事件缓冲事件
// 所有事件都被缓存，在刷新时进行过滤
type EventCache struct {
	evsw   Fireable
	events []eventInfo
}

// NewEventCache 创建一个新的EventCache, EventSwitch作为后端
func NewEventCache(evsw Fireable) *EventCache {
	return &EventCache{
		evsw: evsw,
	}
}

type eventInfo struct {
	event string
	data  EventData
}

// FireEvent 缓存一个将在终结时触发的事件。
func (evc *EventCache) FireEvent(event string, data EventData) {
	// 追加到缓冲区的列表里
	evc.events = append(evc.events, eventInfo{event, data})
}

// Flush 通过evsw触发事件
// 清除缓存事件
func (evc *EventCache) Flush() {
	for _, ei := range evc.events {
		evc.evsw.FireEvent(ei.event, ei.data)
	}
	// 清除缓冲区，因为我们只使用append对它进行添加，所以把它设为nil是安全的，分配也是安全的
	evc.events = nil
}
