package state

import (
	"bytes"
	"errors"
	"fmt"
	cryptoenc "github.com/232425wxy/BFT/crypto/encoding"
	srlog "github.com/232425wxy/BFT/libs/log"
	"github.com/232425wxy/BFT/mempool"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	protostate "github.com/232425wxy/BFT/proto/state"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"github.com/232425wxy/BFT/proxy"
	"github.com/232425wxy/BFT/types"
)

/*
exection.go 的核心函数是 func (blockExec *BlockExecutor) ApplyBlock(state State, blockID types.BlockID, block *types.Block) (State, int64, error)
ApplyBlock 的功能用一句话概括就是，提交区块，把提交后的状态信息永久持久化到数据库中，具体来说，经历了以下几步：
	1. 首先验证区块的正确性，主要就是验证区块的版本、区块链ID、区块高度、区块中存储的上一个区块信息是否和 state 中存储的一样等基础信息；
	2. 将区块里存储的 tx 传递给代理 app，并根据代理 app 的功能计算区块里有多少个 invalid 和 valid 的 tx，代理 app 收到 tx 后会
	   执行相应的操作（这部分由使用者自己设计，已给出的 kvstore 只是单纯的将 tx 存储到数据库里），然后从区块中获取拜占庭节点犯罪的证据，
	   将这些证据提交给代理 app，由代理 app 决定怎么处理这些拜占庭节点，persistent_kvstore 是给每个拜占庭节点的投票权减一，这样就会
	   改变 validator 集合，接着代理 app 就将更新后的 validator 集合反馈回来；
	3. 将代理 app 就 tx 返回的 Response.DeliverTxs 存储到数据库里，其实就是将交易数据持久化到数据库中；
	4. 拿着代理 app 反馈回来的 validatorUpdates，更新下一个高度的 validator 集合，并更新 validator 的优先级，重新选出 proposer，
	   根据刚刚 commit 的区块更新 State；
	5. 调用 mempool 的 FlushAppConn 相当于是调用代理 app 的 FlushSync 向 ABCI 发送 RequestFlush{} 消息，调用 consensus
       的代理 app 的 CommitSync，向 ABCI 发送 RequestCommit{} 消息，然后得到 ResponseCommit{} 消息，调用 mempool 的
       Update 方法，更新内存池的状态；
	6. 更新证据池的状态......
	7. 将 State 存储到 store 的数据库里。
*/

// BlockExecutor 为正确执行一个区块提供了上下文和附件。
type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	// store 里面存储 state、validators、consensus params
	store Store

	// execute the app against this
	proxyApp *proxy.AppConnConsensus

	// events
	eventBus types.BlockEventPublisher

	// manage the mempool lock during commit
	// and update both with block results after commit.
	mempool mempool.Mempool

	logger srlog.CRLogger
}

type BlockExecutorOption func(executor *BlockExecutor)

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(
	stateStore Store,
	logger srlog.CRLogger,
	proxyApp *proxy.AppConnConsensus,
	mempool mempool.Mempool,
	options ...BlockExecutorOption,
) *BlockExecutor {
	res := &BlockExecutor{
		store:    stateStore,
		proxyApp: proxyApp,
		eventBus: types.NopEventBus{},
		mempool:  mempool,
		logger:   logger,
	}

	for _, option := range options {
		option(res)
	}

	return res
}

