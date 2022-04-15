package types

import (
	"BFT/crypto"
	"BFT/libs/bits"
	srbytes "BFT/libs/bytes"
	srjson "BFT/libs/json"
	"BFT/libs/protoio"
	prototypes "BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	nilVoteStr string = "nil-Vote"
)

var (
	ErrVoteUnexpectedStep            = errors.New("unexpected step")
	ErrVoteInvalidValidatorIndex     = errors.New("invalid validator index")
	ErrVoteInvalidValidatorAddress   = errors.New("invalid validator address")
	ErrVoteInvalidSignature          = errors.New("invalid signature")
	ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")
	ErrVoteNil                       = errors.New("nil vote")
)

type ErrVoteConflictingVotes struct {
	VoteA *Vote
	VoteB *Vote
}

func (err *ErrVoteConflictingVotes) Error() string {
	return fmt.Sprintf("conflicting votes from validator %X", err.VoteA.ValidatorAddress)
}

func NewConflictingVoteError(vote1, vote2 *Vote) *ErrVoteConflictingVotes {
	return &ErrVoteConflictingVotes{
		VoteA: vote1,
		VoteB: vote2,
	}
}

// -----------------------------------------------------------------------------
// vote ------------------------------------------------------------------------

// Vote represents a prepare, commit, or reply vote from validators for
// consensus.
type Vote struct {
	Type   prototypes.SignedMsgType `json:"type"`
	Height int64                    `json:"height"`
	Round            int32                    `json:"round"`    // assume there will not be greater than 2_147_483_647 rounds
	BlockID          BlockID                  `json:"block_id"` // zero if vote is nil.
	Timestamp        time.Time                `json:"timestamp"`
	ValidatorAddress Address                  `json:"validator_address"`
	ValidatorIndex   int32                    `json:"validator_index"`
	Signature        []byte                   `json:"signature"`
}

// ReplySig 将 Vote 转换为 ReplySig
func (vote *Vote) ReplySig() ReplySig {
	if vote == nil {
		return NewCommitSigAbsent()
	}

	var blockIDFlag BlockIDFlag
	switch {
	case vote.BlockID.IsComplete():
		blockIDFlag = BlockIDFlagCommit
	case vote.BlockID.IsZero():
		blockIDFlag = BlockIDFlagNil
	default:
		panic(fmt.Sprintf("Invalid vote %v - expected BlockID to be either empty or complete", vote))
	}

	return ReplySig{
		BlockIDFlag:      blockIDFlag,
		ValidatorAddress: vote.ValidatorAddress,
		Timestamp:        vote.Timestamp,
		Signature:        vote.Signature,
	}
}

// VoteSignBytes 先将 chainID 和 prototypes.Vote 组合成 CanonicalVote，
// 后者不包括 prototypes.Vote 里的 ValidatorAddress、ValidatorIndex 和
// Signature 等字段，但多了一个 ChainID 字段，得到 CanonicalVote 之后，用
// protoio 包里的 MarshalDelimited 方法，将其编码成 protobuf 字节码：bz，
// bz 的内部各式如下：<CanonicalVote的长度+CanonicalVote的字节码>
//	CanonicalVote 里包含如下6个字段：Type、Height、Round、BlockID、Timestamp 和 ChainID
func VoteSignBytes(chainID string, vote *prototypes.Vote) []byte {
	pb := CanonicalizeVote(chainID, vote)
	bz, err := protoio.MarshalDelimited(&pb)
	if err != nil {
		panic(err)
	}

	return bz
}

func (vote *Vote) Copy() *Vote {
	voteCopy := *vote
	return &voteCopy
}

// String returns a string representation of Vote.
//
// 1. validator index
// 2. first 6 bytes of validator address
// 3. height
// 4. round,
// 5. type byte
// 6. type string
// 7. first 6 bytes of block hash
// 8. first 6 bytes of signature
// 9. timestamp
func (vote *Vote) String() string {
	if vote == nil {
		return nilVoteStr
	}

	var typeString string
	switch vote.Type {
	case prototypes.PrepareType:
		typeString = "Prepare"
	case prototypes.CommitType:
		typeString = "Reply"
	default:
		panic("Unknown vote type")
	}

	return fmt.Sprintf("Vote{%v:%X %v/%02d/%v(%v) %X %X @ %s}",
		vote.ValidatorIndex,
		srbytes.Fingerprint(vote.ValidatorAddress),
		vote.Height,
		vote.Round,
		vote.Type,
		typeString,
		srbytes.Fingerprint(vote.BlockID.Hash),
		srbytes.Fingerprint(vote.Signature),
		CanonicalTime(vote.Timestamp),
	)
}

