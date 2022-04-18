package server

import (
	"encoding/hex"
	"fmt"
	srjson "github.com/232425wxy/BFT/libs/json"
	"github.com/232425wxy/BFT/libs/log"
	"github.com/232425wxy/BFT/rpc/jsonrpc/types"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

// HTTP + URI handler

var reInt = regexp.MustCompile(`^-?[0-9]+$`)

// convert from a function name to the http handler
func makeHTTPHandler(rpcFunc *RPCFunc, logger log.CRLogger) func(http.ResponseWriter, *http.Request) {
	// Always return -1 as there's no ID here.
	dummyID := types.JSONRPCIntID(-1) // URIClientRequestID

	// 对于 websocket 的终端调用，则直接在 response 中写入一个错误
	if rpcFunc.ws {
		return func(w http.ResponseWriter, r *http.Request) {
			res := types.RPCMethodNotFoundError(dummyID)
			if wErr := WriteRPCResponseHTTPError(w, http.StatusNotFound, res); wErr != nil {
				logger.Warnw("failed to write response", "res", res, "err", wErr)
			}
		}
	}

	// All other endpoints
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Debugw("HTTP HANDLER", "req", r)

		ctx := &types.Context{HTTPReq: r}
		args := []reflect.Value{reflect.ValueOf(ctx)}

		fnArgs, err := httpParamsToArgs(rpcFunc, r)
		if err != nil {
			res := types.RPCInvalidParamsError(dummyID,
				fmt.Errorf("error converting http params to arguments: %w", err),
			)
			if wErr := WriteRPCResponseHTTPError(w, http.StatusInternalServerError, res); wErr != nil {
				logger.Warnw("failed to write response", "res", res, "err", wErr)
			}
			return
		}
		args = append(args, fnArgs...)

		returns := rpcFunc.f.Call(args)

		logger.Debugw("HTTPRestRPC", "method", r.URL.Path, "args", args, "returns", returns)
		result, err := unreflectResult(returns)
		if err != nil {
			if err := WriteRPCResponseHTTPError(w, http.StatusInternalServerError,
				types.RPCInternalError(dummyID, err)); err != nil {
				logger.Warnw("failed to write response", "res", result, "err", err)
				return
			}
			return
		}
		if err := WriteRPCResponseHTTP(w, types.NewRPCSuccessResponse(dummyID, result)); err != nil {
			logger.Warnw("failed to write response", "res", result, "err", err)
			return
		}
	}
}

// 将请求 request 的路径中或者表单中携带的字符串类型参数转换为 RPCFunc 的入参类型并返回
func httpParamsToArgs(rpcFunc *RPCFunc, r *http.Request) ([]reflect.Value, error) {
	// skip types.Context
	const argsOffset = 1

	values := make([]reflect.Value, len(rpcFunc.argNames))

	for i, name := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		values[i] = reflect.Zero(argType) // set default for that type

		arg := getParam(r, name)
		// log.Notice("param to arg", "argType", argType, "name", name, "arg", arg)

		if arg == "" {
			continue
		}

		// 由于 arg 是字符串类型，因此此处将其装环卫 RPCFunc 的入参类型
		v, ok, err := nonJSONStringToArg(argType, arg)
		if err != nil {
			return nil, err
		}
		if ok {
			values[i] = v
			continue
		}

		values[i], err = jsonStringToArg(argType, arg)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func jsonStringToArg(rt reflect.Type, arg string) (reflect.Value, error) {
	rv := reflect.New(rt)
	err := srjson.Unmarshal([]byte(arg), rv.Interface())
	if err != nil {
		return rv, err
	}
	rv = rv.Elem()
	return rv, nil
}

func nonJSONStringToArg(rt reflect.Type, arg string) (reflect.Value, bool, error) {
	if rt.Kind() == reflect.Ptr { // 如果 rt 是指针类型，则递归调用，直到 rt 不是指针
		rv1, ok, err := nonJSONStringToArg(rt.Elem(), arg)
		switch {
		case err != nil:
			return reflect.Value{}, false, err
		case ok:
			rv := reflect.New(rt.Elem())
			rv.Elem().Set(rv1)
			return rv, true, nil
		default:
			return reflect.Value{}, false, nil
		}
	} else {
		return _nonJSONStringToArg(rt, arg)
	}
}

// NOTE: rt.Kind() isn't a pointer.
func _nonJSONStringToArg(rt reflect.Type, arg string) (reflect.Value, bool, error) {
	isIntString := reInt.Match([]byte(arg))
	isQuotedString := strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)
	isHexString := strings.HasPrefix(strings.ToLower(arg), "0x")

	var expectingString, expectingByteSlice, expectingInt bool
	switch rt.Kind() {
	case reflect.Int,
		reflect.Uint,
		reflect.Int8,
		reflect.Uint8,
		reflect.Int16,
		reflect.Uint16,
		reflect.Int32,
		reflect.Uint32,
		reflect.Int64,
		reflect.Uint64:
		expectingInt = true
	case reflect.String:
		expectingString = true
	case reflect.Slice:
		expectingByteSlice = rt.Elem().Kind() == reflect.Uint8
	}

	if isIntString && expectingInt {
		qarg := `"` + arg + `"`
		rv, err := jsonStringToArg(rt, qarg)
		if err != nil {
			return rv, false, err
		}

		return rv, true, nil
	}

	if isHexString {
		if !expectingString && !expectingByteSlice {
			err := fmt.Errorf("got a hex string arg, but expected '%s'",
				rt.Kind().String())
			return reflect.ValueOf(nil), false, err
		}

		var value []byte
		value, err := hex.DecodeString(arg[2:])
		if err != nil {
			return reflect.ValueOf(nil), false, err
		}
		if rt.Kind() == reflect.String {
			return reflect.ValueOf(string(value)), true, nil
		}
		return reflect.ValueOf(value), true, nil
	}

	if isQuotedString && expectingByteSlice {
		v := reflect.New(reflect.TypeOf(""))
		err := srjson.Unmarshal([]byte(arg), v.Interface())
		if err != nil {
			return reflect.ValueOf(nil), false, err
		}
		v = v.Elem()
		return reflect.ValueOf([]byte(v.String())), true, nil
	}

	return reflect.ValueOf(nil), false, nil
}

// 从请求的路径中或者请求的表单里获得对应 param 的值
func getParam(r *http.Request, param string) string {
	s := r.URL.Query().Get(param)
	if s == "" {
		s = r.FormValue(param)
	}
	return s
}