func (blockExec *BlockExecutor) Store() Store {
	return blockExec.store
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (blockExec *BlockExecutor) SetEventBus(eventBus types.BlockEventPublisher) {
	blockExec.eventBus = eventBus
}

// CreateProposalBlock 根据共识参数的配置信息，确定区块的数据部分的最大大小，
// 然后从内存池里获取合法交易数据，并将其打包成区块
func (blockExec *BlockExecutor) CreateProposalBlock(
	height int64,
	state State, commit *types.Reply,
	proposerAddr []byte,
) (*types.Block, *types.PartSet) {

	// 区块的最大大小，单位是字节
	maxBytes := state.ConsensusParams.Block.MaxBytes

	// 一个区块数据部分的最大大小等于：区块的最大大小(maxBytes) - 区块的最大开销(maxOverhead) - 最大区块头大小(maxHead) - commit开销(commits)
	maxDataBytes := types.MaxDataBytes(maxBytes, state.Validators.Size())

	txs := blockExec.mempool.ReapMaxBytes(maxDataBytes)

	return state.MakeBlock(height, txs, commit, proposerAddr)
}

// ValidateBlock 根据给定的状态验证给定的块。
func (blockExec *BlockExecutor) ValidateBlock(state State, block *types.Block) error {
	err := validateBlock(state, block)
	if err != nil {
		return err
	}
	return nil
}

// ApplyBlock 根据状态验证块，针对应用执行它，触发相关事件，
// 提交应用，并保存新的状态和响应。它返回新状态和要保留的块高度
// (删除旧块)。它是唯一需要从包外部调用的函数，以处理和提交整个
// 块。它需要一个blockID来避免重新计算parts哈希。
//
//	1. 根据当前状态和区块内容来验证当前区块是否符合要求
//	2. 提交区块内容到ABCI的应用层, 返回应用层的回应
//	3. 根据当前区块的信息，ABCI回应的内容，生成下一个State
//	4. 再次调用ABCI的Commit返回当前APPHASH
//	5. 持久化此处状态同时返回下一次状态的内容。
func (blockExec *BlockExecutor) ApplyBlock(state State, blockID types.BlockID, block *types.Block) (State, error) {
	// 验证区块的正确性
	if err := validateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}
	// 将区块内容提交给ABCI应用层 这里涉及了ABCI的好几个接口函数 BeginBlock, DeliverTx, EndBlock
	// 返回ABCI返回的结果
	abciResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, blockExec.store, state.InitialHeight, )
	if err != nil {
		return state, ErrProxyAppConn(err)
	}


	// 把返回的结果保存到数据中
	if err := blockExec.store.SaveABCIResponses(block.Height, abciResponses); err != nil {
		return state, err
	}

	// 验证 validator 的状态更新
	// abciValUpdates 是 protobuf 类型的
	abciValUpdates := abciResponses.EndBlock.ValidatorUpdates
	err = validateValidatorUpdates(abciValUpdates, state.ConsensusParams.Validator)
	if err != nil {
		return state, fmt.Errorf("error in validator updates: %v", err)
	}

	// 检查通过后，将 protobuf 形式的 validatorUpdates 转换为原生格式的 validatorUpdates
	validatorUpdates, err := types.PB2SR.ValidatorUpdates(abciValUpdates)
	if err != nil {
		return state, err
	}

	// 根据当前状态, 以及ABCI的返回结果和当前区块的内容 返回下一个状态信息
	// 这里的 validatorUpdates 是原生格式的，并且经过了正确性验证
	state, err = updateState(state, blockID, &block.Header, abciResponses, validatorUpdates)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %v", err)
	}

	// Lock mempool, commit app state, update mempoool.
	appHash, err := blockExec.Commit(state, block, abciResponses.DeliverTxs)
	if err != nil {
		return state, fmt.Errorf("commit failed for application: %v", err)
	}

	// Update the app hash and save the state.
	state.AppHash = appHash
	// 将此次状态的内容持久化保存
	if err := blockExec.store.Save(state); err != nil {
		return state, err
	}

	// Events are fired after everything else.
	// NOTE: if we crash between Reply and Save, events wont be fired during replay
	fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses, validatorUpdates)

	return state, nil
}