func (vote *Vote) Verify(chainID string, pubKey crypto.PubKey) error {
	if !bytes.Equal(pubKey.Address(), vote.ValidatorAddress) {
		return ErrVoteInvalidValidatorAddress
	}
	v := vote.ToProto()
	if !pubKey.VerifySignature(VoteSignBytes(chainID, v), vote.Signature) {
		return ErrVoteInvalidSignature
	}
	return nil
}

// ValidateBasic performs basic validation.
func (vote *Vote) ValidateBasic() error {
	if !IsVoteTypeValid(vote.Type) {
		return errors.New("invalid Type")
	}

	if vote.Height < 0 {
		return errors.New("negative Height")
	}

	if vote.Round < 0 {
		return errors.New("negative Round")
	}

	// NOTE: Timestamp validation is subtle and handled elsewhere.

	if err := vote.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}

	// BlockID.ValidateBasic would not err if we for instance have an empty hash but a
	// non-empty PartsSetHeader:
	if !vote.BlockID.IsZero() && !vote.BlockID.IsComplete() {
		return fmt.Errorf("blockID must be either empty or complete, got: %v", vote.BlockID)
	}

	if len(vote.ValidatorAddress) != crypto.AddressSize {
		return fmt.Errorf("expected ValidatorAddress size to be %d bytes, got %d bytes",
			crypto.AddressSize,
			len(vote.ValidatorAddress),
		)
	}
	if vote.ValidatorIndex < 0 {
		return errors.New("negative ValidatorIndex")
	}
	if len(vote.Signature) == 0 {
		return errors.New("signature is missing")
	}

	if len(vote.Signature) > 64 {
		return fmt.Errorf("signature is too big (max: %d)", 64)
	}

	return nil
}

// ToProto converts the handwritten type to proto generated type
// return type, nil if everything converts safely, otherwise nil, error
func (vote *Vote) ToProto() *prototypes.Vote {
	if vote == nil {
		return nil
	}

	return &prototypes.Vote{
		Type:             vote.Type,
		Height:           vote.Height,
		Round:            vote.Round,
		BlockID:          vote.BlockID.ToProto(),
		Timestamp:        vote.Timestamp,
		ValidatorAddress: vote.ValidatorAddress,
		ValidatorIndex:   vote.ValidatorIndex,
		Signature:        vote.Signature,
	}
}

// VoteFromProto converts a proto generetad type to a handwritten type
// return type, nil if everything converts safely, otherwise nil, error
func VoteFromProto(pv *prototypes.Vote) (*Vote, error) {
	if pv == nil {
		return nil, errors.New("nil vote")
	}

	blockID, err := BlockIDFromProto(&pv.BlockID)
	if err != nil {
		return nil, err
	}

	vote := new(Vote)
	vote.Type = pv.Type
	vote.Height = pv.Height
	vote.Round = pv.Round
	vote.BlockID = *blockID
	vote.Timestamp = pv.Timestamp
	vote.ValidatorAddress = pv.ValidatorAddress
	vote.ValidatorIndex = pv.ValidatorIndex
	vote.Signature = pv.Signature

	return vote, vote.ValidateBasic()
}

// ---------------------------------------------------------------------------------
// VoteSet -------------------------------------------------------------------------

const (
	// MaxVotesCount 是一个集合中最大的投票数。
	// 在 ValidateBasic 函数中用于防止DOS攻击。
	// 注意，这意味着 Validator 的数量有相应的相等限制。
	MaxVotesCount = 10000
)

// P2PID
// 不稳定
// XXX: gossip.ID 副本，以避免包之间的依赖。
// 也许我们可以有一个最小类型包包含这个(和其他东西?)，同时' types '和' p2p '导入?
type P2PID string

