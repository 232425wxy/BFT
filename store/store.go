package store

import (
	protostore "github.com/232425wxy/BFT/proto/store"
	types2 "github.com/232425wxy/BFT/proto/types"
	"fmt"
	"strconv"
	"sync"

	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"

	"github.com/232425wxy/BFT/types"
)

type BlockStore struct {
	db dbm.DB

	mtx sync.RWMutex

	// height - base + 1：表示 BlockStore 中存储的区块个数
	base   int64 // BlockStore 中存储的第一个区块的高度
	height int64 // BlockStore 中存储的最新区块的高度
}

// NewBlockStore 从给定的 DB 里取出 BlockStoreState，BlockStoreState 中含有 Base 和 Height 两个字段，
// 根据 BlockStoreState 内存储的信息和 DB 初始化 BlockStore
func NewBlockStore(db dbm.DB) *BlockStore {
	bs := LoadBlockStoreState(db)
	return &BlockStore{
		base:   bs.Base,
		height: bs.Height,
		db:     db,
	}
}

// Base 返回第一个区块的高度
func (bs *BlockStore) Base() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.base
}

// Height 返回最近一个区块的高度
func (bs *BlockStore) Height() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.height
}

// Size 返回 BlockStore 中存储的区块数
func (bs *BlockStore) Size() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	if bs.height == 0 {
		return 0
	}
	return bs.height - bs.base + 1
}

// LoadBaseMeta 从硬盘里加载第一个区块的元数据
func (bs *BlockStore) LoadBaseMeta() *types.BlockMeta {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	if bs.base == 0 {
		return nil
	}
	return bs.LoadBlockMeta(bs.base)
}

// LoadBlock 从硬盘里获取指定高度的完整区块信息
func (bs *BlockStore) LoadBlock(height int64) *types.Block {
	var blockMeta = bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}

	pbb := new(types2.Block)
	buf := []byte{}
	for i := 0; i < int(blockMeta.BlockID.PartSetHeader.Total); i++ {
		// 构建 []byte("P:height:i") 键，从 硬盘里加载 Block Part
		part := bs.LoadBlockPart(height, i)
		// 如果一个区块的部分丢失了，则认为整个区块都丢失了
		if part == nil {
			return nil
		}
		buf = append(buf, part.Bytes...)
	}
	// 解码区块的字节数据
	err := proto.Unmarshal(buf, pbb)
	if err != nil {
		panic(fmt.Sprintf("Error reading block: %v", err))
	}

	// 将 protobuf 类型的 Block 转换成自定义类型的 Block
	block, err := types.BlockFromProto(pbb)
	if err != nil {
		panic(fmt.Errorf("error from proto block: %w", err))
	}

	return block
}

// LoadBlockByHash 根据给定的区块哈希值，从硬盘中加载完整的区块信息：
//	1. 先根据给定的区块哈希值从硬盘中获取区块高度
//	2. 根据获取到的区块高度，从硬盘中加载完整的区块信息
func (bs *BlockStore) LoadBlockByHash(hash []byte) *types.Block {
	// 构建 DB 中存储区块的 Block 哈希值：[]byte("BH:hash")，
	// 根据键值从硬盘中加载区块高度信息
	bz, err := bs.db.Get(calcBlockHashKey(hash))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}

	s := string(bz)
	height, err := strconv.ParseInt(s, 10, 64)

	if err != nil {
		panic(fmt.Sprintf("failed to extract height from %s: %v", s, err))
	}
	return bs.LoadBlock(height)
}

// LoadBlockPart 根据给定的区块高度和区块 Part 的索引值，获取对应的 Part
func (bs *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	var pbpart = new(types2.Part)

	// 构建从 DB 中获取 Block Part 的键值：[]byte("P:height:index")，
	// 根据计算得来的键值，从硬盘中加载 Block Part
	bz, err := bs.db.Get(calcBlockPartKey(height, index))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}

	// 对 Part 的字节数据进行解码
	err = proto.Unmarshal(bz, pbpart)
	if err != nil {
		panic(fmt.Errorf("unmarshal to prototypes.Part failed: %w", err))
	}
	// 将 protobuf 形式的 Part 转换成自定义的 Part
	part, err := types.PartFromProto(pbpart)
	if err != nil {
		panic(fmt.Sprintf("Error reading block part: %v", err))
	}

	return part
}

