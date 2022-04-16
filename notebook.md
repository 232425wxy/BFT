## protobuf

### 关键字的用法

#### gogoproto.customname

    customname 是怎么用的呢，先看下面的代码案例：

```protobuf
message NetAddress {
  string id = 1 [(gogoproto.customname) = "ID"];
  string ip = 2 [(gogoproto.customname) = "IP"];
  uint32 port = 3;
}
```

    如果我们将上面的 protobuf 代码生成 go 语言，会得到下面用 go 语言定义的结构体：

```go
type NetAddress struct {
	ID                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	IP                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 uint32   `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}
```

    那假如我们把 gogoproto.customname 去掉，重写 protobuf 代码：

```protobuf
message NetAddress {
  string id = 1;
  string ip = 2;
  uint32 port = 3;
}
```

    再次生成 go 代码，会得到下面的结果：

```go
type NetAddress struct {
	Id                   string   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Ip                   string   `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`
	Port                 uint32   `protobuf:"varint,3,opt,name=port,proto3" json:"port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}
```

    可以发现，唯一的区别是，加上 gogoproto.customname 后，我们可以自定义go 语言结构体中的字段名，如果不加，则会由 protobuf 自动为我们指定结构体字段名

#### gogoproto.nullable

    看代码案例：

```protobuf
message DefaultNodeInfo {
  ProtocolVersion      protocol_version = 1 [(gogoproto.nullable) = false];
  string               default_node_id = 2 [(gogoproto.customname) = "DefaultNodeID"];
  string               listen_addr = 3;
  string               network = 4;
  string               version = 5;
  bytes                channels = 6;
  string               moniker = 7;
  DefaultNodeInfoOther other = 8 [(gogoproto.nullable) = false];
}

message DefaultNodeInfoOther {
  string tx_index = 1;
  string rpc_address = 2 [(gogoproto.customname) = "RPCAddress"];
}
```

    生成 go 语言代码后：

```go
type DefaultNodeInfo struct {
	ProtocolVersion      ProtocolVersion      `protobuf:"bytes,1,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version"`
	DefaultNodeID        string               `protobuf:"bytes,2,opt,name=default_node_id,json=defaultNodeId,proto3" json:"default_node_id,omitempty"`
	ListenAddr           string               `protobuf:"bytes,3,opt,name=listen_addr,json=listenAddr,proto3" json:"listen_addr,omitempty"`
	Network              string               `protobuf:"bytes,4,opt,name=network,proto3" json:"network,omitempty"`
	Version              string               `protobuf:"bytes,5,opt,name=version,proto3" json:"version,omitempty"`
	Channels             []byte               `protobuf:"bytes,6,opt,name=channels,proto3" json:"channels,omitempty"`
	Moniker              string               `protobuf:"bytes,7,opt,name=moniker,proto3" json:"moniker,omitempty"`
	Other                DefaultNodeInfoOther `protobuf:"bytes,8,opt,name=other,proto3" json:"other"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

type DefaultNodeInfoOther struct {
TxIndex              string   `protobuf:"bytes,1,opt,name=tx_index,json=txIndex,proto3" json:"tx_index,omitempty"`
RPCAddress           string   `protobuf:"bytes,2,opt,name=rpc_address,json=rpcAddress,proto3" json:"rpc_address,omitempty"`
XXX_NoUnkeyedLiteral struct{} `json:"-"`
XXX_unrecognized     []byte   `json:"-"`
XXX_sizecache        int32    `json:"-"`
}
```

    我们去掉 gogoproto.nullable 后，看看有什么改变：

```go
type DefaultNodeInfo struct {
	ProtocolVersion      ProtocolVersion       `protobuf:"bytes,1,opt,name=protocol_version,json=protocolVersion,proto3" json:"protocol_version"`
	DefaultNodeID        string                `protobuf:"bytes,2,opt,name=default_node_id,json=defaultNodeId,proto3" json:"default_node_id,omitempty"`
	ListenAddr           string                `protobuf:"bytes,3,opt,name=listen_addr,json=listenAddr,proto3" json:"listen_addr,omitempty"`
	Network              string                `protobuf:"bytes,4,opt,name=network,proto3" json:"network,omitempty"`
	Version              string                `protobuf:"bytes,5,opt,name=version,proto3" json:"version,omitempty"`
	Channels             []byte                `protobuf:"bytes,6,opt,name=channels,proto3" json:"channels,omitempty"`
	Moniker              string                `protobuf:"bytes,7,opt,name=moniker,proto3" json:"moniker,omitempty"`
	Other                *DefaultNodeInfoOther `protobuf:"bytes,8,opt,name=other,proto3" json:"other,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

type DefaultNodeInfoOther struct {
    TxIndex              string   `protobuf:"bytes,1,opt,name=tx_index,json=txIndex,proto3" json:"tx_index,omitempty"`
    RPCAddress           string   `protobuf:"bytes,2,opt,name=rpc_address,json=rpcAddress,proto3" json:"rpc_address,omitempty"`
    XXX_NoUnkeyedLiteral struct{} `json:"-"`
    XXX_unrecognized     []byte   `json:"-"`
    XXX_sizecache        int32    `json:"-"`
}
```

    发现去掉 gogoproto.nullable=false 之后，DefaultNodeInfo 的 Other 字段变成引用类型了，但是如果加上 gogoproto.nullable=false，则 Other 字段会转变为值类型。

## 基于相似度加权推荐的信任模型（怎么实施）

### 对模棱两可的节点的惩罚

在 abci/example/persistent_kvstore.go 里，PersistentKVStoreApplication.BeginBlock() 方法对于那些模棱两可的验证器进行了惩罚，惩罚措施是减小它们的投票权