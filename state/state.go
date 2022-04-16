package state

import (
	srtime "github.com/232425wxy/BFT/libs/time"
	protostate "github.com/232425wxy/BFT/proto/state"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"github.com/232425wxy/BFT/types"
	"bytes"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"io/ioutil"
	"time"
)

// stateKey 作为存储区块链 state 的 key 值，stateKey -> state
var (
	stateKey = []byte("stateKey")
)

// State 是 SRBFT 共识最新 commit 的区块的简短描述。
// 它保存所有验证新块所需的信息，包括最后一个验证器集和共识参数。
// 所有的字段都是公开的，因此结构可以很容易地序列化，但它们都不应该被直接修改。
// Instead, use state.Copy() or state.NextState(...).
// NOTE: not goroutine-safe.
type State struct {
	// protostate.Version{Consensus{Block uint64, App uint64}, Software string}

	// 区块链的ID，不可变的
	ChainID       string
	InitialHeight int64 // should be 1, not 0, when starting from height 1

	// LastBlockHeight=0 at genesis (ie. block(H=0) does not exist)
	// 上一个区块的高度
	LastBlockHeight int64
	// 上一个区块的ID，BlockID
	LastBlockID   types.BlockID
	LastBlockTime time.Time

	// LastValidators用来验证block.LastCommits。
	// 验证器每次更改时都被单独持久化到数据库中，因此我们可以查询历史验证器集。
	// Note that if s.LastBlockHeight causes a valset change,
	// we set s.LastHeightValidatorsChanged = s.LastBlockHeight + 1 + 1
	// Extra +1 due to nextValSet delay.
	NextValidators *types.ValidatorSet
	// 代表当前 Validator 集合
	Validators *types.ValidatorSet
	// 上一个区块的 Validator 集合
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64 //

	// Consensus parameters used for validating blocks.
	// Changes returned by EndBlock and updated after Reply.
	// 共识参数的配置 主要是 一个区块的大小 一个交易的大小 区块每个部分的大小
	ConsensusParams prototypes.ConsensusParams

	// Merkle root of the results from executing prev block
	LastResultsHash []byte

	// the latest AppHash we've received from calling abci.Reply()
	AppHash []byte
}

// Copy makes a copy of the State for mutating.
func (state State) Copy() State {

	return State{
		ChainID:       state.ChainID,
		InitialHeight: state.InitialHeight,

		LastBlockHeight: state.LastBlockHeight,
		LastBlockID:     state.LastBlockID,
		LastBlockTime:   state.LastBlockTime,

		NextValidators:              state.NextValidators.Copy(),
		Validators:                  state.Validators.Copy(),
		LastValidators:              state.LastValidators.Copy(),
		LastHeightValidatorsChanged: state.LastHeightValidatorsChanged,

		ConsensusParams:                  state.ConsensusParams,

		AppHash: state.AppHash,

		LastResultsHash: state.LastResultsHash,
	}
}

// Equals returns true if the States are identical.
func (state State) Equals(state2 State) bool {
	sbz, s2bz := state.Bytes(), state2.Bytes()
	return bytes.Equal(sbz, s2bz)
}

// Bytes serializes the State using protobuf.
// It panics if either casting to protobuf or serialization fails.
func (state State) Bytes() []byte {
	sm, err := state.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(sm)
	if err != nil {
		panic(err)
	}
	return bz
}

// IsEmpty returns true if the State is equal to the empty State.
func (state State) IsEmpty() bool {
	return state.Validators == nil // XXX can't compare to Empty
}

// ToProto takes the local state type and returns the equivalent proto type
func (state *State) ToProto() (*protostate.State, error) {
	if state == nil {
		return nil, errors.New("state is nil")
	}

	sm := new(protostate.State)

	sm.ChainID = state.ChainID
	sm.InitialHeight = state.InitialHeight
	sm.LastBlockHeight = state.LastBlockHeight

	sm.LastBlockID = state.LastBlockID.ToProto()
	sm.LastBlockTime = state.LastBlockTime
	vals, err := state.Validators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.Validators = vals

	nVals, err := state.NextValidators.ToProto()
	if err != nil {
		return nil, err
	}
	sm.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := state.LastValidators.ToProto()
		if err != nil {
			return nil, err
		}
		sm.LastValidators = lVals
	}

	sm.LastHeightValidatorsChanged = state.LastHeightValidatorsChanged
	sm.ConsensusParams = state.ConsensusParams
	sm.LastResultsHash = state.LastResultsHash
	sm.AppHash = state.AppHash

	return sm, nil
}

// StateFromProto takes a state proto message & returns the local state type
func StateFromProto(pb *protostate.State) (*State, error) { //nolint:golint
	if pb == nil {
		return nil, errors.New("nil State")
	}

	state := new(State)

	state.ChainID = pb.ChainID
	state.InitialHeight = pb.InitialHeight

	bi, err := types.BlockIDFromProto(&pb.LastBlockID)
	if err != nil {
		return nil, err
	}
	state.LastBlockID = *bi
	state.LastBlockHeight = pb.LastBlockHeight
	state.LastBlockTime = pb.LastBlockTime

	vals, err := types.ValidatorSetFromProto(pb.Validators)
	if err != nil {
		return nil, err
	}
	state.Validators = vals

	nVals, err := types.ValidatorSetFromProto(pb.NextValidators)
	if err != nil {
		return nil, err
	}
	state.NextValidators = nVals

	if state.LastBlockHeight >= 1 { // At Block 1 LastValidators is nil
		lVals, err := types.ValidatorSetFromProto(pb.LastValidators)
		if err != nil {
			return nil, err
		}
		state.LastValidators = lVals
	} else {
		state.LastValidators = types.NewValidatorSet(nil)
	}

	state.LastHeightValidatorsChanged = pb.LastHeightValidatorsChanged
	state.ConsensusParams = pb.ConsensusParams
	state.LastResultsHash = pb.LastResultsHash
	state.AppHash = pb.AppHash

	return state, nil
}

