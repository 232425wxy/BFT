package gossip

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/232425wxy/BFT/crypto"
	cryptoenc "github.com/232425wxy/BFT/crypto/encoding"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	protogossip "github.com/232425wxy/BFT/proto/gossip"
)

const (
	maxNodeInfoSize = 10240 // 10KB
	maxNumChannels  = 16    // plenty of room for upgrades, for now
)

// MaxNodeInfoSize 返回规定的 NodeInfo 结构体的最大大小，默认是 10KB
func MaxNodeInfoSize() int {
	return maxNodeInfoSize
}

// NodeInfo 是 P2P 握手过程中两个 Peer 之间交换的基本节点信息。
type NodeInfo struct {
	NodeID     ID     `json:"id"` // 验证身份
	ListenAddr string `json:"listen_addr"`

	// 用来检查兼容性
	ChainID  string           `json:"network"`
	Channels srbytes.HexBytes `json:"channels"` // 该节点所知道的所有 channel

	Pubkey crypto.PubKey  `json:"pubkey"`
}


// ID 返回节点的 ID
func (info NodeInfo) ID() ID {
	return info.NodeID
}

// Validate 验证检查自我报告的DefaultNodeInfo是安全的。
// 如果有太多的channel，或者如果有任何重复的channel，如
// 果ListenAddr是错误的，或者ListenAddr是一个主机名，不
// 能解析为某些IP，则返回一个错误。
func (info NodeInfo) Validate() error {

	// ID is already validated.

	// Validate ListenAddr.
	_, err := NewNetAddressString(IDAddressString(info.ID(), info.ListenAddr))
	if err != nil {
		return err
	}

	// Validate Channels - ensure max and check for duplicates.
	if len(info.Channels) > maxNumChannels {
		return fmt.Errorf("info.Channels is too long (%v). Max is %v", len(info.Channels), maxNumChannels)
	}
	channels := make(map[byte]struct{})
	for _, ch := range info.Channels {
		_, ok := channels[ch]
		if ok {
			return fmt.Errorf("info.Channels contains duplicate channel id %v", ch)
		}
		channels[ch] = struct{}{}
	}

	return nil
}

// CompatibleWith 检查两个节点是否兼容。
// 如果两个节点的 Block 版本和 ChainID 一样，并且至少含有一个
// 同样的 channel，则两个节点兼容。
func (info NodeInfo) CompatibleWith(other *NodeInfo) error {
	// nodes must be on the same network
	if info.ChainID != other.ChainID {
		return fmt.Errorf("Peer is on a different blockchain. Got %v, expected %v", other.ChainID, info.ChainID)
	}

	// if we have no channels, we're just testing
	if len(info.Channels) == 0 {
		return nil
	}

	// for each of our channels, check if they have it
	found := false
OUTER_LOOP:
	for _, ch1 := range info.Channels {
		for _, ch2 := range other.Channels {
			if ch1 == ch2 {
				found = true
				break OUTER_LOOP // only need one
			}
		}
	}
	if !found {
		return fmt.Errorf("Peer has no common channels. Our channels: %v ; Peer channels: %v", info.Channels, other.Channels)
	}
	return nil
}

// NetAddress 返回一个从 NodeInfo 派生的 NetAddress - 它包括验证的 Peer ID和
// 自我报告的 ListenAddr。
// 注意：ListenAddr 没有经过身份验证，如果它是出站 Peer，可能与实际拨打的地址不匹配。
func (info NodeInfo) NetAddress() (*NetAddress, error) {
	idAddr := IDAddressString(info.ID(), info.ListenAddr)
	return NewNetAddressString(idAddr)
}

func (info NodeInfo) HasChannel(chID byte) bool {
	return bytes.Contains(info.Channels, []byte{chID})
}

func (info NodeInfo) ToProto() *protogossip.NodeInfo {

	n := new(protogossip.NodeInfo)
	pk, err := cryptoenc.PubKeyToProto(info.Pubkey)
	if err != nil {
		panic(err)
	}

	n.NodeID = string(info.NodeID)
	n.ListenAddr = info.ListenAddr
	n.Network = info.ChainID
	n.Channels = info.Channels
	n.PubKey = pk

	return n
}

func NodeInfoFromProto(pb *protogossip.NodeInfo) (NodeInfo, error) {
	if pb == nil {
		return NodeInfo{}, errors.New("nil node info")
	}
	pk, err := cryptoenc.PubKeyFromProto(pb.PubKey)
	if err != nil {
		panic(err)
	}
	dni := NodeInfo{
		NodeID:     ID(pb.NodeID),
		ListenAddr: pb.ListenAddr,
		ChainID:    pb.Network,
		Channels:   pb.Channels,
		Pubkey:     pk,
	}

	return dni, nil
}
