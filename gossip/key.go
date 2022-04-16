package gossip

import (
	srcrypto "github.com/232425wxy/BFT/crypto"
	sred25519 "github.com/232425wxy/BFT/crypto/ed25519"
	srjson "github.com/232425wxy/BFT/libs/json"
	sros "github.com/232425wxy/BFT/libs/os"
	"encoding/hex"
	"io/ioutil"
)

// ID 是节点 16 进制编码的公钥地址，因为公钥地址的长度是 20 字节，并
// 且因为一个字节得 2 个 16 进制的数字才能表示，因此如果转换为 16 进制
// 编码的字符串，长度会变为 40
type ID string

type IDS []ID

func (ids IDS) Len() int {
	return len(ids)
}

func (ids IDS) Swap(i, j int) {
	ids[i], ids[j] = ids[j], ids[i]
}

func (ids IDS) Less(i, j int) bool {
	return ids[i] < ids[j]
}

// IDByteLength 是 srcrypto.Address 的长度，目前设为 20 字节长度，
// 注意：这是字节形式的长度，如果转换为 16 进制的字符串，则长度要乘以 2
const IDByteLength = srcrypto.AddressSize

// NodeKey 是节点唯一的 persistent key，它包含节点用来进行 authentication 的私钥
type NodeKey struct {
	PrivKey srcrypto.PrivKey `json:"priv_key"` // 节点的私钥
}

// ID 将节点公钥转换为地址切片，然后再将地址切片编码为 16 进制的字符串
func (nodeKey *NodeKey) ID() ID {
	return PubKeyToID(nodeKey.PubKey())
}

// PubKey 返回节点的公钥
func (nodeKey *NodeKey) PubKey() srcrypto.PubKey {
	return nodeKey.PrivKey.PubKey()
}

// PubKeyToID 将节点公钥转换为 ID，需要经过以下几个步骤：
//		1. 计算节点公钥的哈希值，截取前 20 字节作为其地址
//		2. 将第一步所得的地址利用 16 进制编码，转换为字符串
func PubKeyToID(pubKey srcrypto.PubKey) ID {
	return ID(hex.EncodeToString(pubKey.Address()))
}

// LoadOrGenNodeKey 尝试从给定的 filePath 加载 NodeKey,
// 如果该文件不存在，则生成并保存一个新的 NodeKey，NodeKey 的
// 私钥是随机生成的
func LoadOrGenNodeKey(filePath string) (*NodeKey, error) {
	if sros.FileExists(filePath) {
		nodeKey, err := LoadNodeKey(filePath)
		if err != nil {
			return nil, err
		}
		return nodeKey, nil
	}

	privKey := sred25519.GenPrivKey()
	nodeKey := &NodeKey{
		PrivKey: privKey,
	}

	if err := nodeKey.SaveAs(filePath); err != nil {
		return nil, err
	}

	return nodeKey, nil
}

// LoadNodeKey 加载位于 filePath 中的 NodeKey
func LoadNodeKey(filePath string) (*NodeKey, error) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	nodeKey := new(NodeKey)
	err = srjson.Unmarshal(jsonBytes, nodeKey)
	if err != nil {
		return nil, err
	}
	return nodeKey, nil
}

// SaveAs 将NodeKey保存到filePath
func (nodeKey *NodeKey) SaveAs(filePath string) error {
	jsonBytes, err := srjson.Marshal(nodeKey)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filePath, jsonBytes, 0600)
	if err != nil {
		return err
	}
	return nil
}
