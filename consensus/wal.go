package consensus

import (
	auto "github.com/232425wxy/BFT/libs/autofile"
	srjson "github.com/232425wxy/BFT/libs/json"
	srlog "github.com/232425wxy/BFT/libs/log"
	sros "github.com/232425wxy/BFT/libs/os"
	"github.com/232425wxy/BFT/libs/service"
	srtime "github.com/232425wxy/BFT/libs/time"
	protoconsensus "github.com/232425wxy/BFT/proto/consensus"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"hash/crc32"
	"io"
	"path/filepath"
	"time"
)

const (
	// time.Time + max consensus msg size
	maxMsgSizeBytes = maxMsgSize + 24

	// how often the WAL should be sync'd during period sync'ing
	walDefaultFlushInterval = 2 * time.Second
)

//--------------------------------------------------------
// types and functions for savings consensus messages

// TimedWALMessage 包装了 WALMessage，在里面添加了时间用来调试
type TimedWALMessage struct {
	Time time.Time  `json:"time"` // 时间
	Msg  WALMessage `json:"msg"`  // msg 是一个接口
}

// EndHeightMessage marks the end of the given height inside WAL.
// @internal used by scripts/wal2json util.
type EndHeightMessage struct {
	Height int64 `json:"height"`
}

// WALMessage 可以是任何消息
type WALMessage interface{}

func init() {
	srjson.RegisterType(msgInfo{}, "github.com/232425wxy/BFT/wal/MsgInfo")
	srjson.RegisterType(timeoutInfo{}, "github.com/232425wxy/BFT/wal/TimeoutInfo")
	srjson.RegisterType(EndHeightMessage{}, "github.com/232425wxy/BFT/wal/EndHeightMessage")
}

//--------------------------------------------------------
// Simple write-ahead logger

// WAL 是任何预写日志记录器的接口。
type WAL interface {
	Write(WALMessage) error
	WriteSync(WALMessage) error
	FlushAndSync() error

	// SearchForEndHeight 搜索具有给定高度的 EndHeightMessage 并返回一个 GroupReader
	SearchForEndHeight(height int64, options *WALSearchOptions) (rd io.ReadCloser, found bool, err error)

	// 服务的启动、停止和等待方法
	Start() error
	Stop() error
	Wait()
}

// BaseWAL 在处理 msgs 之前将其写入磁盘，可用于崩溃恢复和确定性重放。
type BaseWAL struct {
	service.BaseService

	group *auto.Group

	enc *WALEncoder

	flushTicker   *time.Ticker
	flushInterval time.Duration
}

var _ WAL = &BaseWAL{}

// NewWAL 返回一个新的预写日志记录器 It's flushed and synced to disk every 2s and once when stopped.
func NewWAL(walFile string, groupOptions ...func(*auto.Group)) (*BaseWAL, error) {
	// 创建存储 wal 的目录
	err := sros.EnsureDir(filepath.Dir(walFile), 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure WAL directory is in place: %w", err)
	}

	// 打开一个 group
	group, err := auto.OpenGroup(walFile, groupOptions...)
	if err != nil {
		return nil, err
	}
	wal := &BaseWAL{
		group:         group,
		enc:           NewWALEncoder(group),
		flushInterval: walDefaultFlushInterval,
	}
	wal.BaseService = *service.NewBaseService(srlog.NewCRLogger("info").With("module", "WAL"), "baseWAL", wal)
	return wal, nil
}

// SetFlushInterval 设置刷新间隔时间
func (wal *BaseWAL) SetFlushInterval(i time.Duration) {
	wal.flushInterval = i
}

func (wal *BaseWAL) Group() *auto.Group {
	return wal.group
}

func (wal *BaseWAL) SetLogger(l srlog.CRLogger) {
	wal.BaseService.Logger = l
	wal.group.SetLogger(l)
}

func (wal *BaseWAL) OnStart() error {
	size, err := wal.group.Head.Size()
	if err != nil {
		return err
	} else if size == 0 {
		if err := wal.WriteSync(EndHeightMessage{0}); err != nil {
			return err
		}
	}
	err = wal.group.Start()
	if err != nil {
		return err
	}
	wal.flushTicker = time.NewTicker(wal.flushInterval)
	go wal.processFlushTicks()
	return nil
}

