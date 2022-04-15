package server

import (
	srjson "BFT/libs/json"
	"BFT/libs/log"
	"BFT/rpc/jsonrpc/types"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
)

// HTTP + JSON handler

// Jsonrpc 调用获取给定方法的函数信息并运行 reflect.Call
func makeJSONRPCHandler(funcMap map[string]*RPCFunc, logger log.CRLogger) http.HandlerFunc {
	// http.HandlerFunc -> func(ResponseWriter, *Request)
	return func(w http.ResponseWriter, r *http.Request) {
		// 读取请求体
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			res := types.RPCInvalidRequestError(nil,
				fmt.Errorf("error reading request body: %w", err),
			)
			if wErr := WriteRPCResponseHTTPError(w, http.StatusBadRequest, res); wErr != nil {
				logger.Errorw("failed to write response", "res", res, "err", wErr)
			}
			return
		}

		// 如果它是一个空请求，那么就将一切能展示的函数接口放到网页里展示出来
		if len(b) == 0 {
			writeListOfEndpoints(w, r, funcMap)
			return
		}

		// first try to unmarshal the incoming request as an array of RPC requests
		var (
			requests  []types.RPCRequest
			responses []types.RPCResponse
		)
		// 先将请求体内容反序列化到 []types.RPCRequest 里，如果反序列化不成功，则尝试将
		// 请求体里的内容反序列化到一个 types.RPCRequest 里
		if err := json.Unmarshal(b, &requests); err != nil {
			var request types.RPCRequest
			if err := json.Unmarshal(b, &request); err != nil {
				res := types.RPCParseError(fmt.Errorf("error unmarshaling request: %w", err))
				if wErr := WriteRPCResponseHTTPError(w, http.StatusInternalServerError, res); wErr != nil {
					logger.Errorw("failed to write response", "res", res, "err", wErr)
				}
				return
			}
			requests = []types.RPCRequest{request}
		}
		for _, request := range requests {
			req := request

			// 如果请求的 ID 字段是 nil 的，则跳过该请求，不会回复该请求
			if req.ID == nil {
				logger.Debugw(
					"HTTPJSONRPC received a notification, skipping... (please send a non-empty ID if you want to call a method)",
					"req", req,
				)
				continue
			}
			if len(r.URL.Path) > 1 { // 因为 r.URL.Path = "/"，所以 len(r.URL.Path) = 1
				responses = append(
					responses,
					types.RPCInvalidRequestError(req.ID, fmt.Errorf("path %s is invalid", r.URL.Path)),
				)
				continue
			}

			rpcFunc, ok := funcMap[req.Method]
			// 如果对应方法的函数不存在，或者函数是 websocket 形式的，则表示发生了错误
			if !ok || rpcFunc.ws {
				responses = append(responses, types.RPCMethodNotFoundError(req.ID))
				continue
			}
			ctx := &types.Context{JSONReq: &req, HTTPReq: r}
			args := []reflect.Value{reflect.ValueOf(ctx)}
			if len(req.Params) > 0 {
				fnArgs, err := jsonParamsToArgs(rpcFunc, req.Params)
				if err != nil {
					responses = append(
						responses,
						types.RPCInvalidParamsError(req.ID, fmt.Errorf("error converting json params to arguments: %w", err)),
					)
					continue
				}
				args = append(args, fnArgs...)
			}

			returns := rpcFunc.f.Call(args)
			result, err := unreflectResult(returns)
			if err != nil {
				responses = append(responses, types.RPCInternalError(req.ID, err))
				continue
			}
			responses = append(responses, types.NewRPCSuccessResponse(req.ID, result))
		}

		if len(responses) > 0 {
			if wErr := WriteRPCResponseHTTP(w, responses...); wErr != nil {
				logger.Errorw("failed to write responses", "res", responses, "err", wErr)
			}
		}
	}
}

func handleInvalidJSONRPCPaths(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 因为模式 “/” 匹配所有与其他已注册模式不匹配的路径，所以我们检查路径是否确实是 “/”，否则返回 404 错误
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		next(w, r)
	}
}

