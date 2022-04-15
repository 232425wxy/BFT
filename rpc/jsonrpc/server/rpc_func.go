package server

import (
	"BFT/libs/log"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// RegisterRPCFuncs 为 funcMap 中的每个函数添加了一个路由，并为所有函数添加了通用的 jsonrpc 和 websocket 处理程序
// “result” 是注册结果对象的接口，并且用每个 RPCResponse 填充
func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc, logger log.CRLogger) {
	// HTTP 页面
	for funcName, rpcFunc := range funcMap {
		mux.HandleFunc("/"+funcName, makeHTTPHandler(rpcFunc, logger))
	}

	// 根地址
	mux.HandleFunc("/", handleInvalidJSONRPCPaths(makeJSONRPCHandler(funcMap, logger)))
}

// Function introspection

// RPCFunc 包含一个函数的内省类型信息
type RPCFunc struct {
	f       reflect.Value  // 底层的 rpc 函数
	args    []reflect.Type // 函数 f 的入参类型
	returns []reflect.Type // 函数 f 的返回参数类型
	// localhost:16656/abci_query?path=_&data=_&height=_&prove=_，argNames 就是此处链接
	// 中的 path、data、height、prove
	argNames []string
	ws       bool // websocket only
}

// NewRPCFunc 封装了一个用于内省的函数。f 是函数，args 是用逗号分隔的参数名
func NewRPCFunc(f interface{}, args string) *RPCFunc {
	return newRPCFunc(f, args, false)
}

// NewWSRPCFunc 封装了一个用于内省的函数，并在websockets中使用。
func NewWSRPCFunc(f interface{}, args string) *RPCFunc {
	return newRPCFunc(f, args, true)
}

// 实例化一个新的 RPCFunc，f 是一个函数，
func newRPCFunc(f interface{}, args string, ws bool) *RPCFunc {
	var argNames []string
	if args != "" {
		argNames = strings.Split(args, ",")
	}
	return &RPCFunc{
		f:        reflect.ValueOf(f),
		args:     funcArgTypes(f),
		returns:  funcReturnTypes(f),
		argNames: argNames,
		ws:       ws,
	}
}

// 入参 f 是一个函数，返回函数 f 的所有入参的动态反射类型 []reflect.Type
func funcArgTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	// 得到函数 f 的入参个数 n
	n := t.NumIn()
	typez := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		// In 返回函数类型的第 i 个输入参数的类型，如果 type's Kind 不是 Func，或者
		// 如果 i 不在 [0,NumIn()) 范围内，它就会 panic
		typez[i] = t.In(i)
	}
	return typez
}

// 入参 f 是一个函数，返回函数 f 的所有返回参数的动态反射类型 []reflect.Type
func funcReturnTypes(f interface{}) []reflect.Type {
	t := reflect.TypeOf(f)
	n := t.NumOut()
	typez := make([]reflect.Type, n)
	for i := 0; i < n; i++ {
		typez[i] = t.Out(i)
	}
	return typez
}

//-------------------------------------------------------------

// 入参 returns 是一个列表，里面只包含两个值，一个是 interface{} 的动态类型反射 Type，表示可以是任意值，
// 另一个是 error 的动态类型反射 Type，unreflectResult 的作用就是将函数的两个动态类型反射的返回值转换回
// 原本的值，但是如果 error 的动态类型反射不为 nil，则 unreflectResult 返回 nil, err，否则返回 non-nil, nil
func unreflectResult(returns []reflect.Value) (interface{}, error) {
	errV := returns[1]
	if errV.Interface() != nil {
		return nil, fmt.Errorf("%v", errV.Interface())
	}
	rv := returns[0]

	// New 返回一个 Value，表示指向指定类型的新零值的指针。也就是说，返回值的类型是PtrTo(typ)。
	rvp := reflect.New(rv.Type())
	// v.Set(x)
	// Set 将 x 赋值给 v，如果 CanSet 返回false，它会 panic
	// 在Go中，x 的类型必须是可以赋值给v的类型。
	rvp.Elem().Set(rv)
	return rvp.Interface(), nil
}