/*
VoteSet 帮助在预定义的投票类型的每个 height+round 中，从验证器中收集签名。
当验证器重复签名时，我们需要VoteSet能够跟踪冲突的投票。然而，我们不能跟踪看到的*所有*投票，因为这可能是一个 DoS 攻击向量。
有两个存放选票的地方。
	1. VoteSet.votes
	2. VoteSet.votesByBlock
	- ”.votes” 是规范的 vote 列表。它总是至少有一个 vote，如果一个 Validator 的 vote 被看到了。
	通常它会跟踪第一次看到的投票，但当发现2/3多数时，该投票获得优先级，并从 “.votesByBlock“ 复制。
	- ”.votesByBlock“ 跟踪一个特定区块的投票列表。在 “.votesByBlock“ 中创建 &blockVotes{} 有两种方式：
			- 1. 验证器看到的第一个投票是针对特定块的。
			- 2. 一个 peer 称，有三分之二的人支持这个区块。
因为来自验证器的第一个投票总是会被添加到 “.votesByBlock“ 中，”.votes” 里的所有的 vote
将在 “.votesByBlock“ 中有相应的条目。
当 &blockVotes{} 在 “.votesByBlock“ 达到 +2/3 法定人数，其投票被复制到 ”.votes”。
所有这些都是内存有限的，因为只有当一个 peer 告诉我们去跟踪那个区块时，冲突的投票才会被添加进来，
每个 peer 只能告诉我们一个这样的区块，而且，peer 的数量是有限的。
注:假设投票权总数不超过MaxUInt64。
*/
type VoteSet struct {
	chainID       string
	height        int64
	round         int32
	signedMsgType prototypes.SignedMsgType
	valSet        *ValidatorSet

	mtx           sync.Mutex
	votesBitArray *bits.BitArray
	votes         []*Vote  // Primary votes to share
	sum           float64    // 已见 vote 的投票权之和，扣除冲突
	maj23         *BlockID // First 2/3 majority seen
	// string(blockHash|blockParts) -> blockVotes。
	// 根据 BlockID.Key() 来存储投票信息，因为可能有恶意节点故意使坏，发送和别人不一样的投票信息，
	// 干扰共识过程，因此，我们需要按照区块内容分别存储 Validator 的投票信息
	votesByBlock map[string]*blockVotes
	peerMaj23s   map[P2PID]BlockID // Maj23 for each peer
}

// NewVoteSet
// Constructs a new VoteSet struct used to accumulate votes for given height/round.
func NewVoteSet(chainID string, height int64, round int32,
	signedMsgType prototypes.SignedMsgType, valSet *ValidatorSet) *VoteSet {
	if height == 0 {
		panic("Cannot make VoteSet for height == 0, doesn't make sense.")
	}
	return &VoteSet{
		chainID:       chainID,
		height:        height,
		round:         round,
		signedMsgType: signedMsgType,
		valSet:        valSet,
		votesBitArray: bits.NewBitArray(valSet.Size()), // 对应位置的 Vote 合法，才会为 1
		votes:         make([]*Vote, valSet.Size()),
		sum:           0,
		maj23:         nil,
		votesByBlock:  make(map[string]*blockVotes, valSet.Size()),
		peerMaj23s:    make(map[P2PID]BlockID),
	}
}

func (voteSet *VoteSet) ChainID() string {
	return voteSet.chainID
}

// Implements VoteSetReader.
func (voteSet *VoteSet) GetHeight() int64 {
	if voteSet == nil {
		return 0
	}
	return voteSet.height
}

// Implements VoteSetReader.
func (voteSet *VoteSet) GetRound() int32 {
	if voteSet == nil {
		return -1
	}
	return voteSet.round
}

// Implements VoteSetReader.
func (voteSet *VoteSet) Type() byte {
	if voteSet == nil {
		return 0x00
	}
	return byte(voteSet.signedMsgType)
}

// Size 返回 Validator 的数量，Implements VoteSetReader.
func (voteSet *VoteSet) Size() int {
	if voteSet == nil {
		return 0
	}
	return voteSet.valSet.Size()
}

