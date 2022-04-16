package types

import (
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/crypto/ed25519"
	srbytes "github.com/232425wxy/BFT/libs/bytes"
	srjson "github.com/232425wxy/BFT/libs/json"
	sros "github.com/232425wxy/BFT/libs/os"
	"github.com/232425wxy/BFT/libs/protoio"
	"github.com/232425wxy/BFT/libs/tempfile"
	srtime "github.com/232425wxy/BFT/libs/time"
	"github.com/232425wxy/BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"io/ioutil"
	"time"
)

// ---------------------------------------------------------------------------------------------
// 用于对 vote 进行签名的 Validator---------------------------------------------------------------

// PrivValidator 接口定义了一个本地 Validator 的功能，
// 该 Validator 可以对 vote 和 proposal 进行签名，而且不会重复签名。
// 实现该接口的结构体定义在 prival/ 里面
type PrivValidator interface {
	GetPubKey() (crypto.PubKey, error)

	SignVote(chainID string, vote *types.Vote) error
	SignProposal(chainID string, prePrepare *types.PrePrepare) error
}

type PrivValidatorsByAddress []PrivValidator

func (pvs PrivValidatorsByAddress) Len() int {
	return len(pvs)
}

// Less 报告索引为 i 的元素是否应该排在在索引为 j 的元素之前
func (pvs PrivValidatorsByAddress) Less(i, j int) bool {
	pvi, err := pvs[i].GetPubKey()
	if err != nil {
		panic(err)
	}
	pvj, err := pvs[j].GetPubKey()
	if err != nil {
		panic(err)
	}

	// 如果 pvi.Address 小于 pvj.Address，则返回 -1，
	// 也就是说如果 pvi.Address 小于 pvj.Address，Less 返回 true
	// 那么 pvi 应当排在 pvj 之前
	return bytes.Compare(pvi.Address(), pvj.Address()) == -1
}

func (pvs PrivValidatorsByAddress) Swap(i, j int) {
	pvs[i], pvs[j] = pvs[j], pvs[i]
}

const (
	stepNone       int8 = 0 // 用来区分初始状态
	PrePrepareStep int8 = 1 // 提出区块阶段
	PrepareStep    int8 = 2 // 预投票
	CommitStep     int8 = 3 // 预提交
)

func voteToStep(vote *types.Vote) int8 {
	switch vote.Type {
	case types.PrepareType:
		return PrepareStep
	case types.CommitType:
		return CommitStep
	default:
		panic(fmt.Sprintf("Unknown vote type: %v", vote.Type))
	}
}

//-------------------------------------------------------------------------------

// FilePVKey 私密 validator，存储的各个字段是不可变的
type FilePVKey struct {
	Address crypto.Address `json:"address"`
	PubKey  crypto.PubKey  `json:"pub_key"`
	PrivKey crypto.PrivKey `json:"priv_key"`

	filePath string
}

// Save 将 FilePVKey 持久化到硬盘上
//	写到磁盘上的内容是 FilePVKey 的 json 格式
func (pvKey FilePVKey) Save() {
	outFile := pvKey.filePath
	if outFile == "" {
		panic("cannot save PrivValidator key: filePath not set")
	}

	jsonBytes, err := srjson.MarshalIndent(pvKey, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}

}

//-------------------------------------------------------------------------------

// FilePVLastSignState 存储了 PrivValidator 的可变部分。
type FilePVLastSignState struct {
	Height    int64            `json:"height"`              // 区块高度
	Round     int32            `json:"round"`               // 投票轮次
	Step      int8             `json:"step"`                // 投票阶段
	Signature []byte           `json:"signature,omitempty"` // 签名信息
	SignBytes srbytes.HexBytes `json:"signbytes,omitempty"` //

	filePath string
}

