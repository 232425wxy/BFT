package core

import (
	protoabci "github.com/232425wxy/BFT/proto/abci"
	"github.com/232425wxy/BFT/proxy"
	ctypes "github.com/232425wxy/BFT/rpc/core/types"
	rpctypes "github.com/232425wxy/BFT/rpc/jsonrpc/types"
)

// ABCIQuery queries the application for some information.
func ABCIQuery(
	ctx *rpctypes.Context,
	data string,
) (*ctypes.ResultABCIQuery, error) {
	resQuery, err := env.ProxyAppQuery.QuerySync(protoabci.RequestQuery{
		Data:   data,
	})
	if err != nil {
		return nil, err
	}
	env.Logger.Infow("ABCIQuery", "result", resQuery)
	return &ctypes.ResultABCIQuery{Response: *resQuery}, nil
}

// ABCIInfo gets some info about the application.
func ABCIInfo(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
	resInfo, err := env.ProxyAppQuery.InfoSync(proxy.RequestInfo)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultABCIInfo{Response: *resInfo}, nil
}
