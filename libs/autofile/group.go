package autofile

import (
	"BFT/libs/log"
	"BFT/libs/service"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultGroupCheckDuration = 5000 * time.Millisecond
	defaultHeadSizeLimit      = 10 * 1024 * 1024       // 10MB head 文件的大小限制
	defaultTotalSizeLimit     = 1 * 1024 * 1024 * 1024 // 1GB Group 的容量限制
	maxFilesToRemove          = 4                      // needs to be greater than 1
)

// Group 可以通过 Group 来限制 AutoFile，比如每个块的最大大小或 Group 中存储的总字节数
type Group struct {
	service.BaseService

	ID                 string
	Head               *AutoFile // The head AutoFile to write to
	headBuf            *bufio.Writer
	Dir                string // 存放 Head 文件的目录
	ticker             *time.Ticker
	mtx                sync.Mutex
	headSizeLimit      int64
	totalSizeLimit     int64
	groupCheckDuration time.Duration

	// [min, min+1, ..., max-1, max] 这些是 Group 里所有文件的索引，其中
	// [min, min+1, ..., max-1] 是 Group 里其他冗余文件的索引，而 max 是
	// head 文件索引，尽管冗余文件的名字形如 headPath.xxx，其中 xxx 就是冗余
	// 文件在 Group 里的索引，尽管 head 文件的名字中不带数字，但是可以用 max
	// 来指向 head 文件
	minIndex int
	maxIndex int

	// 该 channel 会在 processTicks routine 完成时被关闭
	// 这样我们就可以清理目录里的文件
	doneProcessTicks chan struct{}
}

// OpenGroup 根据 headPath 确定存放文件的目录在哪里，然后创建路径为 headPath 的 AutoFile
func OpenGroup(headPath string, groupOptions ...func(*Group)) (*Group, error) {
	dir, err := filepath.Abs(filepath.Dir(headPath)) // 获得存放 headPath 文件的目录
	if err != nil {
		return nil, err
	}
	head, err := OpenAutoFile(headPath)
	if err != nil {
		return nil, err
	}

	g := &Group{
		ID:                 "group:" + head.ID,
		Head:               head,
		headBuf:            bufio.NewWriterSize(head, 4096*10),
		Dir:                dir,
		headSizeLimit:      defaultHeadSizeLimit,
		totalSizeLimit:     defaultTotalSizeLimit,
		groupCheckDuration: defaultGroupCheckDuration,
		minIndex:           0,
		maxIndex:           0,
		doneProcessTicks:   make(chan struct{}),
	}

	for _, option := range groupOptions {
		option(g)
	}

	g.BaseService = *service.NewBaseService(log.NewCRLogger("info").With("module", "group-file"), "Group", g)

	gInfo := g.readGroupInfo()
	g.minIndex = gInfo.MinIndex
	g.maxIndex = gInfo.MaxIndex
	return g, nil
}

// OnStart 开启 processTicks goroutine 定期检查 Group 里的文件
func (g *Group) OnStart() error {
	g.ticker = time.NewTicker(g.groupCheckDuration)
	go g.processTicks()
	return nil
}

// OnStop 关闭 processTicks goroutine
func (g *Group) OnStop() {
	g.ticker.Stop()
	if err := g.FlushAndSync(); err != nil {
		g.Logger.Error("Error flushin to disk", "err", err)
	}
}

// Wait 等待直到 processTicks goroutine 结束
func (g *Group) Wait() {
	// wait for processTicks routine to finish
	<-g.doneProcessTicks
}

// Close 关闭 head 文件
func (g *Group) Close() {
	if err := g.FlushAndSync(); err != nil {
		g.Logger.Error("Error flushin to disk", "err", err)
	}

	g.mtx.Lock()
	_ = g.Head.closeFile()
	g.mtx.Unlock()
}

// HeadSizeLimit 返回当前 head 文件的最大大小限制
func (g *Group) HeadSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headSizeLimit
}

// TotalSizeLimit 返回当前 Group 的最大大小限制
func (g *Group) TotalSizeLimit() int64 {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.totalSizeLimit
}

// MaxIndex 返回 Group 里最后一个文件的索引
func (g *Group) MaxIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.maxIndex
}

// MinIndex 返回 Group 里第一个文件的索引
func (g *Group) MinIndex() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.minIndex
}

// Write 将 p  写入 Group 的 head 文件里。
// 它返回写入的字节数。如果 nn < len(p)，它还
// 返回一个 error。
// 注意:由于被写入被缓冲区，所以它们不会同步写入
func (g *Group) Write(p []byte) (nn int, err error) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headBuf.Write(p)
}

// WriteLine 将一行话写入到 Group 的 head 文件里，同时会在这行华的最后加上一个换行符。
// 注意:由于被写入被缓冲区，所以它们不会同步写入
func (g *Group) WriteLine(line string) error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	_, err := g.headBuf.Write([]byte(line + "\n"))
	return err
}

