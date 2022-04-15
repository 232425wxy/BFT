package json

import (
	"errors"
	"reflect"
	"sync"
)

var (
	// typeRegistry contains globally registered types for JSON encoding/decoding.
	typeRegistry = newTypes()
)

// RegisterType registers a type for Amino-compatible interface encoding in the global type
// registry. These types will be encoded with a type wrapper `{"type":"<type>","value":<value>}`
// regardless of which interface they are wrapped in (if any). If the type is a pointer, it will
// still be valid both for value and pointer types, but decoding into an interface will generate
// the a value or pointer based on the registered type.
//
// Should only be called in init() functions, as it panics on error.
func RegisterType(_type interface{}, name string) {
	if _type == nil {
		panic("cannot register nil type")
	}
	err := typeRegistry.register(name, reflect.TypeOf(_type))
	if err != nil {
		panic(err)
	}
}

// typeInfo 包含 type 信息
type typeInfo struct {
	name      string
	rt        reflect.Type
	returnPtr bool
}

// types 是一个 type 注册服务组件
type types struct {
	sync.RWMutex
	byType map[reflect.Type]*typeInfo // 由变量的 type 指向 *typeInfo
	byName map[string]*typeInfo       // 由变量名指向 *typeInfo
}

// newTypes 创建一个新的 type 注册组件
func newTypes() *types {
	return &types{
		byType: map[reflect.Type]*typeInfo{},
		byName: map[string]*typeInfo{},
	}
}

// register 注册一个给定的还未注册过的变量名和其对应的 reflect.Type
func (t *types) register(name string, rt reflect.Type) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	returnPtr := false
	for rt.Kind() == reflect.Ptr { // 如果注册的变量是一个指针类型的，那么就不停地递归调用 .Elem()方法，获取该变量的实际值
		returnPtr = true
		rt = rt.Elem()
	}
	tInfo := &typeInfo{
		name:      name,
		rt:        rt,
		returnPtr: returnPtr,
	}

	t.Lock()
	defer t.Unlock()
	t.byName[name] = tInfo
	t.byType[rt] = tInfo
	return nil
}

// lookup 用变量名去查找类型，如果未注册则返回 nil
func (t *types) lookup(name string) (reflect.Type, bool) {
	t.RLock()
	defer t.RUnlock()
	tInfo := t.byName[name]
	if tInfo == nil {
		return nil, false
	}
	return tInfo.rt, tInfo.returnPtr
}

// name 查找类型的名称，如果未注册则为空。必要时打开指针。
func (t *types) name(rt reflect.Type) string {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	t.RLock()
	defer t.RUnlock()
	tInfo := t.byType[rt]
	if tInfo == nil {
		return ""
	}
	return tInfo.name
}