// Commit 必须是线程安全的，因此在 Commit 开始的时候，会先锁定 mempool，之后的操作如下：
//	1. 调用 mempool 的 FlushAppConn 相当于是调用代理 app 的 FlushSync 向 ABCI 发送
//     RequestFlush{} 消息
//	2. 调用 consensus 的代理 app 的 CommitSync，向 ABCI 发送 RequestCommit{} 消息，
//     然后得到 ResponseCommit{} 消息
//	3. 调用 mempool 的 Update 方法，更新内存池的状态，
func (blockExec *BlockExecutor) Commit(
	state State,
	block *types.Block,
	deliverTxResponses []*protoabci.ResponseDeliverTx,
) ([]byte, error) {
	// 将内存池锁住
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// 调用 mempool 的 FlushAppConn 相当于是调用代理 app 的 FlushSync
	// 向 ABCI 发送 RequestFlush{} 消息
	err := blockExec.mempool.FlushAppConn()
	if err != nil {
		blockExec.logger.Warnw("client error during mempool.FlushAppConn", "err", err)
		return nil, err
	}

	// 调用 consensus 的代理 app 的 CommitSync，
	// 向 ABCI 发送 RequestCommit{} 消息，然后得到 ResponseCommit{} 消息
	res, err := blockExec.proxyApp.CommitSync()
	if err != nil {
		blockExec.logger.Warnw("client error during proxyAppConn.CommitSync", "err", err)
		return nil, err
	}

	// ResponseCommit 没有错误代码，只有 Data 字段表示 app_hash
	blockExec.logger.Infow(
		"updated state",
		"height", block.Height,
		"num_txs", len(block.Txs),
		"app_hash", fmt.Sprintf("%X", res.Data),
	)

	// 那么当区块被 commit 以后，区块里已经被提交确认的 tx 就可以从 mempool 里面被删除了
	err = blockExec.mempool.Update(
		block.Height,
		block.Txs,
		deliverTxResponses,
		// 在 Update 里传入 TxPreCheck(state) 和 TxPostCheck(state)
		// 说白了是想更新 ClistMempool 的 preCheck 和 postCheck 字段
		TxPreCheck(state),
	)

	return res.Data, err
}

//---------------------------------------------------------
// Helper functions for executing blocks and updating state