func (wal *BaseWAL) processFlushTicks() {
	for {
		select {
		case <-wal.flushTicker.C:
			if err := wal.FlushAndSync(); err != nil {
				wal.Logger.Errorw("Periodic WAL flush failed", "err", err)
			}
		case <-wal.Quit():
			return
		}
	}
}

// FlushAndSync 将 wal.group.head 缓冲区里的内容刷新到底层的 file 里，然后将 file 里的内容同步到硬盘上
func (wal *BaseWAL) FlushAndSync() error {
	return wal.group.FlushAndSync()
}

// Stop the underlying autofile group.
// Use Wait() to ensure it's finished shutting down
// before cleaning up files.
func (wal *BaseWAL) OnStop() {
	wal.flushTicker.Stop()
	if err := wal.FlushAndSync(); err != nil {
		wal.Logger.Errorw("error on flush data to disk", "error", err)
	}
	if err := wal.group.Stop(); err != nil {
		wal.Logger.Errorw("error trying to stop wal", "error", err)
	}
	wal.group.Close()
}

// Wait for the underlying autofile group to finish shutting down
// so it's safe to cleanup files.
func (wal *BaseWAL) Wait() {
	wal.group.Wait()
}

// Write 在 newStep 和每次从 peerMsgQueue 和 TimeoutTicker 里接收数据时被调用
func (wal *BaseWAL) Write(msg WALMessage) error {
	if wal == nil {
		return nil
	}

	if err := wal.enc.Encode(&TimedWALMessage{srtime.Now(), msg}); err != nil {
		wal.Logger.Errorw("Error writing msg to consensus wal. WARNING: recover may not be possible for the current height",
			"err", err, "msg", msg)
		return err
	}

	return nil
}

// 当我们接收到来自我们自己的消息时，会调用 WriteSync，以便在发送签名消息之前写入磁盘。
func (wal *BaseWAL) WriteSync(msg WALMessage) error {
	if wal == nil {
		return nil
	}

	if err := wal.Write(msg); err != nil {
		return err
	}

	if err := wal.FlushAndSync(); err != nil {
		wal.Logger.Errorw(`WriteSync failed to flush consensus wal.
		WARNING: may result in creating alternative proposals / votes for the current height iff the node restarted`,
			"err", err)
		return err
	}

	return nil
}

// WALSearchOptions 在搜寻 EndHeightMessage 的过程中额外设置的选项，IgnoreDataCorruptionErrors 如果为 true，
// 那么在硬盘中存储的数据发生错误时，不会忽略该错误，会退出查找过程，停止继续查找，但是如果 IgnoreDataCorruptionErrors
// 等于 false，那么在硬盘中存储的数据发生错误时，会忽略该错误，直接跳过，继续查找
type WALSearchOptions struct {
	IgnoreDataCorruptionErrors bool
}

// SearchForEndHeight 搜索具有给定高度的 EndHeightMessage 并返回一个 GroupReader
func (wal *BaseWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	var (
		msg *TimedWALMessage
		gr  *auto.GroupReader
	)
	lastHeightFound := int64(-1)

	// 注意:从组中的最后一个文件开始，因为我们通常搜索的是最后一个高度
	min, max := wal.group.MinIndex(), wal.group.MaxIndex()
	wal.Logger.Infow("Searching for height", "height", height, "min", min, "max", max)
	for index := max; index >= min; index-- {
		gr, err = wal.group.NewReader(index)
		if err != nil {
			return nil, false, err
		}

		// 解码器
		dec := NewWALDecoder(gr)
		for {
			// 解码得到 go 语言内生的 TimedWALMessage
			msg, err = dec.Decode()
			if err == io.EOF {
				// 如果想要查找的区块高度大于最近存储的区块高度，则说明想要查找的内容还未被记录下来，
				// 那么查找就可以到此为止了
				if lastHeightFound > 0 && lastHeightFound < height {
					gr.Close()
					return nil, false, nil
				}
				// 如果当前文件读取完毕，则继续读取下一个文件
				break
			}
			if options.IgnoreDataCorruptionErrors && IsDataCorruptionError(err) {
				wal.Logger.Errorw("Corrupted entry. Skipping...", "err", err)
				// 如果忽略硬盘内存储的数据发生损坏这个错误，那么当遇到此错误时则直接跳过忽略
				continue
			} else if err != nil {
				// 如果不能忽略硬盘里存储的数据发生损坏这一错误，那么在遇到此错误时则直接退出，停止查找
				gr.Close()
				return nil, false, err
			}

			if m, ok := msg.Msg.(EndHeightMessage); ok {
				lastHeightFound = m.Height
				if m.Height == height { // 找到了
					wal.Logger.Infow("Found", "height", height, "index", index)
					return gr, true, nil
				}
			}
		}
		gr.Close()
	}

	return nil, false, nil
}