// LoadBlockMeta 根据给定的区块高度获取区块元数据
func (bs *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	var pbbm = new(types2.BlockMeta)
	// 获取 DB 中存储的 BlockMeta 字节数据
	bz, err := bs.db.Get(calcBlockMetaKey(height))

	if err != nil {
		panic(err)
	}

	if len(bz) == 0 {
		return nil
	}

	// 将字节数据解码成 prototypes.BlockMeta
	err = proto.Unmarshal(bz, pbbm)
	if err != nil {
		panic(fmt.Errorf("unmarshal to prototypes.BlockMeta: %w", err))
	}

	// 将 prototypes.BlockMeta 转换成自定义的 BlockMeta
	blockMeta, err := types.BlockMetaFromProto(pbbm)
	if err != nil {
		panic(fmt.Errorf("error from proto blockMeta: %w", err))
	}

	return blockMeta
}

func (bs *BlockStore) LoadBlockReply(height int64) *types.Reply {
	var pbc = new(types2.Reply)
	// 构建 DB 中存储 Reply 的键：[]byte("C:height")，
	// 根据计算的来的键，从硬盘中获取 Reply 字节数据
	bz, err := bs.db.Get(calcBlockCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	// 对 Reply 字节数据进行解码
	err = proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Errorf("error reading block commit: %w", err))
	}
	// 将 protobuf 形式的 Reply 转换为自定义的 Reply
	commit, err := types.ReplyFromProto(pbc)
	if err != nil {
		panic(fmt.Sprintf("Error reading block commit: %v", err))
	}
	return commit
}

func (bs *BlockStore) LoadSeenReply(height int64) *types.Reply {
	var pbc = new(types2.Reply)
	bz, err := bs.db.Get(calcSeenCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Sprintf("error reading block seen commit: %v", err))
	}

	commit, err := types.ReplyFromProto(pbc)
	if err != nil {
		panic(fmt.Errorf("error from proto commit: %w", err))
	}
	return commit
}