//------------------------------------------------------------------------
// 根据最新的状态创建一个区块

// 	MakeBlock 根据给定的交易列表 commit 信息和相关证据，创造一个区块。注意：
//	该方法还需要显示地传入 proposer 的地址，因为 state 不知道当前的 proposer 是谁
func (state State) MakeBlock(
	height int64,
	txs []types.Tx,
	commit *types.Reply,
	proposerAddress []byte,
) (*types.Block, *types.PartSet) {

	// Build base block with block data.
	block := types.MakeBlock(height, txs, commit)

	// Set time.
	var timestamp time.Time
	if height == state.InitialHeight {
		timestamp = state.LastBlockTime // 创世区块的时间
	} else {
		// 计算所有提交时间的中间时间，计算规则如下：
		// 先将所有 commit 的时间按从小到达进行排序，然后计算所有 commit 的 Validator 的投票
		// 权之和，然后前 k 个 commit 的投票权等于总投票权的一半，那么就以第 k 个 commit 的时间
		// 作为区块的时间戳
		timestamp = MedianTime(commit, state.LastValidators)
	}

	// Fill rest of header with state data.
	block.Header.Populate(
		state.ChainID,
		timestamp, state.LastBlockID,
		state.Validators.Hash(), state.NextValidators.Hash(),
		types.HashConsensusParams(state.ConsensusParams), state.AppHash, state.LastResultsHash,
		proposerAddress,
	)

	return block, block.MakePartSet(types.BlockPartSizeBytes)
}

// MedianTime 计算给定 Reply 的中值时间(基于投票消息的时间戳字段)和相应的验证器集。
// 计算的时间总是在诚实的 Validator 发送的投票的时间戳之间，也就是说，一个恶意的 Validator
// 不能随意增加或减少计算的值。
func MedianTime(commit *types.Reply, validators *types.ValidatorSet) time.Time {
	weightedTimes := make([]*srtime.WeightedTime, len(commit.Signatures))
	totalVotingPower := int64(0)

	for i, commitSig := range commit.Signatures {
		if commitSig.Absent() {
			continue
		}
		_, validator := validators.GetByAddress(commitSig.ValidatorAddress)
		// If there's no condition, TestValidateBlockCommit panics; not needed normally.
		if validator != nil {
			totalVotingPower += int64(validator.Address.Bytes()[0])
			weightedTimes[i] = srtime.NewWeightedTime(commitSig.Timestamp, int64(validator.Address.Bytes()[0]))
		}
	}

	return srtime.WeightedMedian(weightedTimes, totalVotingPower)
}

//------------------------------------------------------------------------
// Genesis

// MakeGenesisStateFromFile reads and unmarshals state from the given
// file.
//
// Used during replay and in tests.
func MakeGenesisStateFromFile(genDocFile string) (State, error) {
	genDoc, err := MakeGenesisDocFromFile(genDocFile)
	if err != nil {
		return State{}, err
	}
	return MakeGenesisState(genDoc)
}

// MakeGenesisDocFromFile reads and unmarshals genesis doc from the given file.
func MakeGenesisDocFromFile(genDocFile string) (*types.GenesisDoc, error) {
	genDocJSON, err := ioutil.ReadFile(genDocFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read GenesisDoc file: %v", err)
	}
	genDoc, err := types.GenesisDocFromJSON(genDocJSON) // 从 json 格式的字符串转换成 GenesisDoc 数据结构
	if err != nil {
		return nil, fmt.Errorf("error reading GenesisDoc: %v", err)
	}
	return genDoc, nil
}

// MakeGenesisState creates state from types.GenesisDoc.
func MakeGenesisState(genDoc *types.GenesisDoc) (State, error) {
	err := genDoc.ValidateAndComplete()
	if err != nil {
		return State{}, fmt.Errorf("error in genesis file: %v", err)
	}

	var validatorSet, nextValidatorSet *types.ValidatorSet
	if genDoc.Validators == nil {
		validatorSet = types.NewValidatorSet(nil)
		nextValidatorSet = types.NewValidatorSet(nil)
	} else {
		validators := make([]*types.Validator, len(genDoc.Validators))
		for i, val := range genDoc.Validators {
			validators[i] = types.NewValidator(val.PubKey, val.Power)
		}
		validatorSet = types.NewValidatorSet(validators)
		nextValidatorSet = types.NewValidatorSet(validators).CopyIteratePrimary(types.BlockID{}, 1)
	}

	return State{
		ChainID:       genDoc.ChainID,
		InitialHeight: genDoc.InitialHeight,

		LastBlockHeight: 0,
		LastBlockID:     types.BlockID{},
		LastBlockTime:   genDoc.GenesisTime,

		NextValidators:              nextValidatorSet,
		Validators:                  validatorSet,
		LastValidators:              types.NewValidatorSet(nil),
		LastHeightValidatorsChanged: genDoc.InitialHeight,

		ConsensusParams:                  *genDoc.ConsensusParams,

		AppHash: genDoc.AppHash,
	}, nil
}
