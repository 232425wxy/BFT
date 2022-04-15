package coretypes

import (
	protoabci "BFT/proto/abci"
	"encoding/json"
	"time"

	"BFT/crypto"
	"BFT/gossip"
	"BFT/libs/bytes"
	tmproto "BFT/proto/types"
	"BFT/types"
)

// List of blocks
type ResultBlockchainInfo struct {
	LastHeight int64              `json:"last_height"`
	BlockMetas []*types.BlockMeta `json:"block_metas"`
}

// Genesis file
type ResultGenesis struct {
	Genesis *types.GenesisDoc `json:"genesis"`
}

// Single block (with meta)
type ResultBlock struct {
	BlockID types.BlockID `json:"block_id"`
	Block   *types.Block  `json:"block"`
}

// Commit and Header
type ResultCommit struct {
	*types.Header `json:"header"`
	*types.Reply `json:"reply"`
	CanonicalCommit    bool `json:"canonical"`
}

// ABCI results from a block
type ResultBlockResults struct {
	Height                int64                          `json:"height"`
	TxsResults            []*protoabci.ResponseDeliverTx `json:"txs_results"`
	BeginBlockEvents      []protoabci.Event              `json:"begin_block_events"`
	EndBlockEvents        []protoabci.Event              `json:"end_block_events"`
	ValidatorUpdates      []protoabci.ValidatorUpdate    `json:"validator_updates"`
}

// NewResultCommit is a helper to initialize the ResultCommit with
// the embedded struct
func NewResultCommit(header *types.Header, reply *types.Reply,
	canonical bool) *ResultCommit {

	return &ResultCommit{
		Header: header,
		Reply: reply,
		CanonicalCommit: canonical,
	}
}

// Info about the node's syncing state
type SyncInfo struct {
	LatestBlockHash   bytes.HexBytes `json:"latest_block_hash"`
	LatestAppHash     bytes.HexBytes `json:"latest_app_hash"`
	LatestBlockHeight int64          `json:"latest_block_height"`
	LatestBlockTime   time.Time      `json:"latest_block_time"`

	EarliestBlockHash   bytes.HexBytes `json:"earliest_block_hash"`
	EarliestAppHash     bytes.HexBytes `json:"earliest_app_hash"`
	EarliestBlockHeight int64          `json:"earliest_block_height"`
	EarliestBlockTime   time.Time      `json:"earliest_block_time"`

	CatchingUp bool `json:"catching_up"`
}

// Info about the node's validator
type ValidatorInfo struct {
	Address     bytes.HexBytes `json:"address"`
	PubKey      crypto.PubKey  `json:"pub_key"`
	VotingPower float64          `json:"voting_power"`
}

// Node Status
type ResultStatus struct {
	NodeInfo      *gossip.NodeInfo `json:"node_info"`
	SyncInfo      SyncInfo     `json:"sync_info"`
	ValidatorInfo ValidatorInfo       `json:"validator_info"`
}

// Info about peer connections
type ResultNetInfo struct {
	Listening bool     `json:"listening"`
	Listeners []string `json:"listeners"`
	NPeers    int      `json:"n_peers"`
	Peers     []Peer   `json:"peers"`
}

// Log from dialing seeds
type ResultDialSeeds struct {
	Log string `json:"log"`
}

// Log from dialing peers
type ResultDialPeers struct {
	Log string `json:"log"`
}

// A peer
type Peer struct {
	NodeInfo         *gossip.NodeInfo `json:"node_info"`
	IsOutbound       bool         `json:"is_outbound"`
	ConnectionStatus gossip.ConnectionStatus `json:"connection_status"`
	RemoteIP         string               `json:"remote_ip"`
}

// Validators for a height.
type ResultValidators struct {
	BlockHeight int64              `json:"block_height"`
	Validators  []*types.Validator `json:"validators"`
	// Count of actual validators in this result
	Count int `json:"count"`
	// Total number of validators
	Total int `json:"total"`
}

// ConsensusParams for given height
type ResultConsensusParams struct {
	BlockHeight     int64                   `json:"block_height"`
	ConsensusParams tmproto.ConsensusParams `json:"consensus_params"`
}

// Info about the consensus state.
// UNSTABLE
type ResultDumpConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
	Peers      []PeerStateInfo `json:"peers"`
}

// UNSTABLE
type PeerStateInfo struct {
	NodeAddress string          `json:"node_address"`
	PeerState   json.RawMessage `json:"peer_state"`
}

// UNSTABLE
type ResultConsensusState struct {
	RoundState json.RawMessage `json:"round_state"`
}

// CheckTx result
type ResultBroadcastTx struct {
	Code      uint32         `json:"code"`
	Data      bytes.HexBytes `json:"data"`
	Log       string         `json:"log"`
	Codespace string         `json:"codespace"`

	Hash bytes.HexBytes `json:"hash"`
}

// CheckTx and DeliverTx results
type ResultBroadcastTxCommit struct {
	CheckTx   protoabci.ResponseCheckTx   `json:"check_tx"`
	DeliverTx protoabci.ResponseDeliverTx `json:"deliver_tx"`
	Hash      bytes.HexBytes              `json:"hash"`
	Height    int64                       `json:"height"`
}

// ResultCheckTx wraps protoabci.ResponseCheckTx.
type ResultCheckTx struct {
	protoabci.ResponseCheckTx
}

// Result of querying for a tx
type ResultTx struct {
	Hash     bytes.HexBytes              `json:"hash"`
	Height   int64                       `json:"height"`
	Index    uint32                      `json:"index"`
	TxResult protoabci.ResponseDeliverTx `json:"tx_result"`
	Tx       types.Tx                    `json:"tx"`
	Proof    types.TxProof               `json:"proof,omitempty"`
}

// Result of searching for txs
type ResultTxSearch struct {
	Txs        []*ResultTx `json:"txs"`
	TotalCount int         `json:"total_count"`
}

// ResultBlockSearch defines the RPC response type for a block search by events.
type ResultBlockSearch struct {
	Blocks     []*ResultBlock `json:"blocks"`
	TotalCount int            `json:"total_count"`
}

// List of mempool txs
type ResultUnconfirmedTxs struct {
	Count      int        `json:"n_txs"`
	Total      int        `json:"total"`
	TotalBytes int64      `json:"total_bytes"`
	Txs        []types.Tx `json:"txs"`
}

// Info abci msg
type ResultABCIInfo struct {
	Response protoabci.ResponseInfo `json:"response"`
}

// Query abci msg
type ResultABCIQuery struct {
	Response protoabci.ResponseQuery `json:"response"`
}

// empty results
type (
	ResultUnsafeFlushMempool struct{}
	ResultUnsafeProfile      struct{}
	ResultSubscribe          struct{}
	ResultUnsubscribe        struct{}
	ResultHealth             struct{}
)

// Event data from a subscription
type ResultEvent struct {
	Query  string              `json:"query"`
	Data   types.CREventData   `json:"data"`
	Events map[string][]string `json:"events"`
}