// Buffered 返回 head 缓冲区里的内容
func (g *Group) Buffered() int {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.headBuf.Buffered()
}

// FlushAndSync 将 head 缓冲区里的内容刷新到底层的 file 里，然后将 file 里的内容同步到硬盘上
func (g *Group) FlushAndSync() error {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	err := g.headBuf.Flush()
	if err == nil {
		err = g.Head.Sync()
	}
	return err
}

// processTicks 每隔一段时间调用以下两个方法：
// .checkHeadSizeLimit() 和 .checkTotalSizeLimit()
func (g *Group) processTicks() {
	defer close(g.doneProcessTicks)
	for {
		select {
		case <-g.ticker.C:
			g.checkHeadSizeLimit()
			g.checkTotalSizeLimit()
		case <-g.Quit():
			return
		}
	}
}

// 注意: 此函数在测试中手动调用。
func (g *Group) checkHeadSizeLimit() {
	limit := g.HeadSizeLimit()
	if limit == 0 {
		return
	}
	size, err := g.Head.Size()
	if err != nil {
		g.Logger.Error("Group's head may grow without bound", "head", g.Head.Path, "err", err)
		return
	}
	if size >= limit {
		g.RotateFile()
	}
}

func (g *Group) checkTotalSizeLimit() {
	limit := g.TotalSizeLimit() // 获取到 Group 的最大容量限制
	if limit == 0 {
		return
	}

	gInfo := g.readGroupInfo()
	totalSize := gInfo.TotalSize            // 获取当前 Group 的总大小
	for i := 0; i < maxFilesToRemove; i++ { // 如果当前 Group 的大小大于总量限制，那么一次最多可以删除 4 个文件
		index := gInfo.MinIndex + i
		if totalSize < limit { // 如果当前 Group 的大小小于最大限制，那么就 OK，直接返回
			return
		}
		if index == gInfo.MaxIndex {
			// 特殊情况，在日志里打印一个错误，就什么也不做
			g.Logger.Error("Group's head may grow without bound", "head", g.Head.Path)
			return
		}
		pathToRemove := filePathForIndex(g.Head.Path, index, gInfo.MaxIndex)
		fInfo, err := os.Stat(pathToRemove)
		if err != nil {
			g.Logger.Error("Failed to fetch info for file", "file", pathToRemove)
			continue
		}
		err = os.Remove(pathToRemove) // 删除文件
		if err != nil {
			g.Logger.Error("Failed to remove path", "path", pathToRemove)
			return
		}
		totalSize -= fInfo.Size() // 给 Group 腾点空间
	}
}

// RotateFile 关闭当前的 head 文件，然后重新创建一个 head 文件句柄，之前的 head 文件只是重新命了个名字
func (g *Group) RotateFile() {
	g.mtx.Lock()
	defer g.mtx.Unlock()

	headPath := g.Head.Path

	if err := g.headBuf.Flush(); err != nil {
		panic(err)
	}

	if err := g.Head.Sync(); err != nil {
		panic(err)
	}

	if err := g.Head.closeFile(); err != nil {
		panic(err)
	}

	indexPath := filePathForIndex(headPath, g.maxIndex, g.maxIndex+1)
	if err := os.Rename(headPath, indexPath); err != nil {
		// 给当前的 head 文件重新命个名字，这样下次打开 head 文件时，由于之前的 head 文件重命名，所以系统找不到 headPath 指向的文件，那么就重新打开一个 head 文件
		panic(err)
	}

	g.maxIndex++
}

