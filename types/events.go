package types

import (
	srjson "github.com/232425wxy/BFT/libs/json"
	srpubsub "github.com/232425wxy/BFT/libs/pubsub"
	srquery "github.com/232425wxy/BFT/libs/pubsub/query"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	"fmt"
)

// Reserved event types (alphabetically sorted).
const (
	// Block level events for mass consumption by users.
	// These events are triggered from the state package,
	// after a block has been committed.
	// These are also used by the tx indexer for async indexing.
	// All of this data can be fetched through the rpc.
	EventNewBlock            = "NewBlock"
	EventNewBlockHeader      = "NewBlockHeader"
	EventTx                  = "Tx"
	EventValidatorSetUpdates = "ValidatorSetUpdates"

	// Internal consensus events.
	// These are used for testing the consensus state machine.
	// They can also be used to build real-time consensus visualizers.
	EventCompleteProposal = "CompleteProposal"
	EventLock             = "Lock"
	EventNewRound         = "NewRound"
	EventNewRoundStep     = "NewRoundStep"
	EventPolka            = "Polka"
	EventRelock           = "Relock"
	EventTimeoutPropose   = "TimeoutPropose"
	EventTimeoutWait      = "TimeoutWait"
	EventUnlock           = "Unlock"
	EventValidBlock       = "ValidBlock"
	EventVote             = "Vote"

	EventReputation       = "Reputation"
)

// ENCODING / DECODING

// CREventData implements events.EventData.
type CREventData interface {
	// empty interface
}

func init() {
	srjson.RegisterType(EventDataNewBlock{}, "github.com/232425wxy/BFT/event/NewBlock")
	srjson.RegisterType(EventDataNewBlockHeader{}, "github.com/232425wxy/BFT/event/NewBlockHeader")
	srjson.RegisterType(EventDataTx{}, "github.com/232425wxy/BFT/event/Tx")
	srjson.RegisterType(EventDataRoundState{}, "github.com/232425wxy/BFT/event/RoundState")
	srjson.RegisterType(EventDataNewRound{}, "github.com/232425wxy/BFT/event/NewRound")
	srjson.RegisterType(EventDataCompleteProposal{}, "github.com/232425wxy/BFT/event/CompleteProposal")
	srjson.RegisterType(EventDataVote{}, "github.com/232425wxy/BFT/event/Vote")
	srjson.RegisterType(EventDataValidatorSetUpdates{}, "github.com/232425wxy/BFT/event/ValidatorSetUpdates")
	srjson.RegisterType(EventDataString(""), "github.com/232425wxy/BFT/event/ProposalString")
}

// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`

	ResultBeginBlock protoabci.ResponseBeginBlock `json:"result_begin_block"`
	ResultEndBlock   protoabci.ResponseEndBlock   `json:"result_end_block"`
}

type EventDataNewBlockHeader struct {
	Header Header `json:"header"`

	NumTxs           int64                        `json:"num_txs"` // Number of txs in a block
	ResultBeginBlock protoabci.ResponseBeginBlock `json:"result_begin_block"`
	ResultEndBlock   protoabci.ResponseEndBlock   `json:"result_end_block"`
}

// All txs fire EventDataTx
type EventDataTx struct {
	protoabci.TxResult
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height int64  `json:"height"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`
}

type ValidatorInfo struct {
	Address Address `json:"address"`
	Index   int32   `json:"index"`
}

type EventDataNewRound struct {
	Height int64  `json:"height"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`

	Proposer ValidatorInfo `json:"proposer"`
}

type EventDataCompleteProposal struct {
	Height int64  `json:"height"`
	Round  int32  `json:"round"`
	Step   string `json:"step"`

	BlockID BlockID `json:"block_id"`
}

type EventDataVote struct {
	Vote *Vote
}

type EventDataString string

type EventDataValidatorSetUpdates struct {
	ValidatorUpdates []*Validator `json:"validator_updates"`
}

// PUBSUB

const (
	// EventTypeKey is a reserved composite key for event name.
	EventTypeKey = "sr.event"
	// TxHashKey is a reserved key, used to specify transaction's hash.
	// see EventBus#PublishEventTx
	TxHashKey = "tx.hash"
	// TxHeightKey is a reserved key, used to specify transaction block's height.
	// see EventBus#PublishEventTx
	TxHeightKey = "tx.height"

	// BlockHeightKey is a reserved key used for indexing BeginBlock and Endblock
	// events.
	BlockHeightKey = "block.height"
)

var (
	EventQueryNewBlockHeader      = QueryForEvent(EventNewBlockHeader)
	EventQueryNewRoundStep        = QueryForEvent(EventNewRoundStep)
	EventQueryTx                  = QueryForEvent(EventTx)
)

// 为交易 Tx 创建一个查询句柄
func EventQueryTxFor(tx Tx) srpubsub.Query {
	return srquery.MustParse(fmt.Sprintf("%s='%s' AND %s='%X'", EventTypeKey, EventTx, TxHashKey, tx.Hash()))
}

func QueryForEvent(eventType string) srpubsub.Query {
	return srquery.MustParse(fmt.Sprintf("%s='%s'", EventTypeKey, eventType))
}

// BlockEventPublisher 发布所有与区块相关的事件
type BlockEventPublisher interface {
	PublishEventNewBlock(block EventDataNewBlock) error
	PublishEventNewBlockHeader(header EventDataNewBlockHeader) error
	PublishEventTx(EventDataTx) error
	PublishEventValidatorSetUpdates(EventDataValidatorSetUpdates) error
}

type TxEventPublisher interface {
	PublishEventTx(EventDataTx) error
}
