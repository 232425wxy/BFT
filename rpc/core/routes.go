package core

import (
	rpc "github.com/232425wxy/BFT/rpc/jsonrpc/server"
)

// TODO: better system than "unsafe" prefix

// Routes is a map of available routes.
var Routes = map[string]*rpc.RPCFunc{
	// info API
	"status":               rpc.NewRPCFunc(Status, ""),
	"net_info":             rpc.NewRPCFunc(NetInfo, ""),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(Genesis, ""),
	"block":                rpc.NewRPCFunc(Block, "height"),
	"block_by_hash":        rpc.NewRPCFunc(BlockByHash, "hash"),
	"block_results":        rpc.NewRPCFunc(BlockResults, "height"),
	"commit":               rpc.NewRPCFunc(Commit, "height"),
	"check_tx":             rpc.NewRPCFunc(CheckTx, "tx"),
	"tx":                   rpc.NewRPCFunc(Tx, "hash,prove"),
	"tx_search":            rpc.NewRPCFunc(TxSearch, "query,prove,page,per_page,order_by"),
	"block_search":         rpc.NewRPCFunc(BlockSearch, "query,page,per_page,order_by"),
	"validators":           rpc.NewRPCFunc(Validators, "height,page,per_page"),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, ""),
	"consensus_state":      rpc.NewRPCFunc(ConsensusState, ""),
	"consensus_params":     rpc.NewRPCFunc(ConsensusParams, "height"),

	// tx broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx"),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "data"),
	"abci_info":  rpc.NewRPCFunc(ABCIInfo, ""),
}

// AddUnsafeRoutes adds unsafe routes.
func AddUnsafeRoutes() {
	// control API
	Routes["dial_seeds"] = rpc.NewRPCFunc(UnsafeDialSeeds, "seeds")
	Routes["dial_peers"] = rpc.NewRPCFunc(UnsafeDialPeers, "peers,persistent,unconditional,private")
}
