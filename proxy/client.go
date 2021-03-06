package proxy

import (
	abcicli "github.com/232425wxy/BFT/abci/client"
	"github.com/232425wxy/BFT/abci/example/kvstore"
	"github.com/232425wxy/BFT/abci/types"
	"sync"
)


type ClientCreator struct {
	mtx *sync.Mutex
	app types.Application
}

// NewLocalClientCreator returns a ClientCreator for the given app,
// which will be running locally.
func NewLocalClientCreator(app types.Application) *ClientCreator {
	return &ClientCreator{
		mtx: new(sync.Mutex),
		app: app,
	}
}

func (l *ClientCreator) NewABCIClient() (abcicli.Client, error) {
	return abcicli.NewLocalClient(l.mtx, l.app), nil
}


func DefaultClientCreator() *ClientCreator {
	return NewLocalClientCreator(kvstore.NewApplication())
}
