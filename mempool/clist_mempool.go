package mempool

import (
	"BFT/config"
	"BFT/gossip"
	auto "BFT/libs/autofile"
	"BFT/libs/clist"
	srlog "BFT/libs/log"
	srmath "BFT/libs/math"
	sros "BFT/libs/os"
	protoabci "BFT/proto/abci"
	"BFT/proxy"
	"BFT/types"
	"bytes"
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
)

// TxKeySize 是 tx 键索引的大小，单位是：字节，默认为 32
// 例如：mapTxCache.cacheMap: make(map[[TxKeySize]byte]*list.Element, cacheSize)
const TxKeySize = sha256.Size

var newline = []byte("\n")

//--------------------------------------------------------------------------------

// CListMempool 是一个有序的内存池，用来存储被共识算法提交之前的交易数据，
// 在将交易数据添加到内存池中之前，会先调用 CheckTx 检查交易数据的合法性，
// 内存池使用一个支持并发操作的 List 结构来存储数据，这样就可以让多个并发的
// reader 从内存池中读取到交易数据
type CListMempool struct {
	// Atomic integers
	height   int64 // the last block Update()'d to
	txsBytes int64 // 内存池中所有交易数据的大小，单位是：字节

	// 当内存池里的交易数据是可用的时候，就通知共识模块
	notifiedTxsAvailable bool
	txsAvailable         chan struct{} // 当内存池不是空的时候，为每个高度触发一次

	config *config.MempoolConfig

	// // Update 方法的互斥锁，防止与 CheckTx 或 ReapMaxBytes 方法并发执行
	updateMtx sync.RWMutex
	preCheck  PreCheckFunc

	wal          *auto.AutoFile // 内存池中 txs 的日志
	txs          *clist.CList   // 存储 good tx 的链表
	proxyAppConn *proxy.AppConnMempool

	// 跟踪我们是否重新检查 tx。
	// 这些不受互斥体的保护，预计将以串行方式变异( 即通过串行调用的 abci响应 )。
	recheckCursor *clist.CElement // next expected response
	recheckEnd    *clist.CElement // re-checking stops here

	//	用于快速定位到某笔交易数据，能够快速定位的依托是 hash(tx) -> txKey，根据 txKey 快速定位到交易数据，
	// 	映射的结构如下：交易的哈希值：clist.Element
	// 	这里 clist.Element.Value 存储的是 *mempoolTx，而 *mempoolTx.tx 存储的是交易的原始数据
	txsMap sync.Map

	// 保存已看到的tx缓存。这就降低了对代理App的压力。
	cache txCache

	logger srlog.CRLogger
}

var _ Mempool = &CListMempool{}

// CListMempoolOption 用来配置 CListMempool 的选项
type CListMempoolOption func(*CListMempool)

// NewCListMempool 根据给定的配置参数实例化一个 CListMempool，并且将其连接到一个代理应用
func NewCListMempool(
	config *config.MempoolConfig,
	proxyAppConn *proxy.AppConnMempool,
	height int64,
	options ...CListMempoolOption,
) *CListMempool {
	mempool := &CListMempool{
		config:        config,
		proxyAppConn:  proxyAppConn,
		txs:           clist.New(),
		height:        height,
		recheckCursor: nil,
		recheckEnd:    nil,
		logger:        srlog.NewCRLogger("info"),
	}
	if config.CacheSize > 0 {
		// tx 缓存池
		mempool.cache = newMapTxCache(config.CacheSize)
	} else {
		mempool.cache = nopTxCache{}
	}
	// 注意这个函数很重要 设置了代理连接的回调函数为resCb(req *protoabci.Request, res *protoabci.Response)
	// 可能当你看到这个不是很理解 可以先只有这个印象。
	// 因为交易池在收到交易后会把交易提交给APP 根据APP的返回来决定后续这个交易
	// 如何处理 所以在APP处理完提交的交易后回调mempool.resCb进而让mempool来继续决定当前交易如何处理
	proxyAppConn.SetResponseCallback(mempool.globalCb)
	for _, option := range options {
		option(mempool)
	}
	return mempool
}

