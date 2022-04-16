package async

import (
	"fmt"
	"runtime"
)

// Task
// val: task 执行完成后返回的值
// err: 在 task 执行过程中返回的错误
// abort:  无论所有的 task 是否执行完，通知 Parallel 立刻返回
type Task func(i int) (val interface{}, abort bool, err error)

/*
               ---------> TaskResultCh ---------|
TaskResult --->|                                | ---> TaskResultSet
               ---------> TaskResultOK ---------|
*/

type TaskResult struct {
	Value interface{}
	Error error
}

type TaskResultCh <-chan TaskResult

// taskResultOK：表示一个 task 执行完后返回的相关信息
type taskResultOK struct {
	TaskResult
	OK bool
}

// TaskResultSet：表示一群 task 的返回值
type TaskResultSet struct {
	chz     []TaskResultCh
	results []taskResultOK
}

func newTaskResultSet(chz []TaskResultCh) *TaskResultSet {
	return &TaskResultSet{
		chz:     chz,
		results: make([]taskResultOK, len(chz)),
	}
}

func (trs *TaskResultSet) Channels() []TaskResultCh {
	return trs.chz
}

// LatestResult 返回 TaskResultSet 指定索引处的 task 执行后的信息
func (trs *TaskResultSet) LatestResult(index int) (TaskResult, bool) {
	if len(trs.results) <= index {
		return TaskResult{}, false
	}
	resultOK := trs.results[index]
	return resultOK.TaskResult, resultOK.OK
}

// Reap
// 注意：不是线程安全的
// 从 TaskResultSet.chz 中非阻塞式地将 task 执行过后的内容读取到 TaskResultSet.results 中
func (trs *TaskResultSet) Reap() *TaskResultSet {
	for i := 0; i < len(trs.results); i++ {
		var trch = trs.chz[i]
		select {
		case result, ok := <-trch:
			if ok {
				// 向 trs.results 中写入 task 执行过后的结果
				trs.results[i] = taskResultOK{
					TaskResult: result,
					OK:         true,
				}
			}
		default:
			// 非阻塞式，什么也不做
		}
	}
	return trs
}

// Wait
// 注意: 不是线程安全的
// 与 Reap 的功能一样，不过会等待所有的 task 任务执行完了，才会将所有 task 执行完后的信息放入到 TaskResultSet.results 里
func (trs *TaskResultSet) Wait() *TaskResultSet {
	for i := 0; i < len(trs.results); i++ {
		var trch = trs.chz[i]
		result, ok := <-trch
		if ok {
			// 向 trs.results 中写入 task 执行过后的结果
			trs.results[i] = taskResultOK{
				TaskResult: result,
				OK:         true,
			}
		}
	}
	return trs
}

// FirstValue 返回 TaskResultSet.results 里第一个 task 执行完后返回值不为空的 task 的返回值
func (trs *TaskResultSet) FirstValue() interface{} {
	for _, result := range trs.results {
		if result.Value != nil {
			return result.Value
		}
	}
	return nil
}

// FirstError 返回 TaskResultSet.results 里第一个 task 执行完后错误信息不为空的 task 的错误信息
func (trs *TaskResultSet) FirstError() error {
	for _, result := range trs.results {
		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

// Parallel
// 将所有 task 并行运行，一旦有一个 task 的 abort 等于 true，则 Parallel 停止运行
// 返回值 ok 如果等于 false，则代表其中有一个 task 的 abort 等于 true
func Parallel(tasks ...Task) (trs *TaskResultSet, ok bool) {
	var taskResultChz = make([]TaskResultCh, len(tasks)) // 用来返回的
	var taskDoneCh = make(chan bool, len(tasks))         // 一个用来等待并指示所有 task 执行完的 channel，如果有任何一个 task 的 abort 等于 true，就会提前退出
	//var numPanics = new(int32)                           // 跟踪每一个 task 执行的时候是否 panic，只要有 task panic，numPanics 就会加一

	// 如果有任何 task panic 或返回的 abort 等于 true 了，则将 ok 设为 false
	ok = true

	// 在各个 goroutine 中执行所有 task
	for i, task := range tasks {
		var taskResultCh = make(chan TaskResult, 1) // taskResultCh 只能放一个 task 的执行结果
		taskResultChz[i] = taskResultCh
		go func(i int, task Task, taskResultCh chan TaskResult) {
			defer func() {
				if pnk := recover(); pnk != nil {
					//atomic.AddInt32(numPanics, 1)
					// 把 panic 的内容放到 TaskResult 里，然后将 TaskResult 推送到 taskResultCh 中，由于 panic 了，所以对应的 task 返回值为 nil
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					taskResultCh <- TaskResult{nil, fmt.Errorf("panic in task %v : %s", pnk, buf)}
					// 把 taskResultCh 关闭掉，这样 taskResultCh 才能执行 .Wait() 方法
					close(taskResultCh)
					// Decrement waitgroup.
					taskDoneCh <- true
				}
			}()
			// 执行 task
			var val, abort, err = task(i)
			// 将 val 和 err 包装到 TaskResult 里
			taskResultCh <- TaskResult{val, err}
			// 关闭 taskResultCh，让 trs.Wait() 可以正常执行
			close(taskResultCh)
			// taskDoneCh 等堵塞量减一
			taskDoneCh <- abort
		}(i, task, taskResultCh)
	}

	// 等待直到所有 task 执行完了或有一个 task 的 abort 等于 true
	for i := 0; i < len(tasks); i++ {
		abort := <-taskDoneCh
		if abort {
			ok = false
			break
		}
	}

	// 只要有任何一个 task 在执行的时候 panic 了，则将 ok 置为 false
	//ok = ok && (atomic.LoadInt32(numPanics) == 0)

	return newTaskResultSet(taskResultChz).Reap(), ok
}