// NewReader 返回一个新的 Group Reader
// 注意: 调用者必须关闭返回的 GroupReader.
func (g *Group) NewReader(index int) (*GroupReader, error) {
	r := newGroupReader(g)
	err := r.SetIndex(index)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GroupInfo holds information about the group.
type GroupInfo struct {
	MinIndex  int   // Group 里的第一个文件索引，包含 head
	MaxIndex  int   // Group 里的最后一个文件索引，包含 head
	TotalSize int64 // Group 的总大小
	HeadSize  int64 // Group 里 head 文件的大小
}

// ReadGroupInfo 在扫描了 Group 里所有文件之后，返回一个 GroupInfo
func (g *Group) ReadGroupInfo() GroupInfo {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	return g.readGroupInfo()
}

// Index includes the head.
// CONTRACT: caller should have called g.mtx.Lock
func (g *Group) readGroupInfo() GroupInfo {
	groupDir := filepath.Dir(g.Head.Path)  // Group 的目录
	headBase := filepath.Base(g.Head.Path) // head 文件的名字
	var minIndex, maxIndex int = -1, -1
	var totalSize, headSize int64 = 0, 0

	dir, err := os.Open(groupDir) // 打开 Group 的目录
	if err != nil {
		panic(err)
	}
	defer dir.Close()
	fiz, err := dir.Readdir(0) // 读取 Group 里所有文件，返回 []FileInfo 和一个 error
	if err != nil {
		panic(err)
	}

	// 扫描 Group 里所有文件
	for _, fileInfo := range fiz {
		if fileInfo.Name() == headBase { // 如果是 head 文件
			fileSize := fileInfo.Size()
			totalSize += fileSize // Group 的总大小累加上 head 文件的大小
			headSize = fileSize   // 记录下 head 文件的大小
			continue
		} else if strings.HasPrefix(fileInfo.Name(), headBase) { // 如果是非 head 文件
			fileSize := fileInfo.Size()
			totalSize += fileSize
			indexedFilePattern := regexp.MustCompile(`^.+\.([0-9]{3,})$`)
			submatch := indexedFilePattern.FindSubmatch([]byte(fileInfo.Name()))
			if len(submatch) != 0 {
				// Matches
				fileIndex, err := strconv.Atoi(string(submatch[1]))
				if err != nil {
					panic(err)
				}
				if maxIndex < fileIndex {
					maxIndex = fileIndex
				}
				if minIndex == -1 || fileIndex < minIndex {
					minIndex = fileIndex
				}
			}
		}
	}

	if minIndex == -1 {
		// 如果此时 Group 里只有一个 head 文件，那么 head 文件的索引就是 0
		minIndex, maxIndex = 0, 0
	} else {
		maxIndex++
	}
	return GroupInfo{minIndex, maxIndex, totalSize, headSize}
}

func filePathForIndex(headPath string, index int, maxIndex int) string {
	if index == maxIndex {
		// 如果 index 等于 maxIndex，则直接返回 head 文件的 path
		return headPath
	}
	return fmt.Sprintf("%v.%03d", headPath, index)
}

//--------------------------------------------------------------------------------

// GroupReader 提供一个从 Group 里读取内容的接口
type GroupReader struct {
	*Group
	mtx       sync.Mutex
	curIndex  int
	curFile   *os.File
	curReader *bufio.Reader
	curLine   []byte
}

func newGroupReader(g *Group) *GroupReader {
	return &GroupReader{
		Group:     g,
		curIndex:  0,
		curFile:   nil,
		curReader: nil,
		curLine:   nil,
	}
}

// Close closes the GroupReader by closing the cursor file.
func (gr *GroupReader) Close() error {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	if gr.curReader != nil {
		err := gr.curFile.Close()
		gr.curIndex = 0
		gr.curReader = nil
		gr.curFile = nil
		gr.curLine = nil
		return err
	}
	return nil
}

// Read implements io.Reader, reading bytes from the current Reader
// incrementing index until enough bytes are read.
func (gr *GroupReader) Read(p []byte) (n int, err error) {
	lenP := len(p)
	if lenP == 0 {
		return 0, errors.New("given empty slice")
	}

	gr.mtx.Lock()
	defer gr.mtx.Unlock()

	// 如果还没打开文件，就打开当前索引处的文件
	if gr.curReader == nil {
		if err = gr.openFile(gr.curIndex); err != nil {
			return 0, err
		}
	}

	// 迭代的读取 Group 里的文件，直到读满 p
	var nn int
	for {
		nn, err = gr.curReader.Read(p[n:])
		n += nn
		switch {
		case err == io.EOF: // 如果读到当前文件的末尾了
			if n >= lenP {
				return n, nil
			}
			// 如果没读满 p，就继续读取下一个文件
			if err1 := gr.openFile(gr.curIndex + 1); err1 != nil {
				return n, err1
			}
		case err != nil: // 如果读取文件的时候出错了
			return n, err
		case nn == 0: // 如果读取的文件是一个空文件
			return n, err
		}
	}
}

// openFile 如果给的要打开的文件索引大于 Group 的 maxIndex，则说明读取 Group 完了，那么就返回一个 io.EOF
func (gr *GroupReader) openFile(index int) error {
	// 锁定 Group，确保在此期间 head 文件不动
	gr.Group.mtx.Lock()
	defer gr.Group.mtx.Unlock()

	if index > gr.Group.maxIndex {
		return io.EOF
	}

	curFilePath := filePathForIndex(gr.Head.Path, index, gr.Group.maxIndex) // 根据给的文件索引返回指定的文件名
	curFile, err := os.OpenFile(curFilePath, os.O_RDONLY|os.O_CREATE, autoFilePerms)
	if err != nil {
		return err
	}
	curReader := bufio.NewReader(curFile)

	// 关闭当前的文件句柄
	if gr.curFile != nil {
		gr.curFile.Close()
	}
	gr.curIndex = index
	gr.curFile = curFile // 更新当前的文件句柄
	gr.curReader = curReader
	gr.curLine = nil
	return nil
}

// CurIndex 返回当前的文件索引
func (gr *GroupReader) CurIndex() int {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	return gr.curIndex
}

// SetIndex 设置当前的文建索引，并将当前的文件句柄更新到当前文件索引处的文件
func (gr *GroupReader) SetIndex(index int) error {
	gr.mtx.Lock()
	defer gr.mtx.Unlock()
	return gr.openFile(index)
}