// CheckHRS 根据给定的 height、round、step 检查 FilePVLastSignState 的正确性：
//	1. 如果给定的 height 小于 FilePVLastSignState.Height，则返回 false 和一个
//	   错误，如果等于，则进入第二步
//	2. 如果给定的 round 小于 FilePVLastSignState.Round，则返回 false 和一个错
//     误，若等于，则进入第三步
//	3. 如果给定的 step 小于 FilePVLastSignState.Step，则返回 false 和一个错误，
//	   ，如果等于则进入第四步
//	4. 如果 FilePVLastSignState.SignBytes 不等于 nil，而 FilePVLastSignState.Signature
//	   等于 nil，则直接 panic，但是如果 FilePVLastSignState.Signature 不等于 nil，
//	   则返回 true 和 nil-err
//	5. 如果 FilePVLastSignState.SignBytes 等于 nil，则返回 false 和一个错误
//	6. 如果 给定的 height 大于 FilePVLastSignState.Height，则返回 false 和 nil-err
func (lss *FilePVLastSignState) CheckHRS(height int64, round int32, step int8) (bool, error) {

	if lss.Height > height {
		return false, fmt.Errorf("height regression. Got %v, last height %v", height, lss.Height)
	}

	if lss.Height == height {
		if lss.Round > round {
			return false, fmt.Errorf("round regression at height %v. Got %v, last round %v", height, round, lss.Round)
		}

		if lss.Round == round {
			if lss.Step > step {
				return false, fmt.Errorf(
					"step regression at height %v round %v. Got %v, last step %v",
					height,
					round,
					step,
					lss.Step,
				)
			} else if lss.Step == step {
				if lss.SignBytes != nil {
					if lss.Signature == nil {
						panic("pv: Signature is nil but SignBytes is not!")
					}
					return true, nil
				}
				return false, errors.New("SignBytes is nil!")
			}
		}
	}
	return false, nil
}

// Save 将 FilePVLastSignState 中的内容持久化到硬盘上
func (lss *FilePVLastSignState) Save() {
	outFile := lss.filePath
	if outFile == "" {
		panic("cannot save FilePVLastSignState: filePath not set")
	}
	jsonBytes, err := srjson.MarshalIndent(lss, "", "  ")
	if err != nil {
		panic(err)
	}
	err = tempfile.WriteFileAtomic(outFile, jsonBytes, 0600)
	if err != nil {
		panic(err)
	}
}

//-------------------------------------------------------------------------------

// FilePV 实现 PrivValidator 接口
// to prevent double signing.
// 注意: 包含 pv.Key.filePath 和 pv.LastSignState.filePath 的目录必须已经存在。
// 它包括 LastSignature 和 LastSignBytes，所以如果进程在签名后但在共识前崩溃，我们也不会丢失签名。
type FilePV struct {
	Key           FilePVKey
	LastSignState FilePVLastSignState
}

// NewFilePV 根据给定的私钥和 paths 生成新的 validator
func NewFilePV(privKey crypto.PrivKey, keyFilePath, stateFilePath string) *FilePV {
	return &FilePV{
		Key: FilePVKey{
			Address:  privKey.PubKey().Address(),
			PubKey:   privKey.PubKey(),
			PrivKey:  privKey,
			filePath: keyFilePath,
		},
		LastSignState: FilePVLastSignState{
			Step:     stepNone, // 初始状态
			filePath: stateFilePath,
		},
	}
}

// GenFilePV 生成的 validator 具有随机的私钥
func GenFilePV(keyFilePath, stateFilePath string) *FilePV {
	return NewFilePV(ed25519.GenPrivKey(), keyFilePath, stateFilePath)
}

// LoadFilePV 从硬盘里加载 FilePV
func LoadFilePV(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, true)
}

// LoadFilePVEmptyState 从硬盘里加载 FilePV，但是 FilePV 里的 LastSignState 不会从硬盘里加载，
// 会设成空的
func LoadFilePVEmptyState(keyFilePath, stateFilePath string) *FilePV {
	return loadFilePV(keyFilePath, stateFilePath, false)
}