// execBlockOnProxyApp 主要工作就是将区块里存储的 tx 发送到代理 app 那里去执行，然后拿到反馈结果，更新 validator 集合：
//	1. 构建 ABCI 的响应返回值：protostate.ABCIResponse abciResponses，随后初始化 abciResponses.DeliverTxs
//	2. 构建代理 app 的响应回调函数，这里的回调函数只处理 protoabci.Response_DeliverTx 类型的响应，主要就是判断 app
//	   返回的 Response 中标识 tx 是否 valid，然后分别给 valid 和 invalid 的 tx 计数，然后无论 tx 是否 valid，都
//	   将 app 返回的 tx 放入到 abciResponse.DeliverTxs 中
//	3. 获取上一个区块的 commit 信息，主要包括：在哪个 round 有哪些 validator 对上个区块的 commit 签名了和未签名确认
//	4. 从入参区块 block 中获取拜占庭节点的犯罪证据，比如重复投票
//	5. 将拜占庭节点的罪证传递给代理 app，比如 persistent_kvstore 收到罪证以后，会挨个将拜占庭节点的投票权减一，然后更新
//	   代理 app 处存储的 validator 集合，主要就是更新它们的投票权
// 	6. 还记得第二步构建的代理 app 的响应回调函数吗，接着将 block 里的每个 tx 通过 proxyAppConn.DeliverTxAsync 发送
// 	   给代理 app，例如 kvstore 会直接将 tx 存储在数据库里，然后返回一个 CodeTypeOK 的 Response_DeliverTx 响应，这
//	   时候回调函数开始起作用，来处理该响应，前面我们说过，此处的回调函数只会计数 response 的 code 是否为 CodeTypeOK，是
//	   的话，代表刚刚传给代理 app 的 tx 是 valid，否则就不是 valid，这样分别给 valid 的 tx 和 invalid tx 计数
//	7. 最后从代理 app 那里获取到更新后的 validator 集合，作为下一个高度的共识节点集合
func execBlockOnProxyApp(logger srlog.CRLogger, proxyAppConn *proxy.AppConnConsensus, block *types.Block, store Store, initialHeight int64) (*protostate.ABCIResponses, error) {
	// 初始化合法交易和不合法交易的数量
	var validTxs, invalidTxs = 0, 0
	txIndex := 0
	// 首先构建返回值 ABCIResponse
	abciResponses := new(protostate.ABCIResponses)
	dtxs := make([]*protoabci.ResponseDeliverTx, len(block.Txs))
	abciResponses.DeliverTxs = dtxs

	// 每次提交每一个交易给 ABCI 之后，会调用此函数
	// 这个回调只是统计了在应用层被认为是无效和有效的交易数量
	// 从这里我们也可以看出来，应用层无论决定提交的交易是否有效，我们都会将其打包到区块链中
	proxyCb := func(req *protoabci.Request, res *protoabci.Response) {
		if r, ok := res.Value.(*protoabci.Response_DeliverTx); ok {
			// Blocks may include invalid txs.
			txRes := r.DeliverTx
			if txRes.Code == protoabci.CodeTypeOK { // 如果代理 app 是 kvstore 的话，会一直返回 CodeTypeOk 的 response
				validTxs++
			} else {
				logger.Debugw("invalid tx", "code", txRes.Code, "log", txRes.Log)
				invalidTxs++
			}

			abciResponses.DeliverTxs[txIndex] = txRes
			txIndex++
		}
	}
	// 给代理 app 设置回调函数
	proxyAppConn.SetResponseCallback(proxyCb)

	// 获取上一个区块的 commit 信息，主要包括：在哪个 round 有哪些 validator 对上个区块的 commit 签名了和未签名确认
	commitInfo := getBeginBlockValidatorInfo(block, store, initialHeight)

	// Begin block
	var err error
	pbh := block.Header.ToProto()
	if pbh == nil {
		return nil, errors.New("nil header")
	}

	// 开始调用ABCI的BeginBlock，同时向其提交区块hash，区块头信息，上一个区块的验证者以及拜占庭验证者，
	// 比如我们此处的 ABCI 应用是 persistent_kvstore，那么 ABCI 应用在收到 protoabci.RequestBeginBlock
	// 以后，会更新 validator 集合
	abciResponses.BeginBlock, err = proxyAppConn.BeginBlockSync(protoabci.RequestBeginBlock{
		Hash:                block.Hash(),
		Header:              *pbh,
		LastCommitInfo:      commitInfo,
	})
	if err != nil {
		logger.Warnw("error in proxyAppConn.BeginBlock", "err", err)
		return nil, err
	}

	// 代理 app 将区块中的每个 tx 都发送到 Application 那里去验证，例如：对于 kvstore 来说，
	// kvstore 应用会直接将 tx 中的 key-value 存储到数据库里
	for _, tx := range block.Txs {
		proxyAppConn.DeliverTxAsync(protoabci.RequestDeliverTx{Tx: tx})
		if err := proxyAppConn.Error(); err != nil { // 返回代理 app 包装的 client 的 err
			return nil, err
		}
	}

	// 通知ABCI应用层此次区块已经提交完毕了，注意这个步骤会从 ABCI app 处获取到更新后的 validator 集合，
	// 然后更新后的 validator 集合会参加一个高度的共识过程
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(protoabci.RequestEndBlock{Height: block.Height})
	if err != nil {
		logger.Warnw("error in proxyAppConn.EndBlock", "err", err)
		return nil, err
	}

	logger.Infow("executed block", "height", block.Height, "num_valid_txs", validTxs, "num_invalid_txs", invalidTxs)
	return abciResponses, nil
}