// EnableTxsAvailable 该方法不是线程安全的，只应该在 CListMempool 启动的时候调用，
// 初始化 CListMempool.txsAvailable，txsAvailable 是一个通道，初始化后其容量变为
//	1，当内存池里有可用交易数据时，会往该通道里传递信息
func (mem *CListMempool) EnableTxsAvailable() {
	mem.txsAvailable = make(chan struct{}, 1)
}

// SetLogger sets the Logger.
func (mem *CListMempool) SetLogger(l srlog.CRLogger) {
	mem.logger = l
}

// WithPreCheck 为内存池设置一个过滤器，当 f(tx) 返回一个 non-nil error 的时候，
// 会拒绝该笔交易，该过滤器在 CheckTx 之前执行。 仅适用于第一个创建的块，之后，
// Update 将覆盖现有值
func WithPreCheck(f PreCheckFunc) CListMempoolOption {
	return func(mem *CListMempool) { mem.preCheck = f }
}

// InitWAL 打开存放 WAL 的目录，并打开一个 WAL 文件，然后将对该文件的操作句柄赋值给 .wal
func (mem *CListMempool) InitWAL() error {
	var (
		walDir  = mem.config.WalDir()
		walFile = walDir + "/wal"
	)

	const perm = 0700
	if err := sros.EnsureDir(walDir, perm); err != nil {
		return err
	}

	af, err := auto.OpenAutoFile(walFile)
	if err != nil {
		return fmt.Errorf("can't open autofile %s: %w", walFile, err)
	}

	mem.wal = af
	return nil
}

// CloseWAL 关闭 .wal 句柄
func (mem *CListMempool) CloseWAL() {
	if err := mem.wal.Close(); err != nil {
		mem.logger.Errorw("Error closing WAL", "err", err)
	}
	mem.wal = nil
}

// Lock 让 .updateMtx 锁住
func (mem *CListMempool) Lock() {
	mem.updateMtx.Lock()
}

// Unlock 让 .updateMtx 解锁
func (mem *CListMempool) Unlock() {
	mem.updateMtx.Unlock()
}

// Size 返回内存池中的交易数量
func (mem *CListMempool) Size() int {
	return mem.txs.Len()
}

// TxsBytes 返回内存池中存储的交易数据大小，单位是：字节
func (mem *CListMempool) TxsBytes() int64 {
	return atomic.LoadInt64(&mem.txsBytes)
}

// FlushAppConn 刷新连接缓存，将里面的数据发送给连接的对等方
func (mem *CListMempool) FlushAppConn() error {
	return mem.proxyAppConn.FlushSync()
}

// Flush 从内存池中和缓存中删除所有交易数据
// 注意：调用 Flush 可能会使内存池处于不一致的状态。
func (mem *CListMempool) Flush() {
	mem.updateMtx.RLock()
	defer mem.updateMtx.RUnlock()

	// 将内存池存储的交易数据大小改为 0
	_ = atomic.SwapInt64(&mem.txsBytes, 0)
	mem.cache.Reset()

	// 从内存池中的第一个交易数据元素开始，删除里面所有的元素
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		mem.txs.Remove(e)
		e.DetachPrev()
	}
	// Range(f func(key, value interface{}) bool) 对映射中的每个键和值依次调用 f。如果 f 返回 false, range 停止迭代。
	// Range 并不一定对应 Map 内容的任何一致的快照:没有键会被访问多次，但是如果任何键的值被同时存储或删除，
	// Range 可以在 Range 调用期间从任何点反映该键的任何映射。
	// Range 可以是 O(N)，包含映射中的元素数量，即使 f 在多次调用后返回 false。
	mem.txsMap.Range(func(key, _ interface{}) bool {
		mem.txsMap.Delete(key)
		return true
	})
}

// TxsFront 返回内存池中存储的第一个 tx
func (mem *CListMempool) TxsFront() *clist.CElement {
	return mem.txs.Front()
}

// TxsWaitChan 返回一个等待交易数据的通道，该通道在内存池中有交易数据时就会被关闭
func (mem *CListMempool) TxsWaitChan() <-chan struct{} {
	return mem.txs.WaitChan()
}