func mapParamsToArgs(
	rpcFunc *RPCFunc,
	params map[string]json.RawMessage,
	argsOffset int,
) ([]reflect.Value, error) {

	values := make([]reflect.Value, len(rpcFunc.argNames))
	for i, argName := range rpcFunc.argNames {
		argType := rpcFunc.args[i+argsOffset]

		if p, ok := params[argName]; ok && p != nil && len(p) > 0 {
			val := reflect.New(argType) // 参数 p 的类型
			err := srjson.Unmarshal(p, val.Interface())
			if err != nil {
				return nil, err
			}
			values[i] = val.Elem()
		} else { // use default for that type
			values[i] = reflect.Zero(argType)
		}
	}

	return values, nil
}

func arrayParamsToArgs(
	rpcFunc *RPCFunc,
	params []json.RawMessage,
	argsOffset int,
) ([]reflect.Value, error) {

	if len(rpcFunc.argNames) != len(params) {
		return nil, fmt.Errorf("expected %v parameters (%v), got %v (%v)",
			len(rpcFunc.argNames), rpcFunc.argNames, len(params), params)
	}

	values := make([]reflect.Value, len(params))
	for i, p := range params {
		argType := rpcFunc.args[i+argsOffset]
		val := reflect.New(argType)
		err := srjson.Unmarshal(p, val.Interface())
		if err != nil {
			return nil, err
		}
		values[i] = val.Elem()
	}
	return values, nil
}

// 传入的参数有两个，一个是 rpcFunc，另一个是 raw，raw 里存储了 rpcFunc 的输入参数的原始信息，
// jsonParamsToArgs 的目的是将 raw 里存储的原始信息转换为可被 rpcFunc.f 直接调用的入参列表
func jsonParamsToArgs(rpcFunc *RPCFunc, raw []byte) ([]reflect.Value, error) {
	const argsOffset = 1

	var m map[string]json.RawMessage
	// 将 raw 反序列化到一个 map[string]json.RawMessage 里
	err := json.Unmarshal(raw, &m)
	if err == nil {
		// 如果反序列化成功，说明 raw 里的数据格式是 {key:value}
		return mapParamsToArgs(rpcFunc, m, argsOffset)
	}

	// 将 raw 反序列化到 []json.RawMessage 里
	var a []json.RawMessage
	err = json.Unmarshal(raw, &a)
	if err == nil {
		// 如果反序列化成功，说明 raw 里的数据格式是 [p1, p2, p3]
		return arrayParamsToArgs(rpcFunc, a, argsOffset)
	}

	// Otherwise, bad format, we cannot parse
	return nil, fmt.Errorf("unknown type for JSON params: %v. Expected map or array", err)
}

// 将所有可被调用的函数接口放到网页上展示出来
func writeListOfEndpoints(w http.ResponseWriter, r *http.Request, funcMap map[string]*RPCFunc) {
	var noArgNames []string // 无需入参即可被调用的函数
	var argNames []string
	for name, funcData := range funcMap {
		if len(funcData.args) == 1 && funcData.args[0].String() == "*types.Context" {
			noArgNames = append(noArgNames, name)
		} else {
			argNames = append(argNames, name)
		}
	}
	sort.Strings(noArgNames)
	sort.Strings(argNames)
	buf := new(bytes.Buffer) // 用来存放网页的 body 的
	buf.WriteString("<html><body>")
	buf.WriteString("<br>Functions that can be called without input arguments:<br>")

	for _, name := range noArgNames {
		link := fmt.Sprintf("//%s/%s", r.Host, name) // r.Host = localhost:16656
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}

	buf.WriteString("<br>Functions that require input arguments to be called:<br>")
	for _, name := range argNames {
		link := fmt.Sprintf("//%s/%s?", r.Host, name)
		funcData := funcMap[name]
		for i, argName := range funcData.argNames {
			link += argName + "=_"
			if i < len(funcData.argNames)-1 {
				link += "&"
			}
		}
		// example: link = //localhost:16656/abci_query?path=_&data=_&height=_&prove=_
		buf.WriteString(fmt.Sprintf("<a href=\"%s\">%s</a></br>", link, link))
	}
	buf.WriteString("</body></html>")
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	_, _ = w.Write(buf.Bytes()) // 写入到响应 body 中
}
