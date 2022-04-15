package clist

import (
	"fmt"
	"sync"
)

/*

CList 的目的是提供一个goroutine-safe的链表。
这个列表可以被任意数量的goroutines同时遍历。
然而，移除的celement不能被添加回来。
注意:不是所有的容器/列表方法都被实现了。
注意:被移除的元素需要始终使用DetachPrev或DetachNext，以确保被移除元素被垃圾回收。

*/

// MaxLength 是一个链表允许包含的最大元素数。
// 如果更多的元素被推到列表中，它将会panic。
const MaxLength = int(^uint(0) >> 1)

type CElement struct {
	mtx        sync.RWMutex
	prev       *CElement
	prevWg     *sync.WaitGroup
	prevWaitCh chan struct{}
	next       *CElement
	nextWg     *sync.WaitGroup
	nextWaitCh chan struct{}
	removed    bool

	Value interface{} // 不可更改
}

// NextWait 有阻塞的返回下一个元素，如果该元素没有被删除，且该元素的下一个元素为 nil，
// 则一直等到该元素的下一个元素不为 nil，然后返回下一个元素
func (e *CElement) NextWait() *CElement {
	for {
		e.mtx.RLock()
		next := e.next
		nextWg := e.nextWg
		removed := e.removed
		e.mtx.RUnlock()

		if next != nil || removed { // 如果该元素的下一个元素不为空，或者该元素被移除了，那么直接返回下一个元素
			return next
		}

		nextWg.Wait()
	}
}

// PrevWait 有阻塞的返回前一个元素，如果该元素没有被删除，且该元素的前一个元素为 nil，
// 则一直等到该元素的前一个元素不为 nil，然后返回前一个元素
func (e *CElement) PrevWait() *CElement {
	for {
		e.mtx.RLock()
		prev := e.prev
		prevWg := e.prevWg
		removed := e.removed
		e.mtx.RUnlock()

		if prev != nil || removed {
			return prev
		}

		prevWg.Wait()
	}
}

// PrevWaitChan 可被用来等待直到该元素的前一个元素不为 nil，一旦该元素的前一个元素不为 nil，那么
// 此 channel 就会被关闭
func (e *CElement) PrevWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.prevWaitCh
}

// NextWaitChan 可被用来等待直到该元素的下一个元素不为 nil，一旦该元素的下一个元素不为 nil，那么
// 此 channel 就会被关闭
func (e *CElement) NextWaitChan() <-chan struct{} {
	e.mtx.RLock()
	defer e.mtx.RUnlock()

	return e.nextWaitCh
}

// Next 无阻塞的返回该元素的下一个元素，如果该元素
// 是最后一个元素，则返回的值为 nil
func (e *CElement) Next() *CElement {
	e.mtx.RLock()
	val := e.next
	e.mtx.RUnlock()
	return val
}

// Prev 无阻塞的返回该元素的前一个元素，如果该元素
// 是第一个元素，则返回的值为 nil
func (e *CElement) Prev() *CElement {
	e.mtx.RLock()
	prev := e.prev
	e.mtx.RUnlock()
	return prev
}

// Removed 返回该元素是否被移除
func (e *CElement) Removed() bool {
	e.mtx.RLock()
	isRemoved := e.removed
	e.mtx.RUnlock()
	return isRemoved
}

// DetachNext 该元素与下一个元素直接脱钩，但是如果该元素还未被移除就与下一个元素脱钩，会 panic
func (e *CElement) DetachNext() {
	e.mtx.Lock()
	if !e.removed {
		e.mtx.Unlock()
		panic("DetachNext() must be called after RemoveConn(e)")
	}
	e.next = nil
	e.mtx.Unlock()
}

// DetachPrev 该元素与前一个元素直接脱钩，但是如果该元素还未被移除就与前一个元素脱钩，会 panic
func (e *CElement) DetachPrev() {
	e.mtx.Lock()
	if !e.removed {
		e.mtx.Unlock()
		panic("DetachPrev() must be called after RemoveConn(e)")
	}
	e.prev = nil
	e.mtx.Unlock()
}