// CheckTx 如果我们调用了 Update 或者 Reap 方法，则 CheckTx 会被阻塞，它负责将新的交易提交给 APP，然后决定
// 是否将新交易加入到内存池里
// CheckTx 的执行流程如下：
//	1. 获取待检查交易数据的大小，检查当前内存池中交易数据的大小加上该交易
//	   数据的大小是否超过上限，并检查内存池中的交易数据数量是否超过上限，
//	   最后检查该交易数据的大小是否超过单笔交易数据的大小上限
//	2. 如果 CListMempool 的 .preCheck 不为 nil，则调用 .preCheck()
//	   检查该笔交易
//	3. 尝试将这笔 tx 放到缓冲区里
//	4. 内存池将新的 tx 提交给 代理应用程序，代理应用程序执行 CheckTx 检查这个新的 tx是否可用，然后调用内
//	   存池的 reqResCb 方法来检查代理应用程序返回的 Response，一般来说，如果应用程序认为 tx 可用，则会
//	   将 Response 的 .Code 字段设置为 CodeTypeOK（0），这样内存池检查 Response 的 Code 字段是否等
//	   于 CodeTypeOK，如果等于，则将其添加到内存池里
func (mem *CListMempool) CheckTx(tx types.Tx, cb func(*protoabci.Response), txInfo TxInfo) error {
	mem.updateMtx.RLock()
	// use defer to unlock mutex because application (*local client*) might panic
	defer mem.updateMtx.RUnlock()

	//------------------------------------------------------
	// 检查大小

	// 获取需要检查的这笔交易的大小
	txSize := len(tx)

	if err := mem.isFull(txSize); err != nil {
		return err
	}

	// 如果单个 tx 的大小大于 tx 大小的上限，就返回错误
	if txSize > mem.config.MaxTxBytes {
		return ErrTxTooLarge{mem.config.MaxTxBytes, txSize}
	}
	//------------------------------------------------------

	// 判断内存池的预检查方法是否为空，不为空的话，先利用预检查方法对交易数据进行检查，
	// 比如限制交易数据大小不超过区块大小限制，如果检查出错误，则返回错误
	if mem.preCheck != nil {
		if err := mem.preCheck(tx); err != nil {
			return ErrPreCheck{err}
		}
	}

	// 将每条交易都写入到 .wal 里，每条交易占据一行，如果写入的时候发生了错误，则返回错误
	if mem.wal != nil {
		_, err := mem.wal.Write(append([]byte(tx), newline...))
		if err != nil {
			return fmt.Errorf("wal.Write: %w", err)
		}
	}

	// 当 proxyAppConn.Client 因为某些原因而被迫停止时（stopForError），此时 .Error() 返回的错误不为 nil
	if err := mem.proxyAppConn.Error(); err != nil {
		return err
	}

	// 尝试将交易的哈希值存储到缓存区中
	if !mem.cache.Push(tx) { // 如果缓存区中已经存在了该交易的哈希值
		// 请注意，有可能 tx 仍然在缓存中，但不再在内存池中(例如。在提交一个块后，TXS 会从内存池中删除，
		// 但不会从缓存中删除)，所以我们只记录 TXS 的发送方仍然在内存池中。
		if e, ok := mem.txsMap.Load(TxKey(tx)); ok {
			// 从内存池中找到该交易的详细信息
			memTx := e.(*clist.CElement).Value.(*mempoolTx)
			memTx.senders.LoadOrStore(txInfo.SenderP2PID, true)
			// LoadOrStore 方法有两个返回值，第一个值返回映射里给定键的对应的现有 value 值，第二个值返回
			// bool 值，用来表示给定的值是否存在于映射中，如果存在则返回 true，所以此处可以判断同一个发送者
			// 是否多次发送了相同的交易数据
		}

		return ErrTxInCache
	}

	// 内存池在这里将新的 tx 提交给 代理应用程序，代理应用程序执行 CheckTx 检查这个新的 tx是否可用
	reqRes := mem.proxyAppConn.CheckTxAsync(protoabci.RequestCheckTx{Tx: tx})
	// 然后调用内存池的 reqResCb 方法来检查代理应用程序返回的 Response，一般来说，如果应用程序认为
	// tx 可用，则会将 Response 的 .Code 字段设置为 CodeTypeOK（0），这样内存池检查 Response 的 Code
	// 字段是否等于 CodeTypeOK，如果等于，则将其添加到内存池里
	reqRes.SetCallback(mem.reqResCb(tx, txInfo.SenderP2PID, cb))

	return nil
}

