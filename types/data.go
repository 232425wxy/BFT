package types

import (
	srbytes "BFT/libs/bytes"
	srmath "BFT/libs/math"
	"BFT/proto/types"
	"errors"
	"fmt"
	"strings"
)

// Data 包含一个区块中所含有的所有 transaction
type Data struct {

	// Txs将被状态@ block.Height+1应用。
	// 注意:这里并不是所有的 tx 都是有效的，我们只是先就顺序达成一致，
	// 这意味着 block.AppHash 不包括这些 txs。
	Txs Txs `json:"txs"`

	// 易变的
	// Hash 将一组 transaction 的哈希值作为 merkle 的叶子，
	// 然后计算根哈希值， 作为这一组 transaction 的哈希值
	hash srbytes.HexBytes
}

// Hash returns the hash of the data
func (data *Data) Hash() srbytes.HexBytes {
	if data == nil {
		return (Txs{}).Hash()
	}
	if data.hash == nil {
		data.hash = data.Txs.Hash() // 注意：merkle 的叶子是 tx 的哈希值！
	}
	return data.hash
}

// StringIndented returns an indented string representation of the transactions.
func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, srmath.MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("%X (%d bytes)", tx.Hash(), len(tx))
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%v`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts Data to protobuf
func (data *Data) ToProto() types.Data {
	tp := new(types.Data)

	if len(data.Txs) > 0 {
		txBzs := make([][]byte, len(data.Txs))
		for i := range data.Txs {
			txBzs[i] = data.Txs[i]
		}
		tp.Txs = txBzs
	}

	return *tp
}

// DataFromProto takes a protobuf representation of Data &
// returns the native type.
func DataFromProto(dp *types.Data) (Data, error) {
	if dp == nil {
		return Data{}, errors.New("nil data")
	}
	data := new(Data)

	if len(dp.Txs) > 0 {
		txBzs := make(Txs, len(dp.Txs))
		for i := range dp.Txs {
			txBzs[i] = Tx(dp.Txs[i])
		}
		data.Txs = txBzs
	} else {
		data.Txs = Txs{}
	}

	return *data, nil
}

