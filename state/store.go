package state

import (
	"errors"
	"fmt"
	srmath "github.com/232425wxy/BFT/libs/math"
	sros "github.com/232425wxy/BFT/libs/os"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	protostate "github.com/232425wxy/BFT/proto/state"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"github.com/232425wxy/BFT/types"
	"github.com/gogo/protobuf/proto"
	dbm "github.com/tendermint/tm-db"
)

const (
	// persist validators every valSetCheckpointInterval blocks to avoid
	// LoadValidators taking too much time.
	// 100000 results in ~ 100ms to get 100 validators (see BenchmarkLoadValidators)
	valSetCheckpointInterval = 100000
)

//------------------------------------------------------------------------

func calcValidatorsKey(height int64) []byte {
	return []byte(fmt.Sprintf("validatorsKey:%v", height))
}

func calcConsensusParamsKey(height int64) []byte {
	return []byte(fmt.Sprintf("consensusParamsKey:%v", height))
}

func calcABCIResponsesKey(height int64) []byte {
	return []byte(fmt.Sprintf("abciResponsesKey:%v", height))
}

//----------------------

//go:generate mockery --case underscore --name Store

// Store defines the state store interface
//
// It is used to retrieve current state and save and load ABCI responses,
// validators and consensus parameters
type Store interface {
	// LoadFromDBOrGenesisDoc loads the most recent state.
	// If the chain is new it will use the genesis doc as the current state.
	LoadFromDBOrGenesisDoc(*types.GenesisDoc) (State, error)
	// Load loads the current state of the blockchain
	Load() (State, error)
	// LoadValidators loads the validator set at a given height
	LoadValidators(int64) (*types.ValidatorSet, error)
	// LoadABCIResponses loads the abciResponse for a given height
	LoadABCIResponses(int64) (*protostate.ABCIResponses, error)
	// LoadConsensusParams loads the consensus params for a given height
	LoadConsensusParams(int64) (prototypes.ConsensusParams, error)
	// Save overwrites the previous state with the updated one
	Save(State) error
	// SaveABCIResponses saves ABCIResponses for a given height
	SaveABCIResponses(int64, *protostate.ABCIResponses) error
}

// dbStore wraps a db (github.com/tendermint/tm-db)
type dbStore struct {
	db dbm.DB
}

var _ Store = (*dbStore)(nil)

// 存储状态的
func NewStore(db dbm.DB) Store {
	return dbStore{db}
}