// Global callback that will be called after every ABCI response.
// Having a single global callback avoids needing to set a callback for each request.
// However, processing the checkTx response requires the peerID (so we can track which txs we heard from who),
// and peerID is not included in the ABCI request, so we have to set request-specific callbacks that
// include this information. If we're not in the midst of a recheck, this function will just return,
// so the request specific callback can do the work.
//
// When rechecking, we don't need the peerID, so the recheck callback happens
// here.
// proxyAppConn 的回调函数
func (mem *CListMempool) globalCb(req *protoabci.Request, res *protoabci.Response) {
	if mem.recheckCursor == nil {
		return
	}
	mem.resCbRecheck(req, res)
}

// Request specific callback that should be set on individual reqRes objects
// to incorporate local information when processing the response.
// This allows us to track the peer that sent us this tx, so we can avoid sending it back to them.
// NOTE: alternatively, we could include this information in the ABCI request itself.
//
// External callers of CheckTx, like the RPC, can also pass an externalCb through here that is called
// when all other response processing is complete.
//
// Used in CheckTx to record PeerID who sent us the tx.
// abci.ReqRes 的回调函数
func (mem *CListMempool) reqResCb(
	tx []byte,
	peerP2PID gossip.ID,
	externalCb func(*protoabci.Response),
) func(res *protoabci.Response) {
	return func(res *protoabci.Response) {
		if mem.recheckCursor != nil {
			// this should never happen
			panic("recheck cursor is not nil in reqResCb")
		}

		mem.resCbFirstTime(tx, peerP2PID, res)

		// passed in by the caller of CheckTx, eg. the RPC
		if externalCb != nil {
			externalCb(res)
		}
	}
}

// Called from:
//  - resCbFirstTime (lock not held) if tx is valid
func (mem *CListMempool) addTx(memTx *mempoolTx) {
	e := mem.txs.PushBack(memTx)
	mem.txsMap.Store(TxKey(memTx.tx), e)
	atomic.AddInt64(&mem.txsBytes, int64(len(memTx.tx)))
}

// Called from:
//  - Update (lock held) if tx was committed
// 	- resCbRecheck (lock not held) if tx was invalidated
func (mem *CListMempool) removeTx(tx types.Tx, elem *clist.CElement) {
	mem.txs.Remove(elem)
	elem.DetachPrev()
	mem.txsMap.Delete(TxKey(tx))
	atomic.AddInt64(&mem.txsBytes, int64(-len(tx)))
	mem.cache.Remove(tx)
}

// isFull 先检查内存池里 tx 的个数是否达到上限，然后加上新 tx 的 txSize 大小后，
// 检查内存池里存储的 tx 容量（单位：字节）是否达到上限，如果达到上限了，返回一个错误
func (mem *CListMempool) isFull(txSize int) error {
	var (
		memSize  = mem.Size()
		txsBytes = mem.TxsBytes()
	)

	if memSize >= mem.config.Size || int64(txSize)+txsBytes > mem.config.MaxTxsBytes {
		return ErrMempoolIsFull{
			memSize, mem.config.Size,
			txsBytes, mem.config.MaxTxsBytes,
		}
	}

	return nil
}

