package types

import (
	"BFT/crypto/merkle"
	"BFT/crypto/srhash"
	prototypes "BFT/proto/types"
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strings"
)

const TotalVotingPower float64 = 10.0

// ValidatorSet represent a set of *Validator at a given height.
type ValidatorSet struct {
	Validators []*Validator `json:"validators"`
	Primary   *Validator   `json:"proposer"`
}

func NewValidatorSet(valz []*Validator) *ValidatorSet {
	vals := &ValidatorSet{}
	err := vals.updateWithChangeSet(valz)
	for i := 0; i < len(vals.Validators); i++ {
		vals.Validators[i].Type = prototypes.ValidatorType_CANDIDATE_PRIMARY
		vals.Validators[i].VotingPower = TotalVotingPower / float64(len(valz))
	}
	if err != nil {
		panic(fmt.Sprintf("Cannot create validator set: %v", err))
	}
	vals.iterateToGetPrimary(BlockID{}, 0)
	return vals
}

func (vals *ValidatorSet) ValidateBasic() error {
	if vals.IsNilOrEmpty() {
		return errors.New("validator set is nil or empty")
	}

	for idx, val := range vals.Validators {
		if err := val.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid validator #%d: %w", idx, err)
		}
	}

	if err := vals.Primary.ValidateBasic(); err != nil {
		return fmt.Errorf("primary failed validate basic, error: %w", err)
	}

	return nil
}

func (vals *ValidatorSet) IsNilOrEmpty() bool {
	return vals == nil || len(vals.Validators) == 0
}

func validatorListCopy(valsList []*Validator) []*Validator {
	if valsList == nil {
		return nil
	}
	valsCopy := make([]*Validator, len(valsList))
	for i, val := range valsList {
		valsCopy[i] = val.Copy()
	}
	return valsCopy
}

// Copy each validator into a new ValidatorSet.
func (vals *ValidatorSet) Copy() *ValidatorSet {
	return &ValidatorSet{
		Validators:       validatorListCopy(vals.Validators),
		Primary:          vals.Primary,
	}
}

func (vals *ValidatorSet) HasAddress(address []byte) bool {
	for _, val := range vals.Validators {
		if bytes.Equal(val.Address, address) {
			return true
		}
	}
	return false
}

func (vals *ValidatorSet) GetByAddress(address []byte) (index int32, val *Validator) {
	for idx, val := range vals.Validators {
		if bytes.Equal(val.Address, address) {
			return int32(idx), val.Copy()
		}
	}
	return -1, nil
}

func (vals *ValidatorSet) GetByIndex(index int32) (address []byte, val *Validator) {
	if index < 0 || int(index) >= len(vals.Validators) {
		return nil, nil
	}
	val = vals.Validators[index]
	return val.Address, val.Copy()
}

func (vals *ValidatorSet) Size() int {
	return len(vals.Validators)
}

func (vals *ValidatorSet) GetPrimary(seed BlockID, times int32) (proposer *Validator) {
	if len(vals.Validators) == 0 {
		return nil
	}
	if vals.Primary != nil {
		return vals.Primary.Copy()
	}
	vals.iterateToGetPrimary(seed, times)
	return vals.Primary.Copy()
}

func (vals *ValidatorSet) CopyIteratePrimary(seed BlockID, times int32) *ValidatorSet {
	if len(vals.Validators) == 0 {
		return nil
	}
	vals.iterateToGetPrimary(seed, times)
	var c_vals = *vals
	return &c_vals
}

func (vals *ValidatorSet) iterateToGetPrimary(seed BlockID, times int32) {
	var hash = seed.Hash
	var diff float64 = math.MaxFloat64
	var primary *Validator
	for times >= 0 {
		hash = srhash.Sum(hash)
		times--
	}
	for _, val := range vals.Validators {
		if val.Type == prototypes.ValidatorType_CANDIDATE_PRIMARY {
			hash_addr := srhash.Sum(val.Address)
			err := diffHash(hash, hash_addr)
			if err < diff {
				diff = err
				primary = val
			}
		}
	}
	vals.Primary = primary
}

