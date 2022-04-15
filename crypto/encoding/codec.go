package encoding

import (
	"BFT/crypto"
	"BFT/crypto/ed25519"
	"BFT/libs/json"
	pc "BFT/proto/crypto"
	"fmt"
)

func init() {
	json.RegisterType((*pc.PublicKey)(nil), "BFT.crypto.PublicKey")
	json.RegisterType((*pc.PublicKey_Ed25519)(nil), "BFT.crypto.PublicKey_Ed25519")
}

// PubKeyToProto 将公钥转换成 protobuf 形式的
func PubKeyToProto(k crypto.PubKey) (pc.PublicKey, error) {
	var kp pc.PublicKey
	switch k := k.(type) {
	case ed25519.PubKey:
		kp = pc.PublicKey{
			Sum: &pc.PublicKey_Ed25519{
				Ed25519: k,
			},
		}
	default:
		return kp, fmt.Errorf("toproto: key type %v is not supported", k)
	}
	return kp, nil
}

// PubKeyFromProto 将 protobuf 形式的公钥转换成自定义的
func PubKeyFromProto(k pc.PublicKey) (crypto.PubKey, error) {
	switch k := k.Sum.(type) {
	case *pc.PublicKey_Ed25519:
		if len(k.Ed25519) != ed25519.PubKeySize {
			return nil, fmt.Errorf("invalid size for PubKeyEd25519. Got %d, expected %d",
				len(k.Ed25519), ed25519.PubKeySize)
		}
		pk := make(ed25519.PubKey, ed25519.PubKeySize)
		copy(pk, k.Ed25519)
		return pk, nil
	default:
		return nil, fmt.Errorf("fromproto: key type %v is not supported", k)
	}
}
