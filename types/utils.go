package types

import (
	"math"
	"reflect"
)

// isTypedNil 如果 o 是引用类型的，那么判断它是不是 nil 的，
// 如果是则返回 true，否则返回 false；如果 o 都不是引用类型
// 的，直接返回 false。
func isTypedNil(o interface{}) bool {
	rv := reflect.ValueOf(o)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// isEmpty 先判断 o 是否是可以调用 Len() 函数计算长度的类型的，
// 如果是则判断它们的长度是否等于 0，如果等于 0，则返回 true，
// 否则返回 false；如果不可以调用 Len() 函数计算 o 的长度，则直
// 接返回 false。
func isEmpty(o interface{}) bool {
	rv := reflect.ValueOf(o)
	switch rv.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	default:
		return false
	}
}

func diffHash(a, b []byte) float64 {
	if len(a) != 32 || len(b) != 32 {
		panic("the length of two hash value are not equal to 32")
	}
	var diff float64
	for i := 0; i < 32; i++ {
		diff += math.Abs(float64(a[i]-b[i]))
	}
	return diff
}