//  在 app 第一次检查 tx 后被调用
// 	该函数的执行逻辑是建立在入参 res.Value 是 protoabci.Response_CheckTx 的基础上的
//	如果 ClistMempool.postCheck 不为空，则调用它对 tx 进行检查，检查通过后，
// 	将 tx 加入到内存池中，否则将其遗弃
func (mem *CListMempool) resCbFirstTime(
	tx []byte,
	peerP2PID gossip.ID,
	res *protoabci.Response,
) {
	switch r := res.Value.(type) {
	case *protoabci.Response_CheckTx: // 如果 Response.Value 是 Response_CheckTx 类型的

		if r.CheckTx.Code == protoabci.CodeTypeOK {
			// 如果 protoabci.Response_CheckTx.CheckTx.Code = protoabci.CodeTypeOk，代表检查通过，
			// 可以添加到 mempool 里了
			// 再次检查 mempool 是否满了
			if err := mem.isFull(len(tx)); err != nil {
				// remove from cache (mempool might have a space later)
				mem.cache.Remove(tx)
				mem.logger.Errorw(err.Error())
				return
			}

			memTx := &mempoolTx{
				height: mem.height,
				tx:     tx,
			}
			memTx.senders.Store(peerP2PID, true)
			// 应该是往内存池里添加这个新 tx 吧
			mem.addTx(memTx)
			mem.logger.Debugw("added good transaction", "tx", txID(tx), "res", r, "height", memTx.height, "total", mem.Size(), )
			// 是通知某个模块，告诉它内存池里有可用的 tx 吗？（是的！）
			mem.notifyTxsAvailable()
		} else {
			// 如果 protoabci.Response_CheckTx.CheckTx.Code != protoabci.CodeTypeOk，代表检查没通过
			mem.logger.Debugw("rejected bad transaction",
				"tx", txID(tx), "peerID", peerP2PID, "res", r)
			mem.cache.Remove(tx)
		}
	default:
		// ignore other messages
	}
}

// 	在 app 对 tx 重新检查以后调用
// 	app 第一次检查 tx 的情况是由 resCbFirstTime 回调处理的。
// 	该函数的执行逻辑是建立在入参 res.Value 是 protoabci.Response_CheckTx 的基础上的
//
func (mem *CListMempool) resCbRecheck(req *protoabci.Request, res *protoabci.Response) {
	switch r := res.Value.(type) {
	case *protoabci.Response_CheckTx:
		// 如果回复的 Value 是 protoabci.Response_CheckTx 类型的，
		// 那么请求的 Value 应该是 Request_CheckTx 类型的
		tx := req.GetCheckTx().Tx
		memTx := mem.recheckCursor.Value.(*mempoolTx)
		if !bytes.Equal(tx, memTx.tx) { // 如果内存池里的重新检查锁定的 tx 与请求里的 tx 不一样，则会 panic
			panic(fmt.Sprintf(
				"Unexpected tx response from proxy during recheck\nExpected %X, got %X",
				memTx.tx,
				tx))
		}

		if r.CheckTx.Code == protoabci.CodeTypeOK {
			// 如果检查通过，则什么也不用做
		} else {
			// 如果检查不通过，tx 不再合法
			mem.logger.Debug("tx is no longer valid", "tx", txID(tx), "res", r)
			// NOTE: we remove tx from the cache because it might be good later
			mem.removeTx(tx, mem.recheckCursor)
		}
		if mem.recheckCursor == mem.recheckEnd {
			mem.recheckCursor = nil
		} else {
			mem.recheckCursor = mem.recheckCursor.Next()
		}
		if mem.recheckCursor == nil {
			// Done!
			mem.logger.Debugw("done rechecking txs")

			if mem.Size() > 0 {
				mem.notifyTxsAvailable()
			}
		}
	default:
		// ignore other messages
	}
}

// 	TxsAvailable consensus 模块会调用该方法监听内存池里的动静
func (mem *CListMempool) TxsAvailable() <-chan struct{} {
	return mem.txsAvailable
}

// notifyTxsAvailable 在往内存池里添加一个 tx 后，会调用此方法，这样在
// 监听 ClistMempool.txsAvailable 的模块就会收到 ClistMempool 状态改
// 变的通知
func (mem *CListMempool) notifyTxsAvailable() {
	if mem.Size() == 0 {
		panic("notified txs available but mempool is empty!")
	}
	if mem.txsAvailable != nil && !mem.notifiedTxsAvailable {
		// 如果 ClistMempool.txsAvailable 为 non-nil，并且 ClistMempool.notifiedTxsAvailable 为 false，
		// 则让 ClistMempool.notifiedTxsAvailable 等于 true，然后往 ClistMempool.txsAvailable 通道里推入
		// 一个空结构体
		mem.notifiedTxsAvailable = true
		select {
		case mem.txsAvailable <- struct{}{}:
		default:
		}
	}
}