// AddVote
// Returns added=true if vote is valid and new.
// Otherwise returns err=ErrVote[UnexpectedStep | InvalidIndex | InvalidAddress | InvalidSignature | InvalidBlockHash | ConflictingVotes ]
// Duplicate votes return added=false, err=nil.
// Conflicting votes return added=*, err=ErrVoteConflictingVotes.
// NOTE: vote should not be mutated after adding.
// NOTE: VoteSet must not be nil
// NOTE: Vote must not be nil
func (voteSet *VoteSet) AddVote(vote *Vote) (added bool, err error) {
	if voteSet == nil {
		panic("AddVote() on nil VoteSet")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	return voteSet.addVote(vote)
}

// NOTE: Validates as much as possible before attempting to verify the signature.
func (voteSet *VoteSet) addVote(vote *Vote) (added bool, err error) {
	if vote == nil {
		return false, ErrVoteNil
	}
	valIndex := vote.ValidatorIndex  // 获取 Validator 的索引
	valAddr := vote.ValidatorAddress // 获取 Validator 的地址
	blockKey := vote.BlockID.Key()   //

	// 对投票者的信息进行最基本的检查
	if valIndex < 0 {
		return false, fmt.Errorf("index < 0: %w", ErrVoteInvalidValidatorIndex)
	} else if len(valAddr) == 0 {
		return false, fmt.Errorf("empty address: %w", ErrVoteInvalidValidatorAddress)
	}

	// Make sure the step matches.
	if (vote.Height != voteSet.height) ||
		(vote.Round != voteSet.round) ||
		(vote.Type != voteSet.signedMsgType) {
		return false, fmt.Errorf("expected %d/%d/%d, but got %d/%d/%d: %w",
			voteSet.height, voteSet.round, voteSet.signedMsgType,
			vote.Height, vote.Round, vote.Type, ErrVoteUnexpectedStep)
	}

	// 确保签名者是一个 Validator
	_, val := voteSet.valSet.GetByIndex(valIndex)
	if val == nil {
		return false, fmt.Errorf(
			"cannot find validator %d in valSet of size %d: %w",
			valIndex, voteSet.valSet.Size(), ErrVoteInvalidValidatorIndex)
	}

	// 我们不应该添加两个相同的投票信息
	if existing, ok := voteSet.getVote(valIndex, blockKey); ok {
		if bytes.Equal(existing.Signature, vote.Signature) {
			return false, nil // duplicate
		}
		// 一个投票者投出了两个不一样的投票，模棱两可，属于恶意节点了属于是
		return false, fmt.Errorf("existing vote: %v; new vote: %v: %w", existing, vote, ErrVoteNonDeterministicSignature)
	}

	// 验证投票的签名是否正确
	if err := vote.Verify(voteSet.chainID, val.PubKey); err != nil {
		return false, fmt.Errorf("failed to verify vote with ChainID %s and PubKey %s: %w", voteSet.chainID, val.PubKey, err)
	}

	// 确保在同一高度的同一轮次中，一个验证器只会给一个投票签名
	added, conflicting := voteSet.addVerifiedVote(vote, blockKey, val.VotingPower)
	if conflicting != nil {
		return added, NewConflictingVoteError(conflicting, vote)
	}
	if !added {
		panic("Expected to add non-conflicting vote")
	}
	return added, nil
}

// getVote 根据 Validator 的索引和 BlockID.Key 获取指定的 Vote
func (voteSet *VoteSet) getVote(valIndex int32, blockKey string) (vote *Vote, ok bool) {
	if existing := voteSet.votes[valIndex]; existing != nil && existing.BlockID.Key() == blockKey {
		return existing, true
	}
	if existing := voteSet.votesByBlock[blockKey].getByIndex(valIndex); existing != nil {
		return existing, true
	}
	return nil, false
}

// 假设签名是合法的
// If conflicting vote exists, returns it.
func (voteSet *VoteSet) addVerifiedVote(vote *Vote, blockKey string, votingPower float64) (added bool, conflict *Vote) {
	// 获取 Validator 的索引
	valIndex := vote.ValidatorIndex

	// VoteSet 的 votes 字段是一个 *Vote 切片，索引值等于 validator 的索引
	if existing := voteSet.votes[valIndex]; existing != nil { // 对应的 Validator 已经产生 vote 了
		if existing.BlockID.Equals(vote.BlockID) {
			// 如果 validator 已经为同样的 block 投过票了，则直接 panic
			panic("addVerifiedVote does not expect duplicate votes")
		} else {
			// 如果 validator 给不同的 block 投了票，则表示产生了冲突
			conflict = existing
		}
		// 如果 blockKey 匹配 voteSet.maj23，则替换 validator 之前的 vote。
		if voteSet.maj23 != nil && voteSet.maj23.Key() == blockKey {
			voteSet.votes[valIndex] = vote
			// 在 vote 集合的对应位置为 validator 设置 true，因为该 validator 为我认可的 block 投了票
			voteSet.votesBitArray.SetIndex(int(valIndex), true)
		}
	} else {
		// 如果该 validator 之前没有发送过 vote 过来，则将其添加到 votes 集合中，
		// 并且将 validator 的投票权累加到 sum 上，表示已收到的 vote 的投票权总和
		voteSet.votes[valIndex] = vote
		voteSet.votesBitArray.SetIndex(int(valIndex), true)
		voteSet.sum += votingPower
	}

	// 如果
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		// 如果产生了冲突并且没有 peer 声明获得超过 2/3 的投票
		if conflict != nil && !votesByBlock.peerMaj23 {
			return false, conflict
		}
		// We'll add the vote in a bit.
	} else {
		// .votesByBlock doesn't exist...
		if conflict != nil {
			// ... and there's a conflict vote.
			// We're not even tracking this blockKey, so just forget it.
			return false, conflict
		}
		// ... and there's no conflict vote.
		// Start tracking this blockKey
		votesByBlock = newBlockVotes(false, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// We'll add the vote in a bit.
	}

	// Before adding to votesByBlock, see if we'll exceed quorum
	origSum := votesByBlock.sum
	quorum := TotalVotingPower * float64(2)/float64(3) + 1

	// Add vote to votesByBlock
	votesByBlock.addVerifiedVote(vote, votingPower)

	// origSum 是 blockVote 在添加新的投票前的已获得的投票权，然后后面的 votesByBlock.sum
	// 表示 blockVote 在添加新的投票后的已获得的投票权
	if origSum < quorum && quorum <= votesByBlock.sum {
		// Only consider the first quorum reached
		if voteSet.maj23 == nil {
			maj23BlockID := vote.BlockID
			voteSet.maj23 = &maj23BlockID
			// And also copy votes over to voteSet.votes
			for i, vote := range votesByBlock.votes {
				if vote != nil {
					voteSet.votes[i] = vote
				}
			}
		}
	}

	return true, conflict
}

// SetPeerMaj23 如果一个 peer 声称它有 +2/3 的给定 blockKey，调用这个。
// 注意:如果有太多的对等点，或太多的对等点流失，这可能会导致内存问题。
func (voteSet *VoteSet) SetPeerMaj23(peerID P2PID, blockID BlockID) error {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	blockKey := blockID.Key()

	// 确保该 peer 不会发送针对不同 Block 的 Maj23
	if existing, ok := voteSet.peerMaj23s[peerID]; ok {
		if existing.Equals(blockID) {
			return nil
		}
		return fmt.Errorf("setPeerMaj23: Received conflicting blockID from peer %v. Got %v, expected %v", peerID, blockID, existing)
	}
	voteSet.peerMaj23s[peerID] = blockID

	// Create .votesByBlock entry if needed.
	votesByBlock, ok := voteSet.votesByBlock[blockKey]
	if ok {
		if votesByBlock.peerMaj23 {
			return nil // Nothing to do
		}
		votesByBlock.peerMaj23 = true
		// No need to copy votes, already there.
	} else {
		votesByBlock = newBlockVotes(true, voteSet.valSet.Size())
		voteSet.votesByBlock[blockKey] = votesByBlock
		// No need to copy votes, no votes to copy over.
	}
	return nil
}

// Implements VoteSetReader.
func (voteSet *VoteSet) BitArray() *bits.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votesBitArray.Copy()
}

