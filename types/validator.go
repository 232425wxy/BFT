package types

import (
	"github.com/232425wxy/BFT/crypto"
	cryptoenc "github.com/232425wxy/BFT/crypto/encoding"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"errors"
	"fmt"
)

type Address = crypto.Address

type Validator struct {
	Address Address `json:"address"`
	PubKey crypto.PubKey `json:"pub_key"`
	VotingPower float64                  `json:"voting_power"`
	Type        prototypes.ValidatorType `json:"type"`
	Trust       float64                  `json:"trust"`
}

func NewValidator(pubkey crypto.PubKey, votingPower float64) *Validator {
	return &Validator{
		Address:     pubkey.Address(),
		PubKey:      pubkey,
		VotingPower: votingPower,
		Type:        prototypes.ValidatorType_CANDIDATE_PRIMARY,
		Trust:       1.0,
	}
}

func (v *Validator) ValidateBasic() error {
	if v == nil {
		return errors.New("nil validator")
	}
	if v.PubKey == nil {
		return errors.New("validator does not have a public key")
	}

	if v.VotingPower < 0 {
		return errors.New("validator has negative voting power")
	}

	if len(v.Address) != crypto.AddressSize {
		return fmt.Errorf("validator address is the wrong size: %v", v.Address)
	}

	return nil
}

func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{Address:%v VotingPower:%v Trust:%.3f Type:%v}", v.Address, v.VotingPower, v.Trust, v.Type)
}

func (v *Validator) Bytes() []byte {
	pk, err := cryptoenc.PubKeyToProto(v.PubKey)
	if err != nil {
		panic(err)
	}

	pbv := prototypes.SimpleValidator{
		PubKey:      &pk,
	}

	bz, err := pbv.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

func (v *Validator) ToProto() (*prototypes.Validator, error) {
	if v == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := cryptoenc.PubKeyToProto(v.PubKey)
	if err != nil {
		return nil, err
	}

	vp := prototypes.Validator{
		Address:          v.Address,
		PubKey:           pk,
		VotingPower:      v.VotingPower,
		Type:             v.Type,
		Trust:            v.Trust,
	}

	return &vp, nil
}

func ValidatorFromProto(vp *prototypes.Validator) (*Validator, error) {
	if vp == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := cryptoenc.PubKeyFromProto(vp.PubKey)
	if err != nil {
		return nil, err
	}
	v := new(Validator)
	v.Address = vp.GetAddress()
	v.PubKey = pk
	v.VotingPower = vp.GetVotingPower()
	v.Type = vp.GetType()
	v.Trust = vp.GetTrust()

	return v, nil
}