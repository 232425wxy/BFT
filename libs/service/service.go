package service

import (
	"BFT/libs/log"
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	// ErrAlreadyStarted 当尝试启动一个正在运行的服务时，会报告此错误
	ErrAlreadyStarted = errors.New("already started")
	// ErrAlreadyStopped 当尝试关闭一个已经关闭的服务时，会报告此错误
	ErrAlreadyStopped = errors.New("already stopped")
	// ErrNotStarted 当尝试关闭一个没有运行的服务时，会报告此错误
	ErrNotStarted = errors.New("not started")
)

// Service 是后面很多组件的基石，可以开启、停止和重置
type Service interface {
	// Start the service.
	// 如果服务已经开启了或者被停止了，调用 Start() 方法会返回错误
	// 如果 OnStart() 返回一个 error, 那么该 error 是由 Start() 返回的
	Start() error
	OnStart() error

	// Stop the service.
	// 如果服务已经停止了，调用 Stop() 方法会返回一个 error
	// OnStop() 必须保证不会出错
	Stop() error
	OnStop()

	// Reset the service.
	Reset() error
	OnReset() error

	// IsRunning 如果服务正在运行，那么会返回 true
	IsRunning() bool

	// Quit 返回一个 channel，该 channel 会在服务停止时被关闭
	Quit() <-chan struct{}

	// String 用一串字符串表示该服务
	String() string

	// SetLogger 设置一个 logger
	SetLogger(logger log.CRLogger)
}

/*
BaseService

Classical-inheritance-style服务声明。服务可以启动，然后停止，然后选择性地重新启动。

用户可以重写 OnStart / OnStop 方法。在没有错误的情况下，保证这些方法最多被调用一次。
如果 OnStart 返回一个错误，服务将不会被标记为 started，所以用户可以再次调用 Start。

调用 Reset 将会 panic，除非 OnReset 被重写，允许再次调用 OnStart / OnStop

调用者必须确保 Start 和 Stop 不是同时调用的。

你可以先调用 Stop 而不调用 Start
*/
type BaseService struct {
	Logger  log.CRLogger
	name    string
	started uint32 // 原子操作
	stopped uint32 // 原子操作
	quit    chan struct{}

	impl Service // 真正的服务组件
}

// NewBaseService 创建一个新的 BaseService
func NewBaseService(logger log.CRLogger, name string, impl Service) *BaseService {
	return &BaseService{
		Logger: logger,
		name:   name,
		quit:   make(chan struct{}),
		impl:   impl,
	}
}

// SetLogger 通过设置 logger 来实现 Service 接口
func (bs *BaseService) SetLogger(l log.CRLogger) {
	bs.Logger = l
}

// Start 通过调用 OnStart 来实现 Service 接口。如果服务已经开启或者关闭，则会返回一个 error。
// 在重新开启一个被关闭的服务前，需要先调用 Reset 方法
func (bs *BaseService) Start() error {
	if atomic.CompareAndSwapUint32(&bs.started, 0, 1) { // 不能开启一个已经运行的服务
		if atomic.LoadUint32(&bs.stopped) == 1 { // 不能开启一个已经被关闭的服务
			bs.Logger.Errorw(fmt.Sprintf("Not starting %v service -- already stopped", bs.name),
				"impl", bs.impl)
			// 回滚
			atomic.StoreUint32(&bs.started, 0)
			return ErrAlreadyStopped
		}
		bs.Logger.Infow(fmt.Sprintf("Starting %v service", bs.name), "impl", bs.impl.String())
		err := bs.impl.OnStart()
		if err != nil { // 如果真正的服务组件启动失败，则回滚到未开启状态
			// 回滚
			atomic.StoreUint32(&bs.started, 0)
			return err
		}
		return nil
	}
	bs.Logger.Debugw(fmt.Sprintf("Not starting %v service -- already started", bs.name), "impl", bs.impl)
	return ErrAlreadyStarted
}

// OnStart 通过什么也不做来实现 Service 接口
func (bs *BaseService) OnStart() error { return nil }

// Stop 通过调用 OnStop 来实现 Service 接口，并关闭 退出 channel。
// 如果服务已经被关闭，会返回一个 error
func (bs *BaseService) Stop() error {
	if atomic.CompareAndSwapUint32(&bs.stopped, 0, 1) { // 不能关闭一个已经被关闭的服务
		if atomic.LoadUint32(&bs.started) == 0 { // 不能关闭一个还未运行的服务
			bs.Logger.Errorw(fmt.Sprintf("Not stopping %v service -- has not been started yet", bs.name),
				"impl", bs.impl)
			// 回滚
			atomic.StoreUint32(&bs.stopped, 0)
			return ErrNotStarted
		}
		bs.Logger.Infow(fmt.Sprintf("Stopping %v service", bs.name), "impl", bs.impl)
		bs.impl.OnStop() // 关闭真正的服务组件
		close(bs.quit)
		return nil
	}
	bs.Logger.Debugw(fmt.Sprintf("Stopping %v service (already stopped)", bs.name), "impl", bs.impl)
	return ErrAlreadyStopped
}

// OnStop 通过什么也不做来实现 Service 接口
func (bs *BaseService) OnStop() {}

// Reset 通过调用 OnReset 来实现 Service 接口
func (bs *BaseService) Reset() error {
	if !atomic.CompareAndSwapUint32(&bs.stopped, 1, 0) { // 将服务转换为未关闭状态
		bs.Logger.Debugw(fmt.Sprintf("Can't reset %v service. Not stopped", bs.name), "impl", bs.impl)
		return fmt.Errorf("can't reset running %s", bs.name)
	}

	// 不管服务有没有开启，我们都可以重置
	atomic.CompareAndSwapUint32(&bs.started, 1, 0)

	bs.quit = make(chan struct{})
	return bs.impl.OnReset() // 让真正的服务组件重置
}

// OnReset 此方法不能被调用，因为会 panic
func (bs *BaseService) OnReset() error {
	panic("The service cannot be reset")
}

// IsRunning 判断服务是否正在运行，是的话返回 true，否则返回 false
func (bs *BaseService) IsRunning() bool {
	return atomic.LoadUint32(&bs.started) == 1 && atomic.LoadUint32(&bs.stopped) == 0
}

// Wait 等待直到服务被关闭
func (bs *BaseService) Wait() {
	<-bs.quit
}

// String 返回服务的名字
func (bs *BaseService) String() string {
	return bs.name
}

// Quit 返回服务的 quit channel
func (bs *BaseService) Quit() <-chan struct{} {
	return bs.quit
}
