#### 1. 问题一：p2p/node_info.go

DefaultNodeInfo.Other 是干嘛的？

#### 2. 问题二：p2p/switch.go

Switch.unconditionalPeerIDs 是干嘛的？

#### 3. 问题三：p2p/pex/pex_reactor.go

在 Reactor.Receive() 里，为什么种子节点给一个节点发送一些地址后，要把这个节点给删除掉

#### 4. 问题四：abci/example/kvstore/kvstore.go

在 Application.Commit() 方法里，resp.RetainHeight = app.state.Height - app.RetainBlocks + 1 这个地方没看懂是干嘛的

#### 5. 问题五：abci/example/kvstore/kvstore.go

为什么使用 “tendermint node --proxy_node=kvstore” 运行实例，然后通过 “curl -s 'localhost:26657/abci_query?data="abcd"'” 命令查询交易时，没有调用
Application.Query() 方法

#### 6. 问题六：types/validator.go

ValidatorSet 的 RescalePriorities() 方法中，diff 可能大于 diffMax 吗？

#### 7. 问题七：mempool/mempool.go

Mempool 的 InitWAL() error 方法中，WAL 是什么东西？

#### 8. 问题八：rpc/jsonrpc/types/types.go

jsonrpcid 是个什么东西？

#### 9. 问题九：prival/file.go

这个文件里的 FilePVLastSignState 结构体究竟有什么作用！

#### 10. 问题十：consensus/state.go

HSM是什么