// SetNext
// 由于此时该元素的下一个元素为 nil，因此为下一个元素设置一个 WaitGroup，这样就可以在后续想要获取该元素的下一个元素的时候被阻塞住
// 由于此时下一个元素不为 nil，因此就让下一个元素的 WaitGroup done，并且把等待下一个元素的 channel 关了，这样在后面想要获取该元素的下一个元素时可以直接获取到
func (e *CElement) SetNext(newNext *CElement) {
	e.mtx.Lock()

	oldNext := e.next                     // 之前的下一个元素
	e.next = newNext                      // 重新设置下一个元素
	if oldNext != nil && newNext == nil { // 如果之前下一个元素不为 nil，而现在下一个元素是 nil 的
		// 由于此时该元素的下一个元素为 nil，因此为下一个元素设置一个 WaitGroup，这样
		// 就可以在后续想要获取该元素的下一个元素的时候被阻塞住
		e.nextWg = waitGroup1()
		e.nextWaitCh = make(chan struct{})
	}
	if oldNext == nil && newNext != nil { // 如果之前下一个元素为 nil，而现在下一个元素不是 nil 的
		// 由于此时下一个元素不为 nil，因此就让下一个元素的 WaitGroup done，并且把等待下一个元素的 channel
		// 关了，这样在后面想要获取该元素的下一个元素时可以直接获取到
		e.nextWg.Done()
		close(e.nextWaitCh)
	}
	e.mtx.Unlock()
}

// SetPrev
// 由于此时该元素的前一个元素为 nil，因此为前一个元素设置一个 WaitGroup，这样就可以在后续想要获取该元素的前一个元素的时候被阻塞住
// 由于此时前一个元素不为 nil，因此就让前一个元素的 WaitGroup done，并且把等待前一个元素的 channel 关了，这样在后面想要获取该元素的前一个元素时可以直接获取到
func (e *CElement) SetPrev(newPrev *CElement) {
	e.mtx.Lock()

	oldPrev := e.prev
	e.prev = newPrev
	if oldPrev != nil && newPrev == nil { // 如果之前前一个元素不为 nil，而现在前一个元素是 nil 的
		// 由于此时该元素的前一个元素为 nil，因此为前一个元素设置一个 WaitGroup，这样
		// 就可以在后续想要获取该元素的前一个元素的时候被阻塞住
		e.prevWg = waitGroup1() // WaitGroups are difficult to re-use.
		e.prevWaitCh = make(chan struct{})
	}
	if oldPrev == nil && newPrev != nil {
		// 由于此时前一个元素不为 nil，因此就让前一个元素的 WaitGroup done，并且把等待前一个元素的 channel
		// 关了，这样在后面想要获取该元素的前一个元素时可以直接获取到
		e.prevWg.Done()
		close(e.prevWaitCh)
	}
	e.mtx.Unlock()
}

// SetRemoved 删除元素
func (e *CElement) SetRemoved() {
	e.mtx.Lock()

	e.removed = true

	// 唤醒任何等待获取该元素前一个或者后一个元素的 goroutine
	if e.prev == nil {
		e.prevWg.Done()
		close(e.prevWaitCh)
	}
	if e.next == nil {
		e.nextWg.Done()
		close(e.nextWaitCh)
	}
	e.mtx.Unlock()
}

//--------------------------------------------------------------------------------

// CList 表示一个链表。
// CList 的 0 值是一个可用的空列表。
// 操作是 goroutine-safe。
// 如果长度超过最大值，则会恐慌。
type CList struct {
	mtx    sync.RWMutex
	wg     *sync.WaitGroup
	waitCh chan struct{}
	head   *CElement // 第一个元素
	tail   *CElement // 最后一个元素
	len    int       // 链表长度
	maxLen int       // 链表的最大长度
}

func (l *CList) Init() *CList {
	l.mtx.Lock()

	l.wg = waitGroup1()
	l.waitCh = make(chan struct{})
	l.head = nil
	l.tail = nil
	l.len = 0
	l.mtx.Unlock()
	return l
}