// getBeginBlockValidatorInfo 获取上一个区块的 commit 信息，主要经历以下几步：
//	1. 检查当前区块高度是否大于1，如果大于1，则可正常从store里获取到上个区块高度对
//     应的 validator 集合，之所以要检查当前区块高度是否大于1，是想确保上一个区块
//	   不是创世区块，因为创世区块里不含有 validator 集合
//	2. 获取到上一个高度的 validator 集合以后，比较 validator 的数量是否能和 commitSig
//	   的数量对的上，对不上则说明出错了，直接 panic
//	3. 继续从当前区块的上一个区块的 commit 信息里获取到相应 validator 是否给上一个
//	   区块投票的信息，然后构造 protoabci.LastCommitInfo{} 并返回
func getBeginBlockValidatorInfo(block *types.Block, store Store, initialHeight int64) protoabci.LastCommitInfo {
	// 初始化 commit 上一个区块的投票列表，列表长度等于 commit 上一个区块的 ReplySig 数量
	voteInfos := make([]protoabci.VoteInfo, block.LastReply.Size())

	// 如果当前区块的高度大于 1
	if block.Height > initialHeight {
		// 那么上一个高度的 validator 集合可以从 store 中获取到
		lastValSet, err := store.LoadValidators(block.Height - 1)
		if err != nil {
			panic(err)
		}

		for i, sig := range block.LastReply.Signatures {
			for _, val := range lastValSet.Validators {
				if bytes.Equal(sig.ValidatorAddress, val.Address) {
					voteInfos[i] = protoabci.VoteInfo{
						Validator:  types.SR2PB.Validator(val),
						SignedLastBlock: !sig.Absent(),
					}
				}
			}
		}
	}

	// 返回上一个高度的 commit 信息
	return protoabci.LastCommitInfo{
		Round: block.LastReply.Round,
		Votes: voteInfos,
	}
}

// validateValidatorUpdates 验证更新后的 validator 集合，验证以下几点内容：
//	1. 挨个检查 abciUpdates 里的每个 validator 的投票权是否小于0，有就报错
//	2. 对于投票权等于 0 的 validator，直接跳过
//	3. 挨个检查 abciUpdates 里的每个 validator 的公钥类型是否和 params 里
//	   规定的公钥类型一样，不一样就报错
func validateValidatorUpdates(abciUpdates []protoabci.ValidatorUpdate, params prototypes.ValidatorParams) error {
	for _, valUpdate := range abciUpdates {
		if valUpdate.GetPower() < 0 {
			return fmt.Errorf("voting power can't be negative %v", valUpdate)
		} else if valUpdate.GetPower() == 0 {
			continue
		}

		// 挨个检查 abciUpdates 里的每个 validator 的公钥类型是否和 params 里
		// 规定的公钥类型一样，不一样就报错
		pk, err := cryptoenc.PubKeyFromProto(valUpdate.PubKey)
		if err != nil {
			return err
		}

		if !types.IsValidPubkeyType(params, pk.Type()) {
			return fmt.Errorf("validator %v is using pubkey %s, which is unsupported for consensus",
				valUpdate, pk.Type())
		}
	}
	return nil
}