func (voteSet *VoteSet) BitArrayByBlockID(blockID BlockID) *bits.BitArray {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	votesByBlock, ok := voteSet.votesByBlock[blockID.Key()]
	if ok {
		return votesByBlock.bitArray.Copy()
	}
	return nil
}

// NOTE: if validator has conflicting votes, returns "canonical" vote
// Implements VoteSetReader.
func (voteSet *VoteSet) GetByIndex(valIndex int32) *Vote {
	if voteSet == nil {
		return nil
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.votes[valIndex]
}

// HasTwoThirdsMajority 判断是否得到超过 2/3 的投票
func (voteSet *VoteSet) HasTwoThirdsMajority() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

// Implements VoteSetReader.
func (voteSet *VoteSet) IsReply() bool {
	if voteSet == nil {
		return false
	}
	if voteSet.signedMsgType != prototypes.CommitType {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.maj23 != nil
}

func (voteSet *VoteSet) HasTwoThirdsAny() bool {
	if voteSet == nil {
		return false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum > TotalVotingPower*2.0/3.0
}

// 判断我们是否收集齐投票
func (voteSet *VoteSet) HasAll() bool {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.sum == TotalVotingPower
}

// TwoThirdsMajority 从投票集合里查找有没有已经获得过超过 2/3 投票的 BlockID，
// 如果有的话，则返回 BlockID，否则返回 BlockID{}
func (voteSet *VoteSet) TwoThirdsMajority() (blockID BlockID, ok bool) {
	if voteSet == nil {
		return BlockID{}, false
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	if voteSet.maj23 != nil {
		return *voteSet.maj23, true
	}
	return BlockID{}, false
}

//--------------------------------------------------------------------------------
// Strings and JSON

const nilVoteSetString = "nil-VoteSet"

// String returns a string representation of VoteSet.
//
// See StringIndented.
func (voteSet *VoteSet) String() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	return voteSet.StringIndented("")
}

// StringIndented returns an indented String.
//
// Height Round Type
// Votes
// Votes bit array
// 2/3+ majority
//
// See Vote#String.
func (voteSet *VoteSet) StringIndented(indent string) string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = nilVoteStr
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return fmt.Sprintf(`VoteSet{
%s  H:%v R:%v T:%v
%s  %v
%s  %v
%s  %v
%s}`,
		indent, voteSet.height, voteSet.round, voteSet.signedMsgType,
		indent, strings.Join(voteStrings, "\n"+indent+"  "),
		indent, voteSet.votesBitArray,
		indent, voteSet.peerMaj23s,
		indent)
}

// Marshal the VoteSet to JSON. Same as String(), just in JSON,
// and without the height/round/signedMsgType (since its already included in the votes).
func (voteSet *VoteSet) MarshalJSON() ([]byte, error) {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return srjson.Marshal(VoteSetJSON{
		voteSet.voteStrings(),
		voteSet.bitArrayString(),
		voteSet.peerMaj23s,
	})
}

// More human readable JSON of the vote set
// NOTE: insufficient for unmarshalling from (compressed votes)
type VoteSetJSON struct {
	Votes         []string          `json:"votes"`
	VotesBitArray string            `json:"votes_bit_array"`
	PeerMaj23s    map[P2PID]BlockID `json:"peer_maj_23s"`
}

// Return the bit-array of votes including
// the fraction of power that has voted like:
// "BA{29:xx__x__x_x___x__x_______xxx__} 856/1304 = 0.66"
func (voteSet *VoteSet) BitArrayString() string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.bitArrayString()
}

func (voteSet *VoteSet) bitArrayString() string {
	bAString := voteSet.votesBitArray.String()
	voted, total, fracVoted := voteSet.sumTotalFrac()
	return fmt.Sprintf("%s %d/%d = %.2f", bAString, voted, total, fracVoted)
}

// Returns a list of votes compressed to more readable strings.
func (voteSet *VoteSet) VoteStrings() []string {
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	return voteSet.voteStrings()
}

func (voteSet *VoteSet) voteStrings() []string {
	voteStrings := make([]string, len(voteSet.votes))
	for i, vote := range voteSet.votes {
		if vote == nil {
			voteStrings[i] = nilVoteStr
		} else {
			voteStrings[i] = vote.String()
		}
	}
	return voteStrings
}

// StringShort returns a short representation of VoteSet.
//
// 1. height
// 2. round
// 3. signed msg type
// 4. first 2/3+ majority
// 5. fraction of voted power
// 6. votes bit array
// 7. 2/3+ majority for each peer
func (voteSet *VoteSet) StringShort() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	_, _, frac := voteSet.sumTotalFrac()
	return fmt.Sprintf(`VoteSet{H:%v R:%v T:%v +2/3:%v(%v) %v %v}`,
		voteSet.height, voteSet.round, voteSet.signedMsgType, voteSet.maj23, frac, voteSet.votesBitArray, voteSet.peerMaj23s)
}