// SaveBlock 将 block, blockParts, 和 seenCommit 持久化到底层的数据库
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Reply) {
	if block == nil {
		panic("BlockStore can only save a non-nil block")
	}

	// 获取新区块的高度和哈希值
	height := block.Height
	hash := block.Hash()

	// 如果新区块的高度不等于 BlockStore 中存储的最新区块的高度加一，则直接 panic
	if g, w := height, bs.Height()+1; bs.Base() > 0 && g != w {
		panic(fmt.Sprintf("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}

	// 如果 Block 的 Part 集合不完整，则也会 panic
	if !blockParts.IsComplete() {
		panic("BlockStore can only save complete block part sets")
	}

	// 先往硬盘中存储 Block Part信息
	for i := 0; i < int(blockParts.Total()); i++ {
		part := blockParts.GetPart(i)
		bs.saveBlockPart(height, i, part)
	}

	// 往硬盘里存储区块元数据
	blockMeta := types.NewBlockMeta(block, blockParts)
	pbm := blockMeta.ToProto()
	if pbm == nil {
		panic("nil blockmeta")
	}
	metaBytes := mustEncode(pbm)
	if err := bs.db.Set(calcBlockMetaKey(height), metaBytes); err != nil {
		panic(err)
	}
	if err := bs.db.Set(calcBlockHashKey(hash), []byte(fmt.Sprintf("%d", height))); err != nil {
		panic(err)
	}

	// Save block commit (duplicate and separate from the Block)
	pbc := block.LastReply.ToProto()
	blockCommitBytes := mustEncode(pbc)
	if err := bs.db.Set(calcBlockCommitKey(height-1), blockCommitBytes); err != nil {
		panic(err)
	}

	pbsc := seenCommit.ToProto()
	seenCommitBytes := mustEncode(pbsc)
	if err := bs.db.Set(calcSeenCommitKey(height), seenCommitBytes); err != nil {
		panic(err)
	}

	// Done!
	bs.mtx.Lock()
	bs.height = height
	if bs.base == 0 {
		bs.base = height
	}
	bs.mtx.Unlock()

	// Save new BlockStoreState descriptor. This also flushes the database.
	bs.saveState()
}

func (bs *BlockStore) saveBlockPart(height int64, index int, part *types.Part) {
	pbp, err := part.ToProto()
	if err != nil {
		panic(fmt.Errorf("unable to make part into proto: %w", err))
	}
	partBytes := mustEncode(pbp)
	if err := bs.db.Set(calcBlockPartKey(height, index), partBytes); err != nil {
		panic(err)
	}
}

func (bs *BlockStore) saveState() {
	bs.mtx.RLock()
	bss := protostore.BlockStoreState{
		Base:   bs.base,
		Height: bs.height,
	}
	bs.mtx.RUnlock()
	SaveBlockStoreState(&bss, bs.db)
}

// SaveSeenCommit saves a seen commit, used by e.g. the state sync reactor when bootstrapping node.
func (bs *BlockStore) SaveSeenCommit(height int64, seenCommit *types.Reply) error {
	pbc := seenCommit.ToProto()
	seenCommitBytes, err := proto.Marshal(pbc)
	if err != nil {
		return fmt.Errorf("unable to marshal commit: %w", err)
	}
	return bs.db.Set(calcSeenCommitKey(height), seenCommitBytes)
}

//-----------------------------------------------------------------------------

// calcBlockMetaKey 构建 DB 中存储 BlockMeta 的键：
//	[]byte("H:height")
func calcBlockMetaKey(height int64) []byte {
	return []byte(fmt.Sprintf("H:%v", height))
}

// calcBlockPartKey 构建 DB 中存储 Part 的键：
//	[]byte("P:height:partIndex")
func calcBlockPartKey(height int64, partIndex int) []byte {
	return []byte(fmt.Sprintf("P:%v:%v", height, partIndex))
}

// calcBlockCommitKey 构建 DB 中存储 Reply 的键：
//	[]byte("C:height")
func calcBlockCommitKey(height int64) []byte {
	return []byte(fmt.Sprintf("C:%v", height))
}

func calcSeenCommitKey(height int64) []byte {
	return []byte(fmt.Sprintf("SC:%v", height))
}

// calcBlockHashKey 构建 DB 中存储 Block 的区块哈希键：
//	[]byte("BH:Block-Hash")
func calcBlockHashKey(hash []byte) []byte {
	return []byte(fmt.Sprintf("BH:%x", hash))
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

// SaveBlockStoreState 将 protostore.BlockStoreState 持久化到硬盘里
func SaveBlockStoreState(bsj *protostore.BlockStoreState, db dbm.DB) {
	bytes, err := proto.Marshal(bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal state bytes: %v", err))
	}
	if err := db.SetSync(blockStoreKey, bytes); err != nil {
		panic(err)
	}
}

// LoadBlockStoreState 从硬盘里加载 BlockStoreState 信息
func LoadBlockStoreState(db dbm.DB) protostore.BlockStoreState {
	// 从 DB 里面获取 BlockStore 信息
	bytes, err := db.Get(blockStoreKey) // blockStoreKey := []byte("blockStore")
	if err != nil {
		panic(err)
	}

	if len(bytes) == 0 { // 表示 DB 之前还未存储过 BlockStore 信息
		// 返回一个空的 BlockStoreState
		return protostore.BlockStoreState{
			Base:   0,
			Height: 0,
		}
	}

	var bss protostore.BlockStoreState
	// 解码 DB 里存储的 BlockStore 字节数据
	if err := proto.Unmarshal(bytes, &bss); err != nil {
		panic(fmt.Sprintf("Could not unmarshal bytes: %X", bytes))
	}

	// 如果 DB 中存储的 BlockStoreState.Height 大于 0，并且 BlockStoreState.Base 等于 0，
	// 则让取出来的 BlockStoreState.Base 等于 1
	if bss.Height > 0 && bss.Base == 0 {
		bss.Base = 1
	}
	return bss
}

// mustEncode 将 proto.Message 编码成 []byte 字节数据
func mustEncode(pb proto.Message) []byte {
	bz, err := proto.Marshal(pb)
	if err != nil {
		panic(fmt.Errorf("unable to marshal: %w", err))
	}
	return bz
}
