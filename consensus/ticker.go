package consensus

import (
	"BFT/libs/log"
	"BFT/libs/service"
	"time"
)

var (
	tickTockBufferSize = 10
)


// TimeoutTicker wraps time.Timer,
// scheduling timeouts only for greater height/round/step
// than what it's already seen.
// Timeouts are scheduled along the tickChan,
// and fired on the tockChan.
type TimeoutTicker struct {
	service.BaseService

	timer    *time.Timer
	tickChan chan timeoutInfo // for scheduling timeouts
	tockChan chan timeoutInfo // for notifying about them
}

// NewTimeoutTicker returns a new TimeoutTicker.
func NewTimeoutTicker() *TimeoutTicker {
	tt := &TimeoutTicker{
		timer:    time.NewTimer(0),
		tickChan: make(chan timeoutInfo, tickTockBufferSize),
		tockChan: make(chan timeoutInfo, tickTockBufferSize),
	}
	tt.BaseService = *service.NewBaseService(log.NewCRLogger("info").With("module", "TimeoutTicker"), "TimeoutTicker", tt)
	tt.stopTimer() // don't want to fire until the first scheduled timeout
	return tt
}

// OnStart implements service.Service. It starts the timeout routine.
func (t *TimeoutTicker) OnStart() error {

	go t.timeoutRoutine()

	return nil
}

// OnStop implements service.Service. It stops the timeout routine.
func (t *TimeoutTicker) OnStop() {
	t.BaseService.OnStop()
	t.stopTimer()
}

// Chan 返回发送 timeoutInfo 的通道
func (t *TimeoutTicker) Chan() <-chan timeoutInfo {
	return t.tockChan
}

// ScheduleTimeout 往 TimeoutTicker.tickChan 通道里传送一个 timeoutInfo
func (t *TimeoutTicker) ScheduleTimeout(ti timeoutInfo) {
	// 往这里面传输的 ti 会被 timeoutRoutine 接收到
	t.tickChan <- ti
}

//-------------------------------------------------------------

// 停止 TimeoutTicker.timer 计时器
func (t *TimeoutTicker) stopTimer() {
	// 如果 timer 已经 stop 了，再调用 Stop() 方法将返回 false
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
			t.Logger.Debugw("Timer already stopped")
		}
	}
}

// send on tickChan to start a new timer.
// timers are interupted and replaced by new ticks from later steps
// timeouts of 0 on the tickChan will be immediately relayed to the tockChan
func (t *TimeoutTicker) timeoutRoutine() {
	t.Logger.Debugw("Starting timeout routine")
	var ti timeoutInfo
	for {
		select {
		case newti := <-t.tickChan:
			// ScheduleTimeout 会往 TimeoutTicker.tickChan 里传送一个 timeoutInfo，然后在此处被接收
			t.Logger.Debugw("Received tick", "old_ti", ti, "new_ti", newti)

			// 忽略过期的 timeoutInfo，所谓过期的 timeoutInfo，是指 newti 的 height、round、step 小于
			// ti 的 height、round、step
			if newti.Height < ti.Height {
				continue
			} else if newti.Height == ti.Height {
				if newti.Round < ti.Round {
					continue
				} else if newti.Round == ti.Round {
					if ti.Step > 0 && newti.Step <= ti.Step {
						continue
					}
				}
			}

			// 关闭之前的 timer，重置 timer，重新开始超时倒计时
			t.stopTimer()
			reset := t.timer.Reset(newti.Duration)

			// 用当前接收的 timeoutInfo 更新 ti
			ti = newti
			t.Logger.Debugw("Scheduled timeout", "reset result", reset, "dur", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)
		case <-t.timer.C:
			// 超时了
			t.Logger.Debugw("Timed out", "dur", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)
			// 这里使用 goroutine 是为了保证 timeoutRoutine 不会被阻塞
			go func(toi timeoutInfo) { t.tockChan <- toi }(ti)
		case <-t.Quit():
			return
		}
	}
}