// If loadState is true, we load from the stateFilePath. Otherwise, we use an empty LastSignState.
func loadFilePV(keyFilePath, stateFilePath string, loadState bool) *FilePV {
	// 从硬盘里读取 FilePVKey 信息
	keyJSONBytes, err := ioutil.ReadFile(keyFilePath)
	if err != nil {
		sros.Exit(err.Error())
	}
	pvKey := FilePVKey{}
	err = srjson.Unmarshal(keyJSONBytes, &pvKey)
	if err != nil {
		sros.Exit(fmt.Sprintf("Error reading PrivValidator key from %v: %v\n", keyFilePath, err))
	}

	//
	pvKey.PubKey = pvKey.PrivKey.PubKey()
	pvKey.Address = pvKey.PubKey.Address()
	pvKey.filePath = keyFilePath

	pvState := FilePVLastSignState{}

	if loadState {
		stateJSONBytes, err := ioutil.ReadFile(stateFilePath)
		if err != nil {
			sros.Exit(err.Error())
		}
		err = srjson.Unmarshal(stateJSONBytes, &pvState)
		if err != nil {
			sros.Exit(fmt.Sprintf("Error reading PrivValidator state from %v: %v\n", stateFilePath, err))
		}
	}

	pvState.filePath = stateFilePath

	return &FilePV{
		Key:           pvKey,
		LastSignState: pvState,
	}
}

// LoadOrGenFilePV 如果硬盘在指定位置存储了 FilePV，就从硬盘里加载，
// 否则就随机生成一个 FilePV，然后再持久化到硬盘里
func LoadOrGenFilePV(keyFilePath, stateFilePath string) *FilePV {
	var pv *FilePV
	if sros.FileExists(keyFilePath) {
		pv = LoadFilePV(keyFilePath, stateFilePath)
	} else {
		pv = GenFilePV(keyFilePath, stateFilePath)
		pv.Save()
	}
	return pv
}

// GetAddress returns the address of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetAddress() Address {
	return pv.Key.Address
}

// GetPubKey returns the public key of the validator.
// Implements PrivValidator.
func (pv *FilePV) GetPubKey() (crypto.PubKey, error) {
	return pv.Key.PubKey, nil
}

// SignVote 将 chainID 和 prototypes.Vote 整合成 CanonicalVote，其中包含如下 6 个字段：
//	Type、Height、Round、BlockID、Timestamp 和 ChainID
// 然后将 CanonicalVote 编码成 protobuf 的字节码：bz，其中 bz 的内部结构如下：
//	bz:<CanonicalVote的长度+CanonicalVote的字节码>
// 	注意：bz 中不包含签名字段，即 Signature
// 接着比较 bz 与 FilePV.LastSignState.SignBytes，如果二者相等，则直接让 vote.Signature
// 等于 FilePV.LastSignState.Signature，如果二者除了时间戳不同，其余相同，则让 vote.Timestamp
// 等于 FilePV.LastSignState.Timestamp，除此以外，让 vote.Signature 等于
// FilePV.LastSignState.Signature，如果二者除了时间戳不同，还存在其他字段不相同，则代表投票冲突，
// 并返回错误，检查通过以后，对 bz 进行签名得到 sig，让 vote.Signature 等于 sig，并更新 FilePV.LastSignState
// 的 SignBytes 和 Signature，最后将 FilePV.LastSignState 持久化到硬盘上
func (pv *FilePV) SignVote(chainID string, vote *types.Vote) error {
	if err := pv.signVote(chainID, vote); err != nil {
		return fmt.Errorf("error signing vote: %v", err)
	}
	return nil
}