// LoadStateFromDBOrGenesisDoc 从数据库中加载最新的状态信息，如果数据库里还没有存储状态信息，
// 那么就根据 GenesisDoc 创建一个新的状态信息
func (store dbStore) LoadFromDBOrGenesisDoc(genesisDoc *types.GenesisDoc) (State, error) {
	state, err := store.Load()
	if err != nil {
		return State{}, err
	}

	if state.IsEmpty() {
		var err error
		state, err = MakeGenesisState(genesisDoc)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}

// LoadState loads the State from the database.
func (store dbStore) Load() (State, error) {
	return store.loadState(stateKey)
}

func (store dbStore) loadState(key []byte) (state State, err error) {
	// 获取 “stateKey” 对应的 value
	buf, err := store.db.Get(key)
	if err != nil {
		return state, err
	}
	if len(buf) == 0 { // 代表 dbStore 中还未存储 state
		return state, nil
	}

	sp := new(protostate.State) // protobuf 形式的 state

	err = proto.Unmarshal(buf, sp)
	if err != nil {
		// 存储的数据已损坏！
		sros.Exit(fmt.Sprintf(`LoadState: Data has been corrupted or its spec has changed:
		%v\n`, err))
	}

	sm, err := StateFromProto(sp) // 从 protobuf 形式的 state 转换为 原生的 state
	if err != nil {
		return state, err
	}

	return *sm, nil
}

// Save 将 state 持久化到数据库里，在数据库里，指向 state 的键
// 是 “stateKey”
func (store dbStore) Save(state State) error {
	return store.save(state, stateKey)
}

func (store dbStore) save(state State, key []byte) error {
	// 计算先一个区块的高度
	nextHeight := state.LastBlockHeight + 1
	// 如果上一个区块是创世区块，那么就将其
	if nextHeight == 1 {
		nextHeight = state.InitialHeight

		// 保存下一个区块的所有验证者信息  key为validatorsKey:height
		// value为所有验证者信息
		if err := store.saveValidatorsInfo(nextHeight, nextHeight, state.Validators); err != nil {
			return err
		}
	}
	// 保存下一个区块的共识参数 key为consensusParamsKey:height
	if err := store.saveValidatorsInfo(nextHeight+1, state.LastHeightValidatorsChanged, state.NextValidators); err != nil {
		return err
	}

	// Save next consensus params.
	if err := store.saveConsensusParamsInfo(nextHeight, state.ConsensusParams); err != nil {
		return err
	}

	// State的key为`stateKey` value为State的二进制序列化
	err := store.db.SetSync(key, state.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// LoadABCIResponses loads the ABCIResponses for the given height from the
// database. If not found, ErrNoABCIResponsesForHeight is returned.
//
// This is useful for recovering from crashes where we called app.Reply and
// before we called s.Save(). It can also be used to produce Merkle proofs of
// the result of txs.
func (store dbStore) LoadABCIResponses(height int64) (*protostate.ABCIResponses, error) {
	buf, err := store.db.Get(calcABCIResponsesKey(height))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {

		return nil, ErrNoABCIResponsesForHeight{height}
	}

	abciResponses := new(protostate.ABCIResponses)
	err = abciResponses.Unmarshal(buf)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		sros.Exit(fmt.Sprintf(`LoadABCIResponses: Data has been corrupted or its spec has
                changed: %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return abciResponses, nil
}

// SaveABCIResponses persists the ABCIResponses to the database.
// This is useful in case we crash after app.Reply and before s.Save().
// Responses are indexed by height so they can also be loaded later to produce
// Merkle proofs.
//
// Exposed for testing.
func (store dbStore) SaveABCIResponses(height int64, abciResponses *protostate.ABCIResponses) error {
	var dtxs []*protoabci.ResponseDeliverTx
	// strip nil values,
	for _, tx := range abciResponses.DeliverTxs {
		if tx != nil {
			dtxs = append(dtxs, tx)
		}
	}
	abciResponses.DeliverTxs = dtxs

	bz, err := abciResponses.Marshal()
	if err != nil {
		return err
	}

	err = store.db.SetSync(calcABCIResponsesKey(height), bz)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------

// LoadValidators loads the ValidatorSet for a given height.
// Returns ErrNoValSetForHeight if the validator set can't be found for this height.
func (store dbStore) LoadValidators(height int64) (*types.ValidatorSet, error) {
	valInfo, err := loadValidatorsInfo(store.db, height)
	if err != nil {
		return nil, ErrNoValSetForHeight{height}
	}
	if valInfo.ValidatorSet == nil {

		lastStoredHeight := lastStoredHeightFor(height, valInfo.LastHeightChanged)
		valInfo2, err := loadValidatorsInfo(store.db, lastStoredHeight)
		if err != nil || valInfo2.ValidatorSet == nil {
			return nil,
				fmt.Errorf("couldn't find validators at height %d (height %d was originally requested): %w",
					lastStoredHeight,
					height,
					err,
				)
		}

		vs, err := types.ValidatorSetFromProto(valInfo2.ValidatorSet)
		if err != nil {
			return nil, err
		}

		state, err := store.Load()
		if err != nil {
			return nil, err
		}
		vs.CopyIteratePrimary(state.LastBlockID, 0) // mutate
		vi2, err := vs.ToProto()
		if err != nil {
			return nil, err
		}

		valInfo2.ValidatorSet = vi2
		valInfo = valInfo2
	}

	vip, err := types.ValidatorSetFromProto(valInfo.ValidatorSet)
	if err != nil {
		return nil, err
	}

	return vip, nil
}

func lastStoredHeightFor(height, lastHeightChanged int64) int64 {
	checkpointHeight := height - height%valSetCheckpointInterval
	return srmath.MaxInt64(checkpointHeight, lastHeightChanged)
}

// CONTRACT: Returned ValidatorsInfo can be mutated.
func loadValidatorsInfo(db dbm.DB, height int64) (*protostate.ValidatorsInfo, error) {
	buf, err := db.Get(calcValidatorsKey(height))
	if err != nil {
		return nil, err
	}

	if len(buf) == 0 {
		return nil, errors.New("value retrieved from db is empty")
	}

	v := new(protostate.ValidatorsInfo)
	err = v.Unmarshal(buf)
	if err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		sros.Exit(fmt.Sprintf(`LoadValidators: Data has been corrupted or its spec has changed:
                %v\n`, err))
	}
	// TODO: ensure that buf is completely read.

	return v, nil
}

// saveValidatorsInfo 将 validator 集合持久化到数据库中
func (store dbStore) saveValidatorsInfo(height, lastHeightChanged int64, valSet *types.ValidatorSet) error {
	if lastHeightChanged > height {
		return errors.New("lastHeightChanged cannot be greater than ValidatorsInfo height")
	}
	valInfo := &protostate.ValidatorsInfo{
		LastHeightChanged: lastHeightChanged,
	}
	// Only persist validator set if it was updated or checkpoint height (see
	// valSetCheckpointInterval) is reached.
	if height == lastHeightChanged || height%valSetCheckpointInterval == 0 {
		pv, err := valSet.ToProto()
		if err != nil {
			return err
		}
		valInfo.ValidatorSet = pv
	}

	bz, err := valInfo.Marshal()
	if err != nil {
		return err
	}

	err = store.db.Set(calcValidatorsKey(height), bz)
	if err != nil {
		return err
	}

	return nil
}

//-----------------------------------------------------------------------------

// ConsensusParamsInfo represents the latest consensus params, or the last height it changed

// LoadConsensusParams loads the ConsensusParams for a given height.
func (store dbStore) LoadConsensusParams(height int64) (prototypes.ConsensusParams, error) {
	empty := prototypes.ConsensusParams{}

	paramsInfo, err := store.loadConsensusParamsInfo(height)
	if err != nil {
		return empty, fmt.Errorf("could not find consensus params for height #%d: %w", height, err)
	}

	return paramsInfo.ConsensusParams, nil
}

func (store dbStore) loadConsensusParamsInfo(height int64) (*protostate.ConsensusParamsInfo, error) {
	buf, err := store.db.Get(calcConsensusParamsKey(height))
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, errors.New("value retrieved from db is empty")
	}

	paramsInfo := new(protostate.ConsensusParamsInfo)
	if err = paramsInfo.Unmarshal(buf); err != nil {
		// DATA HAS BEEN CORRUPTED OR THE SPEC HAS CHANGED
		sros.Exit(fmt.Sprintf(`LoadConsensusParams: Data has been corrupted or its spec has changed: %v\n`, err))
	}

	return paramsInfo, nil
}

// saveConsensusParamsInfo persists the consensus params for the next block to disk.
// It should be called from s.Save(), right before the state itself is persisted.
// If the consensus params did not change after processing the latest block,
// only the last height for which they changed is persisted.
func (store dbStore) saveConsensusParamsInfo(nextHeight int64, params prototypes.ConsensusParams) error {
	paramsInfo := &protostate.ConsensusParamsInfo{
		ConsensusParams: params,
	}
	bz, err := paramsInfo.Marshal()
	if err != nil {
		return err
	}

	err = store.db.Set(calcConsensusParamsKey(nextHeight), bz)
	if err != nil {
		return err
	}

	return nil
}