// WALEncoder 编码器，将 TimedWALMessage 编码成字节数据，并写入到内部的 writer 内，
// 写入的字节数据格式为：
//	四字节的循环冗余校验和 crc + 四字节的数据长度 length + length 长度的数据 data
type WALEncoder struct {
	wr io.Writer
}

// NewWALEncoder 返回一个新的编码器，该编码器内部只有一个 writer，编码后的字节数据写入到 writer 内
func NewWALEncoder(wr io.Writer) *WALEncoder {
	return &WALEncoder{wr}
}

// Encode 用来将 go 语言内生的 TimedWALMessage 编码成字节数据，并写入到 WALEncoder.writer 内：
//	1. 将 go 语言内生的 WALMessage 转换为 protobuf 形式的 protoconsensus.WALMessage，得到 pbMsg
//	2. 利用 pbMsg 和 TimedWALMessage.Time 构造 protoconsensus.TimedWALMessage，得到 pv
//	3. 利用 gogoproto 的 marshal 对 pv 进行编码，得到字节数据 data
//	4. 利用 crc32 计算 data 的循环冗余校验和 crc，获取 data 的长度 length
//	5. 构造最终的数据帧 msg：crc|length|data，并将其写入到 WALEncoder.writer 内
func (enc *WALEncoder) Encode(v *TimedWALMessage) error {
	// 将 go 语言内生的 WALMessage 转换为 protobuf 形式的 protoconsensus.WALMessage pbMsg
	pbMsg, err := WALToProto(v.Msg)
	if err != nil {
		return err
	}

	// 利用 pbMsg 和 TimedWALMessage.Time 构造 protoconsensus.TimedWALMessage pv
	pv := protoconsensus.TimedWALMessage{
		Time: v.Time,
		Msg:  pbMsg,
	}

	// 利用 gogoproto 的 marshal 对 pv 进行编码，得到字节数据 data
	data, err := proto.Marshal(&pv)
	if err != nil {
		panic(fmt.Errorf("encode timed wall message failure: %w", err))
	}

	// 利用 crc32 计算 data 的循环冗余校验和 crc
	crc := crc32.Checksum(data, crc32c)
	// 获取 data 的长度 length
	length := uint32(len(data))
	if length > maxMsgSizeBytes {
		return fmt.Errorf("msg is too big: %d bytes, max: %d bytes", length, maxMsgSizeBytes)
	}

	// 计算数据帧的总大小：循环冗余校验和（4字节）+ data 数据长度标识位（4字节）+ data 数据自身的长度（length）
	totalLength := 8 + int(length)

	msg := make([]byte, totalLength)
	binary.BigEndian.PutUint32(msg[0:4], crc)
	binary.BigEndian.PutUint32(msg[4:8], length)
	copy(msg[8:], data)

	// 构造数据帧：msg:=crc|length|data，并将 msg 写入到 WALEncoder 的 writer 内
	_, err = enc.wr.Write(msg)
	return err
}

// 如果数据在 WAL 内部已损坏，则 IsDataCorruptionError 返回 true
func IsDataCorruptionError(err error) bool {
	_, ok := err.(DataCorruptionError)
	return ok
}

// DataCorruptionError 是磁盘数据损坏时发生的错误
type DataCorruptionError struct {
	cause error
}

func (e DataCorruptionError) Error() string {
	return fmt.Sprintf("DataCorruptionError[%v]", e.cause)
}

func (e DataCorruptionError) Cause() error {
	return e.cause
}