// SignProposal 将 chainID 和 prototypes.proposal 整合成 CanonicalProposal，其中包含如下 7 个字段：
//	Type、Height、Round、POLRound、BlockID、Timestamp 和 ChainID
// 然后将 CanonicalProposal 编码成 protobuf 的字节码：bz，其中 bz 的内部结构如下：
//	bz:<CanonicalProposal的大小+CanonicalProposal的字节码>
// 	注意：bz 中不包含签名字段，即 Signature
// 接着比较 bz 与 FilePV.LastSignState.SignBytes，如果二者相等，则直接让 proposal.Signature
// 等于 FilePV.LastSignState.Signature，如果二者除了时间戳不同，其余相同，则让 proposal.Timestamp
// 等于 FilePV.LastSignState.Timestamp，除此以外，让 proposal.Signature 等于
// FilePV.LastSignState.Signature，如果二者除了时间戳不同，还存在其他字段不相同，则代表提案冲突，
// 并返回错误，检查通过以后，对 bz 进行签名得到 sig，让 proposal.Signature 等于 sig，并更新 FilePV.LastSignState
// 的 SignBytes 和 Signature，最后将 FilePV.LastSignState 持久化到硬盘上
func (pv *FilePV) SignProposal(chainID string, prePrepare *types.PrePrepare) error {
	if err := pv.signProposal(chainID, prePrepare); err != nil {
		return fmt.Errorf("error signing prePrepare: %v", err)
	}
	return nil
}

// Save persists the FilePV to disk.
func (pv *FilePV) Save() {
	pv.Key.Save()
	pv.LastSignState.Save()
}

// Reset resets all fields in the FilePV.
// NOTE: Unsafe!
func (pv *FilePV) Reset() {
	var sig []byte
	pv.LastSignState.Height = 0
	pv.LastSignState.Round = 0
	pv.LastSignState.Step = 0
	pv.LastSignState.Signature = sig
	pv.LastSignState.SignBytes = nil
	pv.Save()
}

// String returns a string representation of the FilePV.
func (pv *FilePV) String() string {
	return fmt.Sprintf(
		"PrivValidator{%v LH:%v, LR:%v, LS:%v}",
		pv.GetAddress(),
		pv.LastSignState.Height,
		pv.LastSignState.Round,
		pv.LastSignState.Step,
	)
}

//------------------------------------------------------------------------------------

