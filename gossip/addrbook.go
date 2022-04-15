package gossip

import (
	"BFT/crypto"
	"BFT/libs/log"
	srrand "BFT/libs/rand"
	"BFT/libs/service"
	"BFT/libs/tempfile"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/minio/highwayhash"
	"os"
	"sync"
	"time"
)

const (
	// dumpAddressInterval 每隔两分钟将地址簿里的内容存到硬盘上
	dumpAddressInterval = time.Minute * 2
)

type AddrBook struct {
	service.BaseService

	// 可并发访问
	mtx        sync.Mutex
	rand       *srrand.Rand
	ourAddrs   map[string]struct{} // 键是：“id@ip:port”
	addrLookup map[ID]*knownAddress

	// 下面的变量在实例化 AddrBook 以后就不可变了
	filePath          string
	key               string
	routabilityStrict bool
	hashKey           []byte // 实例化 AddrBook 以后，hashKey 的长度为 32 字节
}

// newHashKey 用来初始化 AddrBook.hashKey
func newHashKey() []byte {
	result := make([]byte, highwayhash.Size) // result 的长度是 32 字节
	_, _ = crand.Read(result)
	return result
}

// NewAddrBook 实例化一个地址簿
func NewAddrBook(filePath string, routabilityStrict bool) *AddrBook {
	am := &AddrBook{
		rand:              srrand.NewRand(),
		ourAddrs:          make(map[string]struct{}),
		addrLookup:        make(map[ID]*knownAddress),
		filePath:          filePath,
		routabilityStrict: routabilityStrict,
		hashKey:           newHashKey(),
	}
	am.init()
	am.BaseService = *service.NewBaseService(log.NewCRLogger("info").With("module", "AddrBook"), "AddrBook", am)
	return am
}

func (a *AddrBook) init() {
	// 24 个 16 进制数，每两个 16 进制数的长度为 1 个字节，所以 AddrBook.key 的长度为 12 字节
	a.key = crypto.CRandHex(24)
}

// OnStart 实现 service.Service 接口
func (a *AddrBook) OnStart() error {
	_ = a.BaseService.OnStart()
	a.loadFromFile(a.filePath)

	go a.saveRoutine()

	return nil
}

// OnStop 实现 service.Service 接口
func (a *AddrBook) OnStop() {
	a.BaseService.OnStop()
}

// AddOurAddress 将我们的一个地址添加到 .ourAddrs 中
func (a *AddrBook) AddOurAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	a.Logger.Infow("Add our address to book", "addr", addr)
	a.ourAddrs[addr.String()] = struct{}{}
}

// AddAddress 实现 AddrBook 接口
// 往地址簿中里添加地址
func (a *AddrBook) AddAddress(addr *NetAddress) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	return a.addAddress(addr)
}

// RemoveAddress 实现 AddrBook 接口
// 从地址簿里删除地址
func (a *AddrBook) RemoveAddress(addr *NetAddress) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	a.removeAddress(addr)
}

// Save 将地址簿中的内容存储到硬盘上
func (a *AddrBook) Save() {
	a.saveToFile(a.filePath) // thread safe
}

func (a *AddrBook) saveRoutine() {
	// 每隔两分钟，就将地址簿里的内容存到硬盘上
	saveFileTicker := time.NewTicker(dumpAddressInterval)
out:
	for {
		select {
		case <-saveFileTicker.C:
			a.saveToFile(a.filePath)
		case <-a.Quit():
			break out
		}
	}
	saveFileTicker.Stop()
	a.saveToFile(a.filePath)
}

// addAddress 将地址添加到地址簿中，添加之前，会进行一些判断：
//	1. 判断地址簿是否为 nil，判断源地址 src 是否为 nil，任何一个是的话，返回错误
//	2. 判断地址是否合法，不合法返回错误
//	3. 判断地址是否是自己的地址，是的话返回错误
//	4. 判断地址是否可路由，不可路由的话返回错误
func (a *AddrBook) addAddress(addr *NetAddress) error {
	if err := addr.Valid(); err != nil {
		return ErrAddrBookInvalidAddr{Addr: addr, AddrErr: err}
	}

	if _, ok := a.ourAddrs[addr.String()]; ok {
		return ErrAddrBookSelf{addr}
	}

	if a.routabilityStrict && !addr.Routable() {
		return ErrAddrBookNonRoutable{addr}
	}

	ka := a.addrLookup[addr.ID]
	if ka != nil {

	} else {
		ka = newKnownAddress(addr)
	}

	a.addrLookup[ka.ID()] = ka
	return nil
}

// removeAddress 将指定地址从地址簿中删除
func (a *AddrBook) removeAddress(addr *NetAddress) {
	ka := a.addrLookup[addr.ID]
	if ka == nil {
		return
	}
	a.Logger.Infow("Remove address from book", "addr", addr)
	delete(a.addrLookup, ka.ID())
}

type addrBookJSON struct {
	Key   string              `json:"key"`
	Addrs []*knownAddress `json:"addrs"`
}

// saveToFile 将地址簿保存到硬盘上
func (a *AddrBook) saveToFile(filePath string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	//a.Logger.Info("Saving AddrBook to file", "size", a.size())

	addrs := make([]*knownAddress, 0, len(a.addrLookup))
	for _, ka := range a.addrLookup {
		addrs = append(addrs, ka)
	}
	aJSON := &addrBookJSON{
		Key:   a.key,
		Addrs: addrs,
	}

	jsonBytes, err := json.MarshalIndent(aJSON, "", "\t")
	if err != nil {
		a.Logger.Errorw("Failed to save AddrBook to file", "err", err)
		return
	}
	err = tempfile.WriteFileAtomic(filePath, jsonBytes, 0644)
	if err != nil {
		a.Logger.Errorw("Failed to save AddrBook to file", "file", filePath, "err", err)
	}
}

// loadFromFile 从指定文件中加载地址簿
func (a *AddrBook) loadFromFile(filePath string) bool {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}

	r, err := os.Open(filePath)
	if err != nil {
		panic(fmt.Sprintf("Error opening file %s: %v", filePath, err))
	}
	defer r.Close()
	aJSON := &addrBookJSON{}
	dec := json.NewDecoder(r)
	err = dec.Decode(aJSON)
	if err != nil {
		panic(fmt.Sprintf("Error reading file %s: %v", filePath, err))
	}

	a.key = aJSON.Key
	for _, ka := range aJSON.Addrs {
		a.addrLookup[ka.ID()] = ka
	}
	return true
}