// A WALDecoder 将以字节形式存储的 TimedWALMessage 解码到 TimedWALMessage，
// 解码的对象格式如下：
//	四字节的循环冗余校验和 crc + 四字节的数据长度 length + length 长度的数据 data
// 首先会获取循环冗余校验和，然后拿到 data 的长度 length，接着提取 length 长度 data，
// 然后计算校验和是否相等，不相等则表示数据在硬盘中发生了损坏，最后就是利用 gogoproto unmarshal
// 等方法将字节数据解码成 TimedWALMessage
type WALDecoder struct {
	rd io.Reader
}

// NewWALDecoder 返回一个新的解码器，该解码器内部只有一个 reader，需要解码的数据就存储在 reader 里
func NewWALDecoder(rd io.Reader) *WALDecoder {
	return &WALDecoder{rd}
}

// Decode 用来将 WALDecoder 中的 reader 存储的数据正确解码成 TimedWALMessage：
//	1. 首先读取 WALDecoder.reader 的前四个字节的内容，这部分内容存储的是循环冗余校验和 crc
//	2. 接着读取 WALDecoder.reader 的后四个字节的内容，这部分内容存储的是有效数据的长度 length
//	3. 然后读取 WALDecoder.reader 剩下的 length 个字节的内容 data，data 存储的是真正有效的数据
//	4. 利用 crc32 计算 data 的校验和 actualCrc，然后将其与 crc 进行比较是否相同，不相同则代表出错了
//	5. 利用 gogoproto 的 unmarshal 方法将 data 解析成 protoconsensus.TimedWALMessage，然后提取
//	   protoconsensus.TimedWALMessage 里的 Msg，将其转换为 go 语言内生的 WALMessage，最后构造
//	   TimedWALMessage 并返回
func (dec *WALDecoder) Decode() (*TimedWALMessage, error) {
	// 存放循环冗余校验和的
	b := make([]byte, 4)

	// 从 WALDecoder 的 reader 里读取至多四字节的内容，用于循环冗余校验
	_, err := dec.rd.Read(b)
	if errors.Is(err, io.EOF) {
		return nil, err
	}
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to read checksum: %v", err)}
	}

	// 将循环冗余校验和以大端的编码形式转换为 uint32，得到 crc
	crc := binary.BigEndian.Uint32(b)

	// 接着读取 WALDecoder 的 reader 的后面四个字节的内容，此处存放的是长度
	b = make([]byte, 4)
	_, err = dec.rd.Read(b)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to read length: %v", err)}
	}
	// 将长度以大端的编码方式转换为 uint32，得到 length
	length := binary.BigEndian.Uint32(b)

	if length > maxMsgSizeBytes {
		return nil, DataCorruptionError{fmt.Errorf(
			"length %d exceeded maximum possible value of %d bytes",
			length,
			maxMsgSizeBytes)}
	}

	// 从 WALDecoder 的 reader 里读取剩下的 length 字节，这部分数据是存储的内容
	data := make([]byte, length)
	n, err := dec.rd.Read(data)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to read data: %v (read: %d, wanted: %d)", err, n, length)}
	}

	// 检查数据的校验和对不对
	actualCRC := crc32.Checksum(data, crc32c)
	if actualCRC != crc {
		return nil, DataCorruptionError{fmt.Errorf("checksums do not match: read: %v, actual: %v", crc, actualCRC)}
	}

	// 将字节数据解码到 protoconsensus.TimedWALMessage 中
	var res = new(protoconsensus.TimedWALMessage)
	err = proto.Unmarshal(data, res)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to decode data: %v", err)}
	}

	// 将 protoconsensus.TimedWALMessage.Msg 转换为 go 语言内生的 WALMessage
	walMsg, err := WALFromProto(res.Msg)
	if err != nil {
		return nil, DataCorruptionError{fmt.Errorf("failed to convert from proto: %w", err)}
	}

	// 构建 TimedWALMessage，并将其返回出去
	tMsgWal := &TimedWALMessage{
		Time: res.Time,
		Msg:  walMsg,
	}

	return tMsgWal, err
}

type nilWAL struct{}

var _ WAL = nilWAL{}

func (nilWAL) Write(m WALMessage) error     { return nil }
func (nilWAL) WriteSync(m WALMessage) error { return nil }
func (nilWAL) FlushAndSync() error          { return nil }
func (nilWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return nil, false, nil
}
func (nilWAL) Start() error { return nil }
func (nilWAL) Stop() error  { return nil }
func (nilWAL) Wait()        {}
