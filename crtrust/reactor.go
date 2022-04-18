package crtrust

import (
	"github.com/232425wxy/BFT/abci/example/kvstore"
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/gossip"
	"sort"
	"time"
)

/*
为什么要通过计算全局信任值来更新validator集合呢？只通过自己本地的记录信息来更新不行吗？
不行的，因为本地的值仅代表自己对全网节点的看法，例如自己在某一轮没收到primary的block，
可能并不是primary作恶，可能是自己网络出现了故障没有收到，而其他节点都收到了
*/

const (
	WhistleChannel = byte(0x07)
	LocalEvaluationChannel = byte(0x08)
	maxMsgSize     = 1048576
	repChanCapacity = 100
)


type Reactor struct {
	gossip.BaseReactor
	sys *system
	self *peer
	collect map[string]*LocalEvaluation
	byzantines map[string]struct{}  // 不可撤回 TODO -> 希望可撤回，即改过自新
	height int64  // 触发信任评估时的区块高度
}

func NewReactor(Pubkey crypto.PubKey, id gossip.ID) *Reactor {
	r := &Reactor{
		sys:         new(system),
		self:        &peer{
			ID:      id,
			Pubkey: Pubkey,
		},
		collect: make(map[string]*LocalEvaluation),
		byzantines: make(map[string]struct{}),
	}
	r.sys.recommendations = make(map[*peer]float64)
	crPeer := &peer{
		ID:      id,
		Pubkey: Pubkey,
		goods:   0,
		bads:    0,
		trust:   0,
		weight:  0,
	}
	r.sys.recommendations[crPeer] = 0.5
	r.sys.similarities = make(map[string]map[string]float64)
	r.sys.similarities[string(id)] = make(map[string]float64)
	r.sys.tok = time.NewTicker(time.Second * 3)
	r.sys.globalT = make(map[string]float64)
	r.sys.globalT[string(id)] = 0.1  // 初始信任值都设为0.1吧
	go r.sys.evaluateRoutine()
	r.BaseReactor = *gossip.NewBaseReactor("CRTrust", r)
	return r
}

func (r *Reactor) InitPeer(p *gossip.Peer) *gossip.Peer {
	crPeer := &peer{
		ID:      p.ID(),
		Pubkey: p.NodeInfo().Pubkey,
		goods:   0,
		bads:    0,
		trust:   0,
		weight:  0,
	}
	r.sys.recommendations[crPeer] = 0.5  // 初始情况下给每个节点的推荐值都是0.5
	r.sys.similarities[string(crPeer.ID)] =  make(map[string]float64)
	r.sys.globalT[string(p.ID())] = 0.1  // 初始信任值都设为0.1吧
	return p
}

func (r *Reactor) GetChannels() []*gossip.ChannelDescriptor {
	return []*gossip.ChannelDescriptor{
		{
			ID:                  WhistleChannel,
			Priority:            20,
			SendQueueCapacity:   repChanCapacity,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  LocalEvaluationChannel,
			Priority:            15,
			SendQueueCapacity:   repChanCapacity,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

func (r *Reactor) Receive(chID byte, src *gossip.Peer, msgBytes []byte) {
	switch chID {
	case WhistleChannel:
		notice := CanonicalDecodeNotice(msgBytes)
		r.BroadcastTrustMessages(notice.Height)
		r.height = notice.Height
	case LocalEvaluationChannel:
		local := CanonicalDecodeLocalEvaluation(msgBytes)
		r.collect[local.Src] = local  // 收集其他节点的本地推荐意见
		if len(r.collect) == len(r.sys.recommendations) {  // 如果收集齐全网节点的推荐意见，那就可以开始计算全局信任值了
			updates := r.sys.globalTrust(r.collect)
			r.collect = make(map[string]*LocalEvaluation)
			sort.Sort(updates)  // 按信任值从小到大排列
			updates.modify(r.byzantines)  // 想办法传给共识模块
			kvstore.ValUpdateChan <- updates.constructUpdateValidators(len(r.sys.globalT)-len(r.byzantines), r.height)
		}
	}
}

func (r *Reactor) UpdateBads(address crypto.Address) {
	if r.self.Pubkey.Address().String() == address.String() {
		// 自己不对自己做出惩罚
		return
	}
	for p, _ := range r.sys.recommendations {
		if p.Pubkey.Address().String() == address.String() {
			p.bads += 20
		}
	}
}

func (r *Reactor) UpdateGoods() {
	for p, _ := range r.sys.recommendations {
		p.goods += 1
	}
}

func (r *Reactor) BroadcastTrustMessages(height int64) {
	local := &LocalEvaluation{Src: string(r.self.ID), Height: height}
	local.PeerEval = make(map[string]float64)
	for p, rec := range r.sys.recommendations {
		local.PeerEval[string(p.ID)] = rec  // 自己给其他节点的推荐值
	}
	local.PeerEval[string(r.self.ID)] = 0.5  // 自己给自己的推荐值
	r.collect[string(r.self.ID)] = local
	bz := CanonicalEncodeLocalEvaluation(local)
	r.Switch.Broadcast(LocalEvaluationChannel, bz)
	r.Logger.Infow("Broadcast Local Evaluation to other peers")
}

func (r *Reactor) NoticeCalcGlobalTrust(height int64) {
	r.height = height
	r.Switch.Broadcast(WhistleChannel, CanonicalEncodeNotice(&Notice{Src: string(r.self.ID), Height: height}))
	time.Sleep(time.Millisecond * 5)
	r.BroadcastTrustMessages(height)
}