// New 新建一个 CList，并设置 CList 的最大长度
func New() *CList { return newWithMax(MaxLength) }

func newWithMax(maxLength int) *CList {
	l := new(CList)
	l.maxLen = maxLength
	return l.Init()
}

// Len 返回链表的当前长度
func (l *CList) Len() int {
	l.mtx.RLock()
	len := l.len
	l.mtx.RUnlock()
	return len
}

// Front 无阻塞的返回链表的第一个元素
func (l *CList) Front() *CElement {
	l.mtx.RLock()
	head := l.head
	l.mtx.RUnlock()
	return head
}

// FrontWait 有阻塞的返回链表的第一个元素
func (l *CList) FrontWait() *CElement {
	for {
		l.mtx.RLock()
		head := l.head
		wg := l.wg
		l.mtx.RUnlock()

		if head != nil {
			return head
		}
		wg.Wait()
	}
}

// Back 无阻塞的返回链表的最后一个元素
func (l *CList) Back() *CElement {
	l.mtx.RLock()
	back := l.tail
	l.mtx.RUnlock()
	return back
}

// BackWait 有阻塞的返回链表的最后一个元素
func (l *CList) BackWait() *CElement {
	for {
		l.mtx.RLock()
		tail := l.tail
		wg := l.wg
		l.mtx.RUnlock()

		if tail != nil {
			return tail
		}
		wg.Wait()
	}
}

// WaitChan 可被用来等待直到链表的第一元素或者最后一个元素不为 nil 的 channel，
// 如果链表的最后一个元素或者第一个元素不为 nil，那么该 channel 就会被关闭
func (l *CList) WaitChan() <-chan struct{} {
	l.mtx.Lock()
	defer l.mtx.Unlock()

	return l.waitCh
}

// PushBack 往链表的末尾追加新的元素
func (l *CList) PushBack(v interface{}) *CElement {
	l.mtx.Lock()

	// Construct a new element
	e := &CElement{
		prev:       nil,
		prevWg:     waitGroup1(),
		prevWaitCh: make(chan struct{}),
		next:       nil,
		nextWg:     waitGroup1(),
		nextWaitCh: make(chan struct{}),
		removed:    false,
		Value:      v,
	}

	// 如果在追加新的元素之前，链表是空链表，那么让链表的 WaitGroup done，并且关闭等待的 channel
	if l.len == 0 {
		l.wg.Done()
		close(l.waitCh)
	}
	if l.len >= l.maxLen {
		panic(fmt.Sprintf("clist: maximum length list reached %d", l.maxLen))
	}
	l.len++

	if l.tail == nil { // 如果 l 里面还没有元素
		l.head = e
		l.tail = e
	} else {
		e.SetPrev(l.tail)
		l.tail.SetNext(e)
		l.tail = e
	}
	l.mtx.Unlock()
	return e
}

// Remove 删除链表里的元素
func (l *CList) Remove(e *CElement) interface{} {
	l.mtx.Lock()

	prev := e.Prev()
	next := e.Next()

	if l.head == nil || l.tail == nil {
		l.mtx.Unlock()
		panic("RemoveConn(e) on empty CList")
	}
	if prev == nil && l.head != e {
		l.mtx.Unlock()
		panic("RemoveConn(e) with false head")
	}
	if next == nil && l.tail != e {
		l.mtx.Unlock()
		panic("RemoveConn(e) with false tail")
	}

	// 如果我们删除的是链表里的唯一一个元素，那么就让 FrontWait 和 BackWait 等待
	if l.len == 1 {
		l.wg = waitGroup1()
		l.waitCh = make(chan struct{})
	}

	l.len--

	if prev == nil {
		l.head = next
	} else {
		prev.SetNext(next)
	}
	if next == nil {
		l.tail = prev
	} else {
		next.SetPrev(prev)
	}

	e.SetRemoved()

	l.mtx.Unlock()
	return e.Value
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return
}