// updateState 根据本次高度 commit 的区块和代理 app 的响应更新 State，并将更新后的 State 返回回来：
//	1. 首先是获取针对下一个高度的 validator 集合，如果代理 app 更新了 validator 的信息，相应的也要更改
// 	   下一个高度的 validator 集合信息；
//	2. 更新下一个高度的 proposer，并按照相应的规则更新 proposer 的优先级，ValidatorSet.Primary = newProposer；
//	3. 将当前刚刚 commit 的区块作为 last 区块存储到新的 State 里，并且 State 里的下一个 validator 集合
//	   也相应做出改变，最后将新的 State 返回出来。
func updateState(state State, blockID types.BlockID, header *types.Header, abciResponses *protostate.ABCIResponses, validatorUpdates []*types.Validator) (State, error) {
	// 获取下一个高度的 validator 集合
	nValSet := state.NextValidators.Copy()

	// 根据abciResponses返回的验证者来更新当前的验证者集合
	// 更新原则是这样：
	// 如果当前不存在则直接加入一个验证者
	// 如果当前存在并且投票权为0则删除
	// 如果当前存在其投票权不为0则更新
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(validatorUpdates) > 0 { // validatorUpdates 表示状态改变的 validator 集合
		//for _, v := range validatorUpdates {
		//	fmt.Println("------------------>>>>>>>>>>>", v.Address, v.VotingPower, v.Type)
		//}
		// 此处的 validatorUpdates 经过了完整性检查，是原生格式的，它是由客户端应用程序返回的
		err := nValSet.UpdateWithChangeSet(validatorUpdates)

		if err != nil {
			return state, fmt.Errorf("error changing validator set: %v", err)
		}
		// Change results from this height but only applies to the next next height.
		lastHeightValsChanged = header.Height + 1 + 1
	}

	// 更新 validator 集合里各 validator 的优先级，并重新挑选出优先级最大的 validator 作为 proposer
	nValSet.CopyIteratePrimary(state.LastBlockID, 1)

	// 根据返回结果更新一下共识参数
	nextParams := state.ConsensusParams

	// 返回此次区块被验证成功之后的 State，此State也就是为了验证下一个区块
	// 注意 APPHASH 还没有更新，因为还有一步没有做
	return State{
		ChainID:                          state.ChainID,
		InitialHeight:                    state.InitialHeight,
		LastBlockHeight:                  header.Height,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		NextValidators:                   nValSet,
		Validators:                       state.NextValidators.Copy(),
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if system crashes before commit, some or all of these events may be published again.
func fireEvents(
	logger srlog.CRLogger,
	eventBus types.BlockEventPublisher,
	block *types.Block,
	abciResponses *protostate.ABCIResponses,
	validatorUpdates []*types.Validator,
) {
	if err := eventBus.PublishEventNewBlock(types.EventDataNewBlock{
		Block:            block,
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	}); err != nil {
		logger.Warnw("failed publishing new block", "err", err)
	}

	if err := eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{
		Header:           block.Header,
		NumTxs:           int64(len(block.Txs)),
		ResultBeginBlock: *abciResponses.BeginBlock,
		ResultEndBlock:   *abciResponses.EndBlock,
	}); err != nil {
		logger.Warnw("failed publishing new block header", "err", err)
	}

	for i, tx := range block.Data.Txs {
		if err := eventBus.PublishEventTx(types.EventDataTx{TxResult: protoabci.TxResult{
			Height: block.Height,
			Index:  uint32(i),
			Tx:     tx,
			Result: *(abciResponses.DeliverTxs[i]),
		}}); err != nil {
			logger.Warnw("failed publishing event TX", "err", err)
		}
	}

	if len(validatorUpdates) > 0 {
		if err := eventBus.PublishEventValidatorSetUpdates(
			types.EventDataValidatorSetUpdates{ValidatorUpdates: validatorUpdates}); err != nil {
			logger.Warnw("failed publishing event", "err", err)
		}
	}
}

func ExecBlock(
	appConnConsensus *proxy.AppConnConsensus,
	block *types.Block,
	logger srlog.CRLogger,
	store Store,
	initialHeight int64,
) ([]byte, error) {
	_, err := execBlockOnProxyApp(logger, appConnConsensus, block, store, initialHeight)
	if err != nil {
		logger.Warnw("failed executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}

	// Reply block, get hash back
	res, err := appConnConsensus.CommitSync()
	if err != nil {
		logger.Warnw("client error during proxyAppConn.CommitSync", "err", res)
		return nil, err
	}

	// ResponseCommit has no error or log, just data
	return res.Data, nil
}
