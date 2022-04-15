package bits

import (
	srmath "BFT/libs/math"
	srrand "BFT/libs/rand"
	protobits "BFT/proto/libs/bits"
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// BitArray 是线程安全的 bit 数组
type BitArray struct {
	mtx   sync.Mutex
	Bits  int      `json:"bits"`  // bit 数组含有比特的个数，注意: 通过反射持久化，必须导出
	Elems []uint64 `json:"elems"` // 注意: 通过反射持久化，必须导出
}

// NewBitArray 创建一个新的 bit 数组
func NewBitArray(bits int) *BitArray {
	if bits <= 0 {
		return nil
	}
	return &BitArray{
		Bits:  bits,
		Elems: make([]uint64, (bits+63)/64),
	}
}

// Size 返回 bit 数组里比特的个数
func (bA *BitArray) Size() int {
	if bA == nil {
		return 0
	}
	return bA.Bits
}

// GetIndex 返回 bit 数组第 i 个位置的比特，i 不能大于 bit 数组里比特的个数
func (bA *BitArray) GetIndex(i int) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.getIndex(i)
}

func (bA *BitArray) getIndex(i int) bool {
	if i >= bA.Bits {
		return false
	}
	// 假设现在 bit 数组里有145个比特，现在我想获取第67个比特的值，那么 i 应该等于66，
	// bA.Elems[i/64] => bA.Elems[66/64] => bA.Elems[1];
	// i % 64 => 66 % 64 => 2, uint64(1)<<uint(i%64) => 1<<2 => 0100
	// bA.Elems[i/64]&(uint64(1)<<uint(i%64)) => bA.Elems[1]&(0100)
	// 让 bA.Elems 的第2个元素和（0100）做与运算，获得 bA.Elems 的第2个元素的第三个比特位置的值
	return bA.Elems[i/64]&(uint64(1)<<uint(i%64)) > 0
}

// SetIndex 设置 bit 数组第 i 个位置的比特值，i 不能大于 bit 数组里比特的个数
func (bA *BitArray) SetIndex(i int, v bool) bool {
	if bA == nil {
		return false
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.setIndex(i, v)
}

func (bA *BitArray) setIndex(i int, v bool) bool {
	if i >= bA.Bits {
		return false
	}
	if v {
		bA.Elems[i/64] |= (uint64(1) << uint(i%64))
	} else {
		// 如果 i 等于2，
		// uint64(1)<<uint(i%64) => uint64(1)<<uint(2) => 0100
		// ^(uint64(1)<<uint(i%64)) => ^(uint64(1)<<uint(2)) => ^(0100) => 1011
		bA.Elems[i/64] &= ^(uint64(1) << uint(i%64))
	}
	return true
}

// Copy 拷贝当前的 bit 数组，并返回拷贝出来的副本
func (bA *BitArray) Copy() *BitArray {
	if bA == nil {
		return nil
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.copy()
}

func (bA *BitArray) copy() *BitArray {
	c := make([]uint64, len(bA.Elems))
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bA.Bits,
		Elems: c,
	}
}

// copyBits
// 假设 bA 中存储了145个比特，现在传入的参数 bits 等于96，
// c := make([]uint64, (bits+63)/64) => c := make([]uint64, (96+63)/64) => c := make([]uint64, 2)
// copy(c, bA.Elems)，因此只复制了 bA.Elems 的前两个元素，即只复制了 bA 的前128位比特
func (bA *BitArray) copyBits(bits int) *BitArray {
	c := make([]uint64, (bits+63)/64)
	copy(c, bA.Elems)
	return &BitArray{
		Bits:  bits,
		Elems: c,
	}
}

// Or 返回两个 bit 数组按位或产生的 bit 数组，
// 如果两个 bit 数组的长度不同，或运算会在两个数组中较小的右边补0，
// 因此返回的数组大小等于所提供的两个 bit 数组中较大的那个数组的大小
func (bA *BitArray) Or(o *BitArray) *BitArray {
	if bA == nil && o == nil {
		return nil
	}
	if bA == nil && o != nil {
		return o.Copy()
	}
	if o == nil {
		return bA.Copy()
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	c := bA.copyBits(srmath.MaxInt(bA.Bits, o.Bits))
	smaller := srmath.MinInt(len(bA.Elems), len(o.Elems))
	for i := 0; i < smaller; i++ {
		c.Elems[i] |= o.Elems[i]
	}
	bA.mtx.Unlock()
	o.mtx.Unlock()
	return c
}

// And 返回两个 bit 数组按位与产生的 bit 数组，
// 如果两个 bit 数组的长度不一样，则会截断两个数组中较大的那个数组的右边
// 因此返回的数组大小等于所提供的两个 bit 数组中较小的那个数组的大小
func (bA *BitArray) And(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		return nil
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	defer func() {
		bA.mtx.Unlock()
		o.mtx.Unlock()
	}()
	return bA.and(o)
}

func (bA *BitArray) and(o *BitArray) *BitArray {
	c := bA.copyBits(srmath.MinInt(bA.Bits, o.Bits))
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] &= o.Elems[i]
	}
	return c
}

