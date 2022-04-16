package types

import (
	"github.com/232425wxy/BFT/libs/bytes"
	gogotypes "github.com/gogo/protobuf/types"
)

// cdcEncode 对 item 进行 protobuf.Marshal 编码
func cdcEncode(item interface{}) []byte {
	if item != nil && !isTypedNil(item) && !isEmpty(item) {
		switch item := item.(type) {
		case string:
			i := gogotypes.StringValue{
				Value: item,
			}
			bz, err := i.Marshal()
			if err != nil {
				return nil
			}
			return bz
		case int64:
			i := gogotypes.Int64Value{
				Value: item,
			}
			bz, err := i.Marshal()
			if err != nil {
				return nil
			}
			return bz
		case bytes.HexBytes:
			i := gogotypes.BytesValue{
				Value: item,
			}
			bz, err := i.Marshal()
			if err != nil {
				return nil
			}
			return bz
		default:
			return nil
		}
	}

	return nil
}