// 	ReapMaxBytes 从内存池里逐个取出 tx 放到 txs 里，并将 txs 返回，取的时候需要遵循如下规则：
//	如果 txs 里装的 tx 总大小已经大于 maxBytes 了，则到此为止，并返回 txs
func (mem *CListMempool) ReapMaxBytes(maxBytes int64) types.Txs {
	mem.updateMtx.RLock()
	defer mem.updateMtx.RUnlock()

	// size per tx, and set the initial capacity based off of that.
	// txs := make([]types.Tx, 0, srmath.MinInt(mem.txs.Len(), max/mem.avgTxSize))
	txs := make([]types.Tx, 0, mem.txs.Len())
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)

		// 将内存池里的 tx 按顺序取出来，并放到 txs 中，然后计算 txs 的大小
		dataSize := types.ComputeProtoSizeForTxs(append(txs, memTx.tx))

		// 如果此时 txs 里装的 tx 总大小已经大于 maxBytes 了，则到此为止，并返回 txs
		if maxBytes > -1 && dataSize > maxBytes {
			return txs
		}

		txs = append(txs, memTx.tx)
	}
	return txs
}

// 	ReapMaxTxs 从内存池里取出一定数量的 tx 放到 txs 里，并将 txs 返回
//	取的时候遵循下面这个条件：
//	如果 max 小于 0，则将内存池里的 tx 全部取出放到 txs 里，否则最多只取 max 个 tx
func (mem *CListMempool) ReapMaxTxs(max int) types.Txs {
	mem.updateMtx.RLock()
	defer mem.updateMtx.RUnlock()

	if max < 0 {
		max = mem.txs.Len()
	}

	txs := make([]types.Tx, 0, srmath.MinInt(mem.txs.Len(), max))
	for e := mem.txs.Front(); e != nil && len(txs) <= max; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		txs = append(txs, memTx.tx)
	}
	return txs
}

//	Update 是将共识引擎已经打包的交易从内存池中删除掉
func (mem *CListMempool) Update(
	height int64,
	txs types.Txs,
	deliverTxResponses []*protoabci.ResponseDeliverTx,
	preCheck PreCheckFunc,
) error {
	// 将 ClistMempool.height 字段设为 height
	mem.height = height
	// 将 ClistMempool.notifiedTxsAvailable 字段设为 false
	mem.notifiedTxsAvailable = false

	if preCheck != nil {
		mem.preCheck = preCheck
	}

	for i, tx := range txs {
		if deliverTxResponses[i].Code == protoabci.CodeTypeOK {
			// 将已经 commit 的 tx 放入到缓冲区里
			_ = mem.cache.Push(tx)
		} else {
			// 将未被 commit 的 tx 从缓冲区里删除
			mem.cache.Remove(tx)
		}
		// 将已被 commit 的 tx 从 内存池里删除
		if e, ok := mem.txsMap.Load(TxKey(tx)); ok {
			mem.removeTx(tx, e.(*clist.CElement))
		}
	}

	// 要么重新检查未提交的 txs，看看它们是否无效，要么只是通知有一些 txs 剩余。
	if mem.Size() > 0 {
		if mem.config.Recheck { // 如果需要重新检查
			mem.logger.Debugw("recheck txs", "numtxs", mem.Size(), "height", height)
			mem.recheckTxs()
		} else {
			mem.notifyTxsAvailable()
		}
	}

	return nil
}

// recheckTxs 在调用 ClistMempool.Update 之后，如果内存池里还有 tx，并且配置里
// 要求需要对内存池进行重新检查，则调用此函数
func (mem *CListMempool) recheckTxs() {
	if mem.Size() == 0 {
		panic("recheckTxs is called, but the mempool is empty")
	}

	// 将重新检查的游标移动到内存池里的第一个 tx 上
	mem.recheckCursor = mem.txs.Front()
	// 将重新检查的结束标志设为内存池里的最后一个 tx
	mem.recheckEnd = mem.txs.Back()

	// Push txs to proxyAppConn
	// NOTE: globalCb may be called concurrently.
	for e := mem.txs.Front(); e != nil; e = e.Next() {
		memTx := e.Value.(*mempoolTx)
		mem.proxyAppConn.CheckTxAsync(protoabci.RequestCheckTx{ // 将检查请求发送过代理 app
			Tx:   memTx.tx,
			Type: protoabci.CheckTxType_Recheck,
		})
	}

	mem.proxyAppConn.FlushAsync()
}