// Not 返回 bit 数组按位取反后的拷贝副本
func (bA *BitArray) Not() *BitArray {
	if bA == nil {
		return nil // Degenerate
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.not()
}

func (bA *BitArray) not() *BitArray {
	c := bA.copy()
	for i := 0; i < len(c.Elems); i++ {
		c.Elems[i] = ^c.Elems[i]
	}
	return c
}

// Sub 27=16+8+2+1 => 00011011，20=16+4 => 00010100
// 00011011 &^ 00010100 = 11 => 00001011
func (bA *BitArray) Sub(o *BitArray) *BitArray {
	if bA == nil || o == nil {
		// TODO: Decide if we should do 1's complement here?
		return nil
	}
	bA.mtx.Lock()
	o.mtx.Lock()
	// c 和 bA一样
	c := bA.copyBits(bA.Bits)
	// 获取提供的两个 bit 数组中长度较短的那个数组的长度
	smaller := srmath.MinInt(len(bA.Elems), len(o.Elems))
	for i := 0; i < smaller; i++ {
		// 27=16+8+2+1 => 00011011，20=16+4 => 00010100
		// 00011011 &^ 00010100 = 11 => 00001011
		c.Elems[i] &^= o.Elems[i]
	}
	bA.mtx.Unlock()
	o.mtx.Unlock()
	return c
}

// IsEmpty 如果 bit 数组里每个比特都是 0，则返回 true
func (bA *BitArray) IsEmpty() bool {
	if bA == nil {
		return true // should this be opposite?
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	for _, e := range bA.Elems {
		if e > 0 {
			return false
		}
	}
	return true
}

// IsFull 如果 bit 数组里每个比特都是 1，则返回 true
func (bA *BitArray) IsFull() bool {
	if bA == nil {
		return true
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	// 检查所有元素，除了最后一个
	for _, elem := range bA.Elems[:len(bA.Elems)-1] {
		if (^elem) != 0 {
			return false
		}
	}

	// 检查最后一个元素
	lastElemBits := (bA.Bits+63)%64 + 1
	lastElem := bA.Elems[len(bA.Elems)-1]
	return (lastElem+1)&((uint64(1)<<uint(lastElemBits))-1) == 0
}

// PickRandom 随机返回 bit 数组里比特值为 1 的位置索引
// bit 数组里没有比特等于 1，则返回 0 和 false
func (bA *BitArray) PickRandom() (int, bool) {
	if bA == nil {
		return 0, false
	}

	bA.mtx.Lock()
	trueIndices := bA.getTrueIndices()
	bA.mtx.Unlock()

	if len(trueIndices) == 0 { // bit 数组里没有比特等于 1
		return 0, false
	}

	return trueIndices[srrand.Intn(len(trueIndices))], true
}

// getTrueIndices 获取 bit 数组里值为 1 的所有位置索引
func (bA *BitArray) getTrueIndices() []int {
	trueIndices := make([]int, 0, bA.Bits)
	curBit := 0
	numElems := len(bA.Elems)
	// set all true indices
	for i := 0; i < numElems-1; i++ {
		elem := bA.Elems[i]
		if elem == 0 {
			curBit += 64
			continue
		}
		for j := 0; j < 64; j++ {
			if (elem & (uint64(1) << uint64(j))) > 0 {
				trueIndices = append(trueIndices, curBit)
			}
			curBit++
		}
	}
	// handle last element
	lastElem := bA.Elems[numElems-1]
	numFinalBits := bA.Bits - curBit
	for i := 0; i < numFinalBits; i++ {
		if (lastElem & (uint64(1) << uint64(i))) > 0 {
			trueIndices = append(trueIndices, curBit)
		}
		curBit++
	}
	return trueIndices
}

// String 用字符串表示 bit 数组，其中，用 “x” 表示 1，用 “_” 表示 0，
// 对于空数组，输出：“nil-BitArray”
func (bA *BitArray) String() string {
	return bA.StringIndented("")
}

// StringIndented returns the same thing as String(), but applies the indent
// at every 10th bit, and twice at every 50th bit.
func (bA *BitArray) StringIndented(indent string) string {
	if bA == nil {
		return "nil-BitArray"
	}
	bA.mtx.Lock()
	defer bA.mtx.Unlock()
	return bA.stringIndented(indent)
}

func (bA *BitArray) stringIndented(indent string) string {
	lines := []string{}
	bits := ""
	for i := 0; i < bA.Bits; i++ {
		if bA.getIndex(i) {
			bits += "x"
		} else {
			bits += "_"
		}
		if i%100 == 99 {
			lines = append(lines, bits)
			bits = ""
		}
		if i%10 == 9 {
			bits += indent
		}
		if i%50 == 49 {
			bits += indent
		}
	}
	if len(bits) > 0 {
		lines = append(lines, bits)
	}
	return fmt.Sprintf("BitArray{%v:%v}", bA.Bits, strings.Join(lines, indent))
}

// Bytes 返回 bit 数组的字节表示形式
func (bA *BitArray) Bytes() []byte {
	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	numBytes := (bA.Bits + 7) / 8
	bytes := make([]byte, numBytes)
	for i := 0; i < len(bA.Elems); i++ {
		elemBytes := [8]byte{}
		// 小端模式：地址的增长顺序与值的增长顺序相同
		binary.LittleEndian.PutUint64(elemBytes[:], bA.Elems[i])
		copy(bytes[i*8:], elemBytes[:])
	}
	return bytes
}

// Update sets the bA's bits to be that of the other bit array.
// The copying begins from the begin of both bit arrays.
func (bA *BitArray) Update(o *BitArray) {
	if bA == nil || o == nil {
		return
	}

	bA.mtx.Lock()
	o.mtx.Lock()
	copy(bA.Elems, o.Elems)
	o.mtx.Unlock()
	bA.mtx.Unlock()
}

// MarshalJSON 利用自定义的格式：“x_”来实现 json.Marshaler 接口，其中“x”表示1，“_”表示0
func (bA *BitArray) MarshalJSON() ([]byte, error) {
	if bA == nil {
		return []byte("null"), nil
	}

	bA.mtx.Lock()
	defer bA.mtx.Unlock()

	bits := `"`
	for i := 0; i < bA.Bits; i++ {
		if bA.getIndex(i) {
			bits += `x`
		} else {
			bits += `_`
		}
	}
	bits += `"`
	return []byte(bits), nil
}

var bitArrayJSONRegexp = regexp.MustCompile(`\A"([_x]*)"\z`)

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (bA *BitArray) UnmarshalJSON(bz []byte) error {
	b := string(bz)
	if b == "null" {
		bA.Bits = 0
		bA.Elems = nil
		return nil
	}

	match := bitArrayJSONRegexp.FindStringSubmatch(b)
	if match == nil {
		return fmt.Errorf("bitArray in JSON should be a string of format %q but got %s", bitArrayJSONRegexp.String(), b)
	}
	bits := match[1]

	// Construct new BitArray and copy over.
	numBits := len(bits)
	bA2 := NewBitArray(numBits)
	for i := 0; i < numBits; i++ {
		if bits[i] == 'x' {
			bA2.SetIndex(i, true)
		}
	}
	*bA = *bA2 //nolint:govet
	return nil
}

// ToProto 将 BitArray 转换为 protobuf
func (bA *BitArray) ToProto() *protobits.BitArray {
	if bA == nil || len(bA.Elems) == 0 {
		return nil
	}

	return &protobits.BitArray{
		Bits:  int64(bA.Bits),
		Elems: bA.Elems,
	}
}

// FromProto 从 protobuf 转换为 BitArray
func (bA *BitArray) FromProto(protoBitArray *protobits.BitArray) {
	if protoBitArray == nil {
		bA = nil
		return
	}

	bA.Bits = int(protoBitArray.Bits)
	if len(protoBitArray.Elems) > 0 {
		bA.Elems = protoBitArray.Elems
	}
}
