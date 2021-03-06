package kvstore

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/232425wxy/BFT/abci/example/code"
	"github.com/232425wxy/BFT/abci/types"
	"github.com/232425wxy/BFT/crypto/srhash"
	hex_bytes "github.com/232425wxy/BFT/libs/bytes"
	"github.com/232425wxy/BFT/libs/cmap"
	protoabci "github.com/232425wxy/BFT/proto/abci"
)

var (
	ValUpdateChan = make(chan []protoabci.ValidatorUpdate)
)

// ValueIndex 为了能在查询 key-value 的时候直到 Index
type ValueIndex struct {
	Key []byte `json:"key"`
	Value []byte `json:"value"`
	Index int64  `json:"index"`
	Height int64 `json:"height"`
}

type State struct {
	db      *cmap.CMap
	Size    int64  `json:"size"`   // 代表 db 里面存储的 transaction 数量
	Height  int64  `json:"height"` // 区块高度
	AppHash []byte `json:"app_hash"`
}

func hashTx(tx []byte) string {
	bz := srhash.Sum(tx)
	hbz := hex_bytes.HexBytes(bz)
	return hbz.String()
}

//---------------------------------------------------

var _ types.Application = (*Application)(nil)

type Application struct {
	types.BaseApplication
	ValUpdates []protoabci.ValidatorUpdate

	state        State
}

func NewApplication() *Application {
	state := State{db: cmap.NewCMap()}
	app :=  &Application{state: state}
	go app.updateValUpdateRoutine()
	return app
}

// Info 重写了 BaseApplication 的 Info 方法
func (app *Application) Info(req protoabci.RequestInfo) (resInfo protoabci.ResponseInfo) {
	return protoabci.ResponseInfo{
		Data:             fmt.Sprintf("{\"size\":%v}", app.state.Size),
		LastBlockHeight:  app.state.Height,
		LastBlockAppHash: app.state.AppHash,
	}
}

func (app *Application) EndBlock(req protoabci.RequestEndBlock) protoabci.ResponseEndBlock {
	if app.ValUpdates != nil {
		if app.ValUpdates[0].Height == req.Height {
			cv := copyValidatorUpdates(app.ValUpdates)
			app.ValUpdates = nil
			//for _, v := range cv {
			//	fmt.Println(">>>>>>>>>>>>>>>>>>>>>>------------------------", v.Type, v.Power)
			//}
			return protoabci.ResponseEndBlock{ValidatorUpdates: cv}
		}
		app.ValUpdates = nil
	}
	return protoabci.ResponseEndBlock{ValidatorUpdates: nil}
}

// DeliverTx 重写了 BaseApplication 的 DeliverTx 方法，
// req.Tx 要么是 “key=value”，要么就是任意的字节，DeliverTx 直接将
// transaction 存储到 app.state.db 里了
func (app *Application) DeliverTx(req protoabci.RequestDeliverTx) protoabci.ResponseDeliverTx {
	var key, value []byte
	parts := bytes.Split(req.Tx, []byte("="))
	if len(parts) == 2 {
		key, value = parts[0], parts[1]
	} else {
		key, value = req.Tx, req.Tx
	}

	valueIndex := ValueIndex{key, value, app.state.Size + 1, app.state.Height + 1}
	bz, err := json.Marshal(valueIndex)
	if err != nil {
		panic(err)
	}
	app.state.db.Set(hashTx(req.Tx), bz)
	app.state.Size++
	events := []protoabci.Event{
		{
			Type: "KVStore",
			Attributes: []protoabci.EventAttribute{
				{Key: []byte("key"), Value: key, Index: true},
				{Key: []byte("value"), Value: value, Index: true},
			},
		},
	}

	return protoabci.ResponseDeliverTx{Code: code.CodeTypeOK, Events: events}
}

// CheckTx 非常简单的重写了 BaseApplication 的 CheckTx 方法
func (app *Application) CheckTx(req protoabci.RequestCheckTx) protoabci.ResponseCheckTx {
	return protoabci.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *Application) Commit() protoabci.ResponseCommit {
	// 使用memdb -只需返回数据库的大端字节大小
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash // appHash 里放的其实是 app.state.Size，即 transaction 的数量
	app.state.Height++
	//saveState(app.state)
	resp := protoabci.ResponseCommit{Data: appHash}
	return resp
}

// Query 根据要查询的 key 找到对应的 value 并返回 protoabci.ResponseQuery 查询结果
func (app *Application) Query(reqQuery protoabci.RequestQuery) (resQuery protoabci.ResponseQuery) {
	value := app.state.db.Get(reqQuery.Data)
	v, _ := value.([]byte)
	if v == nil {
		resQuery.Log = "does not exist"
	} else {
		resQuery.Log = "exists"
	}
	var valueIndex ValueIndex
	json.Unmarshal(v, &valueIndex)
	resQuery.Key = valueIndex.Key
	resQuery.Value = valueIndex.Value
	resQuery.Height = valueIndex.Height
	resQuery.Encode = "base64"
	resQuery.Index = valueIndex.Index

	return resQuery
}

func (app *Application) updateValUpdateRoutine() {
	for {
		select {
		case valUpdate := <- ValUpdateChan:
			app.ValUpdates = valUpdate
		}
	}
}

func copyValidatorUpdates(vus []protoabci.ValidatorUpdate) []protoabci.ValidatorUpdate {
	if len(vus) == 0 {
		return nil
	}
	us := make([]protoabci.ValidatorUpdate, len(vus))
	for i := 0; i <len(us); i++ {
		us[i] = protoabci.ValidatorUpdate{
			PubKey: vus[i].PubKey,
			Power:   vus[i].Power,
			Type:    vus[i].Type,
			Height:  vus[i].Height,
		}
	}
	return us
}