//--------------------------------------------------------------------------------

// mempoolTx 是一个成功运行的 transaction
type mempoolTx struct {
	height int64 // 已被验证的 height

	tx types.Tx //

	// 存储了发送给我们这个 tx 的节点身份信息
	// senders: PeerID -> bool
	senders sync.Map
}

// Height returns the height for this transaction
func (memTx *mempoolTx) Height() int64 {
	return atomic.LoadInt64(&memTx.height)
}

//--------------------------------------------------------------------------------

type txCache interface {
	Reset()
	Push(tx types.Tx) bool
	Remove(tx types.Tx)
}

// mapTxCache 维护事务的LRU缓存。由于内存问题，这只存储tx的哈希值。
type mapTxCache struct {
	mtx      sync.Mutex
	size     int
	cacheMap map[[TxKeySize]byte]*list.Element // cacheMap 存储了 TxKey(tx)->list.Element{Value:TxKey(tx)}
	list     *list.List                        // list 里存储了 list.Element{Value:TxKey(tx)}
}

var _ txCache = (*mapTxCache)(nil)

// newMapTxCache returns a new mapTxCache.
func newMapTxCache(cacheSize int) *mapTxCache {
	return &mapTxCache{
		size:     cacheSize,
		cacheMap: make(map[[TxKeySize]byte]*list.Element, cacheSize),
		list:     list.New(),
	}
}

// Reset 清空缓冲区
func (cache *mapTxCache) Reset() {
	cache.mtx.Lock()
	cache.cacheMap = make(map[[TxKeySize]byte]*list.Element, cache.size)
	cache.list.Init()
	cache.mtx.Unlock()
}

// Push adds the given tx to the cache and returns true. It returns
// false if tx is already in the cache.
func (cache *mapTxCache) Push(tx types.Tx) bool {
	cache.mtx.Lock()
	defer cache.mtx.Unlock()

	// Use the tx hash in the cache
	txHash := TxKey(tx)
	if moved, exists := cache.cacheMap[txHash]; exists {
		// 如果要添加的 tx 已经存在于缓冲区里，则返回 false
		cache.list.MoveToBack(moved)
		return false
	}

	// 如果缓冲区满了，就将缓冲区里的第一个 tx 给删除掉
	if cache.list.Len() >= cache.size {
		popped := cache.list.Front()
		if popped != nil {
			poppedTxHash := popped.Value.([TxKeySize]byte)
			delete(cache.cacheMap, poppedTxHash)
			cache.list.Remove(popped)
		}
	}
	e := cache.list.PushBack(txHash)
	// e 是 list.Element：
	//	type Element struct {
	//    next, prev *Element
	//    list       *List
	//    Value      interface{}
	//	}
	// 其中 e.Value 存储了 TxKey(tx)，TxKey(tx) 返回一个长度为 32 的字节数组
	cache.cacheMap[txHash] = e
	return true
}

// Remove removes the given tx from the cache.
func (cache *mapTxCache) Remove(tx types.Tx) {
	cache.mtx.Lock()
	txHash := TxKey(tx)
	popped := cache.cacheMap[txHash]
	delete(cache.cacheMap, txHash)
	if popped != nil {
		cache.list.Remove(popped)
	}

	cache.mtx.Unlock()
}

type nopTxCache struct{}

var _ txCache = (*nopTxCache)(nil)

func (nopTxCache) Reset()             {}
func (nopTxCache) Push(types.Tx) bool { return true }
func (nopTxCache) Remove(types.Tx)    {}

//--------------------------------------------------------------------------------

// TxKey is the fixed length array hash used as the key in maps.
func TxKey(tx types.Tx) [TxKeySize]byte {
	return sha256.Sum256(tx)
}

// txID 与 TxKey 计算出来的值都是一样的，不同的是：
// TxKey 返回的是长度为 32 的字节数组，txId 返回的是字节切片
func txID(tx []byte) []byte {
	return types.Tx(tx).Hash()
}
