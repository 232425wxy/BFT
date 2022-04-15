package types

import (
	"BFT/crypto/srhash"
	prototypes "BFT/proto/types"
	"errors"
	"fmt"
)

const (
	// MaxBlockSizeBytes 一个区块的最大大小 100MB
	MaxBlockSizeBytes = 104857600 // 100MB

	// BlockPartSizeBytes 是区块的 part 的大小 64KB
	BlockPartSizeBytes uint32 = 65536 // 64kB

	// MaxBlockPartsCount 一个区块最多能分成多少个 part：1601 个 part
	MaxBlockPartsCount = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1
)

// DefaultConsensusParams 返回默认的共识参数
func DefaultConsensusParams() *prototypes.ConsensusParams {
	return &prototypes.ConsensusParams{
		Block:     DefaultBlockParams(),
		Validator: DefaultValidatorParams(),
	}
}

// DefaultBlockParams 返回默认的共识参数
func DefaultBlockParams() prototypes.BlockParams {
	return prototypes.BlockParams{
		MaxBytes:   22020096, // 21MB
		TimeIotaMs: 1000,     // 1s
	}
}

// DefaultValidatorParams returns a default ValidatorParams, which allows
// only ed25519 pubkeys.
func DefaultValidatorParams() prototypes.ValidatorParams {
	return prototypes.ValidatorParams{
		PubKeyTypes: []string{ABCIPubKeyTypeEd25519},
	}
}

func IsValidPubkeyType(params prototypes.ValidatorParams, pubkeyType string) bool {
	for i := 0; i < len(params.PubKeyTypes); i++ {
		if params.PubKeyTypes[i] == pubkeyType {
			return true
		}
	}
	return false
}

// Validate validates the ConsensusParams to ensure all values are within their
// allowed limits, and returns an error if they are not.
func ValidateConsensusParams(params prototypes.ConsensusParams) error {
	if params.Block.MaxBytes <= 0 {
		return fmt.Errorf("block.MaxBytes must be greater than 0. Got %d",
			params.Block.MaxBytes)
	}
	if params.Block.MaxBytes > MaxBlockSizeBytes {
		return fmt.Errorf("block.MaxBytes is too big. %d > %d",
			params.Block.MaxBytes, MaxBlockSizeBytes)
	}

	if params.Block.TimeIotaMs <= 0 {
		return fmt.Errorf("block.TimeIotaMs must be greater than 0. Got %v",
			params.Block.TimeIotaMs)
	}

	if len(params.Validator.PubKeyTypes) == 0 {
		return errors.New("len(Validator.PubKeyTypes) must be greater than 0")
	}

	// Check if keyType is a known ABCIPubKeyType
	for i := 0; i < len(params.Validator.PubKeyTypes); i++ {
		keyType := params.Validator.PubKeyTypes[i]
		if _, ok := ABCIPubKeyTypesToNames[keyType]; !ok {
			return fmt.Errorf("params.Validator.PubKeyTypes[%d], %s, is an unknown pubkey type",
				i, keyType)
		}
	}

	return nil
}

// HashConsensusParams 返回一个参数子集的哈希值，存储在块头中。
// 只有 Block.MaxBytes 包含在散列中。
// 这允许 ConsensusParams 在不破坏区块协议的情况下进行更多的演化。
// 这里不需要 Merkle 树，只需要一个小结构来哈希。
func HashConsensusParams(params prototypes.ConsensusParams) []byte {
	hasher := srhash.New()

	hp := prototypes.HashedParams{
		BlockMaxBytes: params.Block.MaxBytes,
	}

	bz, err := hp.Marshal()
	if err != nil {
		panic(err)
	}

	_, err = hasher.Write(bz)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// UpdateConsensusParams 返回一个 ConsensusParams 的副本，其中包含来自 params2 的非零字段的更新。
// NOTE: must not modify the original
func UpdateConsensusParams(params prototypes.ConsensusParams) prototypes.ConsensusParams {
	res := params // explicit copy
	return res
}
