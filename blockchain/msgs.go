package blockchain

import (
	protoblockchain "github.com/232425wxy/BFT/proto/blockchain"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"

	"github.com/232425wxy/BFT/types"
)

const (
	// NOTE: keep up to date with protoblockchain.BlockResponse
	BlockResponseMessagePrefixSize   = 4
	BlockResponseMessageFieldKeySize = 1
	MaxMsgSize                       = types.MaxBlockSizeBytes + BlockResponseMessagePrefixSize + BlockResponseMessageFieldKeySize
)

// EncodeMsg encodes a Protobuf message
func EncodeMsg(pb proto.Message) ([]byte, error) {
	msg := protoblockchain.Message{}

	switch pb := pb.(type) {
	case *protoblockchain.BlockRequest:
		msg.Sum = &protoblockchain.Message_BlockRequest{BlockRequest: pb}
	case *protoblockchain.BlockResponse:
		msg.Sum = &protoblockchain.Message_BlockResponse{BlockResponse: pb}
	case *protoblockchain.NoBlockResponse:
		msg.Sum = &protoblockchain.Message_NoBlockResponse{NoBlockResponse: pb}
	case *protoblockchain.StatusRequest:
		msg.Sum = &protoblockchain.Message_StatusRequest{StatusRequest: pb}
	case *protoblockchain.StatusResponse:
		msg.Sum = &protoblockchain.Message_StatusResponse{StatusResponse: pb}
	default:
		return nil, fmt.Errorf("unknown message type %T", pb)
	}

	bz, err := proto.Marshal(&msg)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal %T: %w", pb, err)
	}

	return bz, nil
}

// DecodeMsg decodes a Protobuf message.
func DecodeMsg(bz []byte) (proto.Message, error) {
	pb := &protoblockchain.Message{}

	err := proto.Unmarshal(bz, pb)
	if err != nil {
		return nil, err
	}

	switch msg := pb.Sum.(type) {
	case *protoblockchain.Message_BlockRequest:
		return msg.BlockRequest, nil
	case *protoblockchain.Message_BlockResponse:
		return msg.BlockResponse, nil
	case *protoblockchain.Message_NoBlockResponse:
		return msg.NoBlockResponse, nil
	case *protoblockchain.Message_StatusRequest:
		return msg.StatusRequest, nil
	case *protoblockchain.Message_StatusResponse:
		return msg.StatusResponse, nil
	default:
		return nil, fmt.Errorf("unknown message type %T", msg)
	}
}

// ValidateMsg validates a message.
func ValidateMsg(pb proto.Message) error {
	if pb == nil {
		return errors.New("message cannot be nil")
	}

	switch msg := pb.(type) {
	case *protoblockchain.BlockRequest:
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
	case *protoblockchain.BlockResponse:
		_, err := types.BlockFromProto(msg.Block)
		if err != nil {
			return err
		}
	case *protoblockchain.NoBlockResponse:
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
	case *protoblockchain.StatusResponse:
		if msg.Base < 0 {
			return errors.New("negative Base")
		}
		if msg.Height < 0 {
			return errors.New("negative Height")
		}
		if msg.Base > msg.Height {
			return fmt.Errorf("base %v cannot be greater than height %v", msg.Base, msg.Height)
		}
	case *protoblockchain.StatusRequest:
		return nil
	default:
		return fmt.Errorf("unknown message type %T", msg)
	}
	return nil
}