func (vals *ValidatorSet) Hash() []byte {
	bzs := make([][]byte, len(vals.Validators))
	for i, val := range vals.Validators {
		bzs[i] = val.Bytes()
	}
	return merkle.HashFromByteSlices(bzs)
}

func processChanges(origChanges []*Validator) (updates []*Validator, err error) {
	changes := validatorListCopy(origChanges)
	sort.Sort(ValidatorsByAddress(changes))

	updates = make([]*Validator, 0, len(changes))
	var prevAddr Address

	// 由于对 changes 按照地址大小进行排序了，因此，两个一样的 Validator 会挨着放在一起
	for _, valUpdate := range changes {
		if bytes.Equal(valUpdate.Address, prevAddr) {
			err = fmt.Errorf("duplicate entry %v in %v", valUpdate, changes)
			return nil, err
		}

		switch {
		case valUpdate.VotingPower < 0:
			err = fmt.Errorf("voting power can't be negative: %d", valUpdate.VotingPower)
			return nil, err
		default: // 对其他合法的 Validator 更新过后放到新的 Validator 列表里
			updates = append(updates, valUpdate)
		}

		prevAddr = valUpdate.Address
	}

	return updates, err
}

func numNewValidators(updates []*Validator, vals *ValidatorSet) int {
	numNewValidators := 0
	for _, valUpdate := range updates {
		if !vals.HasAddress(valUpdate.Address) {
			numNewValidators++
		}
	}
	return numNewValidators
}

func (vals *ValidatorSet) applyUpdates(updates []*Validator) {
	existing := vals.Validators
	sort.Sort(ValidatorsByAddress(existing))

	merged := make([]*Validator, len(existing)+len(updates))
	i := 0

	for len(existing) > 0 && len(updates) > 0 {
		if bytes.Compare(existing[0].Address, updates[0].Address) < 0 {
			// 如果 existing[0] 的地址小于 updates[0] 的地址，就将 existing[0] 添加到 merged 里
			merged[i] = existing[0]
			existing = existing[1:]
		} else {
			// 否则就将 updates[0] 添加到 merged 里
			merged[i] = updates[0]
			if bytes.Equal(existing[0].Address, updates[0].Address) {
				// Validator is present in both, advance existing.
				existing = existing[1:]
			}
			updates = updates[1:]
		}
		i++
	}

	// Add the elements which are left.
	for j := 0; j < len(existing); j++ {
		merged[i] = existing[j]
		i++
	}
	// OR add updates which are left.
	for j := 0; j < len(updates); j++ {
		merged[i] = updates[j]
		i++
	}

	vals.Validators = merged[:i]
}

func (vals *ValidatorSet) updateWithChangeSet(changes []*Validator) error {
	if len(changes) == 0 {
		return nil
	}

	// 检查 changes 里面有没有重复的 Validator，并且将投票权大于 0 的 Validator 放入到 updates 中，
	// 而投票权等于 0 的 Validator 被放入到 removals 中
	updates, err := processChanges(changes)
	if err != nil {
		return err
	}

	// 将 updates 合并到 ValidatorSet 中
	vals.applyUpdates(updates)

	// 对 ValidatorSet 中的 Validator 按照地址大小进行排序，地址小的排在前面
	sort.Sort(ValidatorsByAddress(vals.Validators))

	return nil
}

func (vals *ValidatorSet) UpdateWithChangeSet(changes []*Validator) error {
	return vals.updateWithChangeSet(changes)
}

type ValidatorsByAddress []*Validator

func (valz ValidatorsByAddress) Len() int { return len(valz) }

func (valz ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(valz[i].Address, valz[j].Address) == -1
}

