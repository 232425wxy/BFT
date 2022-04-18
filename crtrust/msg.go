package crtrust

import (
	"fmt"
	"github.com/232425wxy/BFT/crypto"
	cryptoenc "github.com/232425wxy/BFT/crypto/encoding"
	protoabci "github.com/232425wxy/BFT/proto/abci"
	prototrust "github.com/232425wxy/BFT/proto/crtrust"
	prototypes "github.com/232425wxy/BFT/proto/types"
	"github.com/gogo/protobuf/proto"
)

type Notice struct {
	Src string
	Height int64
}

func (n *Notice) ToProto() *prototrust.Notice {
	return &prototrust.Notice{Src: n.Src, Height: n.Height}
}

func NoticeFromProto(pb *prototrust.Notice) *Notice {
	return &Notice{Src: pb.Src, Height: pb.Height}
}

func CanonicalEncodeNotice(n *Notice) []byte {
	pb := n.ToProto()
	bz, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return bz
}

func CanonicalDecodeNotice(bz []byte) *Notice {
	var pb prototrust.Notice
	err := proto.Unmarshal(bz, &pb)
	if err != nil {
		panic(err)
	}
	return NoticeFromProto(&pb)
}

// --------------------------------------------------------------------

type LocalEvaluation struct {
	PeerEval map[string]float64
	Src string
	Height int64
}

func (local *LocalEvaluation) ToProto() *prototrust.LocalEvaluation {
	return &prototrust.LocalEvaluation{
		PeerEval: local.PeerEval,
		Src:      local.Src,
		Height:   local.Height,
	}
}

func LocalEvaluationFromProto(pb *prototrust.LocalEvaluation) *LocalEvaluation {
	return &LocalEvaluation{
		PeerEval: pb.GetPeerEval(),
		Src:      pb.GetSrc(),
		Height:   pb.GetHeight(),
	}
}

func CanonicalEncodeLocalEvaluation(local *LocalEvaluation) []byte {
	pb := local.ToProto()
	bz, err := proto.Marshal(pb)
	if err != nil {
		panic(err)
	}
	return bz
}

func CanonicalDecodeLocalEvaluation(bz []byte) *LocalEvaluation {
	var pb prototrust.LocalEvaluation
	err := proto.Unmarshal(bz, &pb)
	if err != nil {
		panic(err)
	}
	return LocalEvaluationFromProto(&pb)
}

// --------------------------------------------------------------------

type Update struct {
	Id string
	Pubkey crypto.PubKey
	Trust float64
	Type prototypes.ValidatorType
}

type Updates []*Update

func (u Updates) Len() int {
	return len(u)
}

func (u Updates) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

func (u Updates) Less(i, j int) bool {
	return u[i].Trust < u[j].Trust
}

func (u Updates) String() string {
	str := ""
	for _, uu := range u {
		str += fmt.Sprintf(" ->%s : %.8f : %v", uu.Id[:5], uu.Trust, uu.Type)
	}
	return str
}

func (u Updates) modify(byzantines map[string]struct{}) {
	nums := len(byzantines) + 1  // 决定要让多少个节点成为非共识节点
	for i := 0; i < nums; i++ {
		u[i].Type = prototypes.ValidatorType_NORMAL
		if !isAlreadyExist(u[i].Id, byzantines) {
			byzantines[u[i].Id] = struct{}{}
		}
	}
}

func isAlreadyExist(id string, byzantines map[string]struct{}) bool {
	for idd, _ := range byzantines {
		if id == idd {
			return true
		}
	}
	return false
}

func (u Updates) constructUpdateValidators(consensus int, height int64) []protoabci.ValidatorUpdate {
	power := 10.0 / float64(consensus)
	vus := make([]protoabci.ValidatorUpdate, len(u))
	for i := 0; i < len(u); i++ {
		pk, err := cryptoenc.PubKeyToProto(u[i].Pubkey)
		if err != nil {
			panic(err)
		}
		vus[i] = protoabci.ValidatorUpdate{
			PubKey:  pk,
			Power:   0,
			Type:    u[i].Type,
			Height:  height,
		}
		if vus[i].Type == prototypes.ValidatorType_NORMAL {
			vus[i].Power = 0.0
		} else {
			vus[i].Power = power
			vus[i].Type = prototypes.ValidatorType_CANDIDATE_PRIMARY
		}
	}
	return vus
}