// signVote checks if the vote is good to sign and sets the vote signature.
// It may need to set the timestamp as well if the vote is otherwise the same as
// a previously signed vote (ie. we crashed after signing but before the vote hit the WAL).
func (pv *FilePV) signVote(chainID string, vote *types.Vote) error {
	// 获得新的投票的区块高度、轮次以及投票阶段
	height, round, step := vote.Height, vote.Round, voteToStep(vote)

	// 获取上一次的投票信息
	lss := pv.LastSignState

	// 只有在 height、round、step 都等于 lss 的相应字段后，并且 lss.SignBytes
	// 与 lss.Signature 都不为空时，sameHRS 等于 true
	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	// 投票的 protobuf 字节码形式
	// signBytes 里不包含 signature 字段
	signBytes := VoteSignBytes(chainID, vote)

	// 如新投票的 Type、Height、Round、BlockID、Timestamp 和
	// ChainID 等于旧投票，那么直接让新投票的签名等于旧投票的签名
	// 如果新旧投票除了时间戳不同，其他都相同，那么直接让新投票的时间戳等于旧投票的时间戳，
	// 并且让新投票的签名等于旧投票的签名
	if sameHRS { // 如果 height、round、step 都等于 lss 的相应字段
		// 如果新投票的 Type、Height、Round、BlockID、Timestamp 和
		// ChainID 等于旧投票，那么直接让新投票的签名等于旧投票的签名
		if bytes.Equal(signBytes, lss.SignBytes) { // signBytes 与 lss.SignBytes 里不包含签名字段
			// 新的投票与旧投票完全一样的情况下
			vote.Signature = lss.Signature
		} else if timestamp, ok := checkVotesOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			// 新投票与旧投票除了时间戳不一样，其他都一样的情况下
			vote.Timestamp = timestamp
			vote.Signature = lss.Signature
		} else {
			// 新旧投票冲突
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	// 通过了检查后，对投票进行签名
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	// 为新投票赋予签名信息
	vote.Signature = sig
	return nil
}

// signProposal checks if the proposal is good to sign and sets the proposal signature.
// It may need to set the timestamp as well if the proposal is otherwise the same as
// a previously signed proposal ie. we crashed after signing but before the proposal hit the WAL).
func (pv *FilePV) signProposal(chainID string, prePrepare *types.PrePrepare) error {
	height, round, step := prePrepare.Height, prePrepare.Round, PrePrepareStep

	lss := pv.LastSignState

	sameHRS, err := lss.CheckHRS(height, round, step)
	if err != nil {
		return err
	}

	// 提案 protobuf 字节码形式
	// signBytes 里不包含 signature 字段
	signBytes := PrePrepareSignBytes(chainID, prePrepare)

	// 如新 signBytes 的 Type、Height、Round、POLRound、BlockID、Timestam
	// 和 ChainID 等于 lss.SignBytes，那么直接让新 prePrepare 的签名等于 lss.Signature，
	// 如果新旧 prePrepare 除了时间戳不同，其他都相同，那么直接让新 prePrepare 的时间戳等于 lss.Timestamp，
	// 并且让新 prePrepare 的签名等于 lss.Signature
	if sameHRS {
		// 如果新 prePrepare 的 Type、Height、Round、POLRound、BlockID、Timestam
		// 和 ChainID 等于 lss.SignBytes，那么直接让新 prePrepare 的签名等于 lss.Signature
		if bytes.Equal(signBytes, lss.SignBytes) {
			prePrepare.Signature = lss.Signature
		} else if timestamp, ok := checkProposalsOnlyDifferByTimestamp(lss.SignBytes, signBytes); ok {
			prePrepare.Timestamp = timestamp
			prePrepare.Signature = lss.Signature
		} else {
			err = fmt.Errorf("conflicting data")
		}
		return err
	}

	// It passed the checks. Sign the prePrepare
	sig, err := pv.Key.PrivKey.Sign(signBytes)
	if err != nil {
		return err
	}
	pv.saveSigned(height, round, step, signBytes, sig)
	// 为新 prePrepare 赋予签名信息
	prePrepare.Signature = sig
	return nil
}

// Persist height/round/step and signature
func (pv *FilePV) saveSigned(height int64, round int32, step int8,
	signBytes []byte, sig []byte) {

	pv.LastSignState.Height = height
	pv.LastSignState.Round = round
	pv.LastSignState.Step = step
	pv.LastSignState.Signature = sig // 更新 FilePV.LastSignState.Signature
	pv.LastSignState.SignBytes = signBytes
	pv.LastSignState.Save()
}

//-----------------------------------------------------------------------------------------

// 返回 lastSignBytes 的时间戳，并且，如果 newSignBytes 与 lastSignBytes 除了时间戳
// 不一样，其他都一样，则返回 true
func checkVotesOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastVote, newVote types.CanonicalVote
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastVote); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into vote: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newVote); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into vote: %v", err))
	}

	lastTime := lastVote.Timestamp
	// 让新旧投票的时间戳一致，再检查新旧投票是否相同的
	now := srtime.Now()
	lastVote.Timestamp = now
	newVote.Timestamp = now

	return lastTime, proto.Equal(&newVote, &lastVote)
}

// 返回 lastSignBytes 的时间戳，并且，如果 newSignBytes 与 lastSignBytes 除了时间戳
// 不一样，其他都一样，则返回 true
func checkProposalsOnlyDifferByTimestamp(lastSignBytes, newSignBytes []byte) (time.Time, bool) {
	var lastPrePrepare, newPrePrepare types.CanonicalPrePrepare
	if err := protoio.UnmarshalDelimited(lastSignBytes, &lastPrePrepare); err != nil {
		panic(fmt.Sprintf("LastSignBytes cannot be unmarshalled into proposal: %v", err))
	}
	if err := protoio.UnmarshalDelimited(newSignBytes, &newPrePrepare); err != nil {
		panic(fmt.Sprintf("signBytes cannot be unmarshalled into proposal: %v", err))
	}

	lastTime := lastPrePrepare.Timestamp
	// set the times to the same value and check equality
	now := srtime.Now()
	lastPrePrepare.Timestamp = now
	newPrePrepare.Timestamp = now

	return lastTime, proto.Equal(&newPrePrepare, &lastPrePrepare)
}