func (valz ValidatorsByAddress) Swap(i, j int) {
	valz[i], valz[j] = valz[j], valz[i]
}

func (vals *ValidatorSet) ToProto() (*prototypes.ValidatorSet, error) {

	//fmt.Println("HHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHH", vals.StringIndented(" "))
	if vals.IsNilOrEmpty() {
		return &prototypes.ValidatorSet{}, nil // validator set should never be nil
	}

	vp := new(prototypes.ValidatorSet)
	valsProto := make([]*prototypes.Validator, len(vals.Validators))
	for i := 0; i < len(vals.Validators); i++ {
		valp, err := vals.Validators[i].ToProto()
		if err != nil {
			return nil, err
		}
		valsProto[i] = valp
	}
	vp.Validators = valsProto

	valProposer, err := vals.Primary.ToProto()
	if err != nil {
		const size = 64 << 10
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		fmt.Printf("%s\n", buf)
		return nil, fmt.Errorf("toProto: validatorSet proposer error: %w", err)
	}
	vp.Primary = valProposer

	return vp, nil
}

// ValidatorSetFromProto sets a protobuf ValidatorSet to the given pointer.
// It returns an error if any of the validators from the set or the proposer
// is invalid
func ValidatorSetFromProto(vp *prototypes.ValidatorSet) (*ValidatorSet, error) {
	if vp == nil {
		return nil, errors.New("nil validator set") // validator set should never be nil, bigger issues are at play if empty
	}
	vals := new(ValidatorSet)

	valsProto := make([]*Validator, len(vp.Validators))
	for i := 0; i < len(vp.Validators); i++ {
		v, err := ValidatorFromProto(vp.Validators[i])
		if err != nil {
			return nil, err
		}
		valsProto[i] = v
	}
	vals.Validators = valsProto

	p, err := ValidatorFromProto(vp.GetPrimary())
	if err != nil {
		return nil, fmt.Errorf("fromProto: validatorSet proposer error: %w", err)
	}

	vals.Primary = p

	return vals, vals.ValidateBasic()
}

func (vals *ValidatorSet) VerifyReply(chainID string, blockID BlockID,
	height int64, reply *Reply) error {

	if vals.Size() != len(reply.Signatures) {
		return NewErrInvalidCommitSignatures(vals.Size(), len(reply.Signatures))
	}

	// Validate Height and BlockID.
	if height != reply.Height {
		return NewErrInvalidCommitHeight(height, reply.Height)
	}
	if !blockID.Equals(reply.BlockID) {
		return fmt.Errorf("invalid reply -- wrong block ID: want %v, got %v",
			blockID, reply.BlockID)
	}

	talliedVotingPower := float64(0)
	votingPowerNeeded := TotalVotingPower * 2.0 / 3.0
	for idx, commitSig := range reply.Signatures {
		if commitSig.Absent() {
			continue // OK, some signatures can be absent.
		}

		// The vals and reply have a 1-to-1 correspondance.
		// This means we don't need the validator address or to do any lookup.
		val := vals.Validators[idx]

		// Validate signature.
		voteSignBytes := reply.VoteSignBytes(chainID, int32(idx))
		if !val.PubKey.VerifySignature(voteSignBytes, commitSig.Signature) {
			return fmt.Errorf("wrong signature (#%d): %X", idx, commitSig.Signature)
		}
		// Good!
		if commitSig.ForBlock() {
			talliedVotingPower += val.VotingPower
		}
	}

	if got, needed := talliedVotingPower, votingPowerNeeded; got <= needed {
		return ErrNotEnoughVotingPowerSigned{Got: got, Needed: needed}
	}

	return nil
}

func (vals *ValidatorSet) StringIndented(indent string) string {
	if vals == nil {
		return "nil-ValidatorSet"
	}
	var valStrings []string
	vals.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
%s  Primary: %v
%s  Validators:
%s    %v
%s}`,
		indent, vals.Primary.String(),
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}

func (vals *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range vals.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}