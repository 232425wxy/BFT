package types

import (
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/crypto/ed25519"
	cryptoenc "github.com/232425wxy/BFT/crypto/encoding"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	prototypes "github.com/232425wxy/BFT/proto/types"
)

//-------------------------------------------------------
// Use strings to distinguish types in ABCI messages

const (
	ABCIPubKeyTypeEd25519 = ed25519.KeyType
)

var ABCIPubKeyTypesToNames = map[string]string{
	ABCIPubKeyTypeEd25519: ed25519.PubKeyName,
}

//-------------------------------------------------------

// UNSTABLE
var SR2PB = sr2pb{}

type sr2pb struct{}

func (sr2pb) Header(header *Header) prototypes.Header {
	return prototypes.Header{
		ChainID: header.ChainID,
		Height:  header.Height,
		Time:    header.Time,

		LastBlockId: header.LastBlockID.ToProto(),

		LastCommitHash: header.LastCommitHash,
		DataHash:       header.DataHash,

		ConsensusHash:      header.ConsensusHash,
		AppHash:            header.AppHash,

		ProposerAddress: header.ProposerAddress,
	}
}

func (sr2pb) Validator(val *Validator) protoabci.Validator {
	return protoabci.Validator{
		Address: val.PubKey.Address(),
		Power:   val.VotingPower,
	}
}

func (sr2pb) BlockID(blockID BlockID) prototypes.BlockID {
	return prototypes.BlockID{
		Hash:          blockID.Hash,
		PartSetHeader: SR2PB.PartSetHeader(blockID.PartSetHeader),
	}
}

func (sr2pb) PartSetHeader(header PartSetHeader) prototypes.PartSetHeader {
	return prototypes.PartSetHeader{
		Total: header.Total,
		Hash:  header.Hash,
	}
}

// XXX: panics on unknown pubkey type
func (sr2pb) ValidatorUpdate(val *Validator) protoabci.ValidatorUpdate {
	pk, err := cryptoenc.PubKeyToProto(val.PubKey)
	if err != nil {
		panic(err)
	}
	return protoabci.ValidatorUpdate{
		PubKey: pk,
		Power:  val.VotingPower,
	}
}

// XXX: panics on nil or unknown pubkey type
func (sr2pb) ValidatorUpdates(vals *ValidatorSet) []protoabci.ValidatorUpdate {
	validators := make([]protoabci.ValidatorUpdate, vals.Size())
	for i, val := range vals.Validators {
		validators[i] = SR2PB.ValidatorUpdate(val)
	}
	return validators
}

// XXX: panics on nil or unknown pubkey type
func (sr2pb) NewValidatorUpdate(pubkey crypto.PubKey, power float64) protoabci.ValidatorUpdate {
	pubkeyABCI, err := cryptoenc.PubKeyToProto(pubkey)
	if err != nil {
		panic(err)
	}
	return protoabci.ValidatorUpdate{
		PubKey: pubkeyABCI,
		Power:  power,
	}
}

//----------------------------------------------------------------------------

// PB2SR is used for converting protobuf ABCI to SRBFT protoabci.
// UNSTABLE
var PB2SR = pb2sr{}

type pb2sr struct{}

func (pb2sr) ValidatorUpdates(vals []protoabci.ValidatorUpdate) ([]*Validator, error) {
	crVals := make([]*Validator, len(vals))
	for i, v := range vals {
		pub, err := cryptoenc.PubKeyFromProto(v.PubKey)
		if err != nil {
			return nil, err
		}
		crVals[i] = NewValidator(pub, v.Power)
		crVals[i].Type = v.Type
	}
	return crVals, nil
}