// LogString produces a logging suitable string representation of the
// vote set.
func (voteSet *VoteSet) LogString() string {
	if voteSet == nil {
		return nilVoteSetString
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()
	voted, total, frac := voteSet.sumTotalFrac()

	return fmt.Sprintf("Votes:%f/%f(%.3f)", voted, total, frac)
}

// return the power voted, the total, and the fraction
func (voteSet *VoteSet) sumTotalFrac() (float64, float64, float64) {
	voted, total := voteSet.sum, TotalVotingPower
	fracVoted := float64(voted) / float64(total)
	return voted, total, fracVoted
}

//--------------------------------------------------------------------------------
// Reply

// MakeReply 构建 Reply，当收到超过 2/3 投票的 commit 后，构建 Reply
func (voteSet *VoteSet) MakeReply() *Reply {
	if voteSet.signedMsgType != prototypes.CommitType {
		panic("Cannot MakeReply() unless VoteSet.Type is CommitType")
	}
	voteSet.mtx.Lock()
	defer voteSet.mtx.Unlock()

	if voteSet.maj23 == nil {
		panic("Cannot MakeReply() unless a blockhash has +2/3")
	}

	commitSigs := make([]ReplySig, len(voteSet.votes))
	for i, v := range voteSet.votes {
		commitSig := v.ReplySig()
		// 如果是为了 block 而 commit 的，则检查投票的 BlockID 是否和 2/3 的投票者认为是一样的
		// 如果不一样，则认为该 validator 的投票不正确，因此将该 validator 的投票转换为针对 absent 的投票
		if commitSig.ForBlock() && !v.BlockID.Equals(*voteSet.maj23) {
			commitSig = NewCommitSigAbsent()
		}
		commitSigs[i] = commitSig
	}

	return NewReply(voteSet.GetHeight(), voteSet.GetRound(), *voteSet.maj23, commitSigs)
}

//--------------------------------------------------------------------------------

/*
	Votes for a particular block
	There are two ways a *blockVotes gets created for a blockKey.
	1. first (non-conflicting) vote of a validator w/ blockKey (peerMaj23=false)
	2. A peer claims to have a 2/3 majority w/ blockKey (peerMaj23=true)
*/
type blockVotes struct {
	peerMaj23 bool           // peer claims to have maj23
	bitArray  *bits.BitArray // valIndex -> hasVote?
	votes     []*Vote        // valIndex -> *Vote
	sum       float64          // vote sum
}

func newBlockVotes(peerMaj23 bool, numValidators int) *blockVotes {
	return &blockVotes{
		peerMaj23: peerMaj23,
		bitArray:  bits.NewBitArray(numValidators),
		votes:     make([]*Vote, numValidators),
		sum:       0,
	}
}

func (vs *blockVotes) addVerifiedVote(vote *Vote, votingPower float64) {
	valIndex := vote.ValidatorIndex
	if existing := vs.votes[valIndex]; existing == nil {
		vs.bitArray.SetIndex(int(valIndex), true)
		vs.votes[valIndex] = vote
		vs.sum += votingPower
	}
}

func (vs *blockVotes) getByIndex(index int32) *Vote {
	if vs == nil {
		return nil
	}
	return vs.votes[index]
}

//--------------------------------------------------------------------------------

type VoteSetReader interface {
	GetHeight() int64
	GetRound() int32
	Type() byte
	Size() int
	BitArray() *bits.BitArray
	GetByIndex(int32) *Vote
	IsReply() bool
}

func IsVoteTypeValid(t prototypes.SignedMsgType) bool {
	switch t {
	case prototypes.PrepareType, prototypes.CommitType:
		return true
	default:
		return false
	}
}
