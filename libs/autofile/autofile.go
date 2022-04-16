package autofile

import (
	srrand "github.com/232425wxy/BFT/libs/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

/* AutoFile 的用法

// Create/Append to ./autofile_test
af, err := OpenAutoFile("autofile_test")
if err != nil {
	panic(err)
}

// Stream of writes.
// During this time, the file may be moved e.g. by logRotate.
for i := 0; i < 60; i++ {
	af.Write([]byte(Fmt("LOOP(%v)", i)))
	time.Sleep(time.Second)
}

// Close the AutoFile
err = af.Close()
if err != nil {
	panic(err)
}
*/

const (
	autoFileClosePeriod = 1000 * time.Millisecond // 默认打开的 autoFile 会在 1 秒钟以后被关闭
	autoFilePerms       = os.FileMode(0600)
)

// AutoFile 在打开 1 秒钟以后会被自动关闭，或者在收到 SIGHUB 信号时也会被关闭
// 可以用在日志文件里
type AutoFile struct {
	ID   string
	Path string // autoFile 的真正地址，是绝对路径

	closeTicker      *time.Ticker
	closeTickerStopc chan struct{} // closed when closeTicker is stopped
	hupc             chan os.Signal

	mtx  sync.Mutex
	file *os.File
}

// OpenAutoFile 创建一个 AutoFile 实例
func OpenAutoFile(path string) (*AutoFile, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	af := &AutoFile{
		ID:               srrand.Str(12) + ":" + path, // 随机创建 autoFile 的 ID
		Path:             path,
		closeTicker:      time.NewTicker(autoFileClosePeriod),
		closeTickerStopc: make(chan struct{}),
	}
	if err := af.openFile(); err != nil {
		af.Close()
		return nil, err
	}

	// 等待 syscall.SIGHUP 信号来关闭 af
	af.hupc = make(chan os.Signal, 1)
	signal.Notify(af.hupc, syscall.SIGHUP) // 将接收到的 syscall.SIGHUP 信号放到 af.hupc channel 里
	go func() {                            // 时刻监听 af.hupc 里是否有 syscall.SIGHUP 信号，有的话，就把 af 关了
		for range af.hupc {
			_ = af.closeFile()
		}
	}()

	go af.closeFileRoutine() // 将关闭 af 的功能放到 goroutine 里

	return af, nil
}

// Close 关闭 af
func (af *AutoFile) Close() error {
	af.closeTicker.Stop()
	close(af.closeTickerStopc) // 向 af.closeFileRoutine() 发送信号，表示可以关闭 af 了
	if af.hupc != nil {        // 如果 af.hupc 初始化了，则假装向 af.hupc 里发送 syscall.SIGHUP 信号，从而关闭 af
		close(af.hupc)
	}
	return af.closeFile()
}

func (af *AutoFile) closeFileRoutine() {
	for {
		select {
		case <-af.closeTicker.C: // 默认情况下，在 af 打开 1 秒钟以后，就会调用 af.closeFile()
			_ = af.closeFile()
		case <-af.closeTickerStopc:
			return
		}
	}
}

func (af *AutoFile) closeFile() (err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	file := af.file // 让 file 等于 af.file，目的是为了关闭 af.file
	if file == nil {
		return nil
	}

	af.file = nil // 这一步很关键，让 af.file 等于 nil
	return file.Close()
}

// Write 向 af.file 中写入 b，返回写入内容的字节数和错误，
// 如果 af.file 等于 nil，则有必要调用 af.openFile()
func (af *AutoFile) Write(b []byte) (n int, err error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		if err = af.openFile(); err != nil {
			return
		}
	}

	n, err = af.file.Write(b)
	return
}

// Sync 将 af.file 里的内容同步到硬盘上，如果 af 已经被关闭了，
// 则有必要调用 af.openFile()
func (af *AutoFile) Sync() error {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		if err := af.openFile(); err != nil {
			return err
		}
	}
	return af.file.Sync()
}

func (af *AutoFile) openFile() error {
	file, err := os.OpenFile(af.Path, os.O_RDWR|os.O_CREATE|os.O_APPEND, autoFilePerms) // 真正的打开文件操作
	if err != nil {
		return err
	}
	af.file = file
	return nil
}

// Size 返回 af.file 的大小，如果 af 已经被关闭了，
// 则有必要调用 af.openFile()
func (af *AutoFile) Size() (int64, error) {
	af.mtx.Lock()
	defer af.mtx.Unlock()

	if af.file == nil {
		if err := af.openFile(); err != nil {
			return -1, err
		}
	}

	stat, err := af.file.Stat()
	if err != nil {
		return -1, err
	}
	return stat.Size(), nil
}
