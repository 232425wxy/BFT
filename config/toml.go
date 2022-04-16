package config

import (
	"bytes"
	"path/filepath"
	"strings"
	"text/template"

	sros "github.com/232425wxy/BFT/libs/os"
)

// DefaultDirPerm is the default permissions used when creating directories.
const DefaultDirPerm = 0700

var configTemplate *template.Template

func init() {
	var err error
	tmpl := template.New("configFileTemplate").Funcs(template.FuncMap{
		"StringsJoin": strings.Join,
	})
	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

/****** these are for production settings ***********/

// EnsureRoot creates the root, config, and data directories if they don't exist,
// and panics if it fails.
func EnsureRoot(rootDir string) {
	if err := sros.EnsureDir(rootDir, DefaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := sros.EnsureDir(filepath.Join(rootDir, defaultConfigDir), DefaultDirPerm); err != nil {
		panic(err.Error())
	}
	if err := sros.EnsureDir(filepath.Join(rootDir, defaultDataDir), DefaultDirPerm); err != nil {
		panic(err.Error())
	}

	configFilePath := filepath.Join(rootDir, defaultConfigFilePath)

	// Write default config file if missing.
	if !sros.FileExists(configFilePath) {
		writeDefaultConfigFile(configFilePath)
	}
}

// XXX: this func should probably be called by cmd/github.com/232425wxy/BFT/commands/init.go
// alongside the writing of the genesis.json and priv_validator.json
func writeDefaultConfigFile(configFilePath string) {
	WriteConfigFile(configFilePath, DefaultConfig())
}

// WriteConfigFile renders config using the template and writes it to configFilePath.
func WriteConfigFile(configFilePath string, config *Config) {
	var buffer bytes.Buffer

	// 将 config 中的各个字段填充到模板的对应位置上
	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	sros.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}

const defaultConfigTemplate = `# 这是一个 TOML 配置文件.
# 要想了解更多信息，请参考：https://github.com/toml-lang/toml

# 注意: 该配置文件里的所有路径都是相对路径，相对于主目录：“$HOME/.SRBFT”（默认情况下），
# 但也可以通过环境变量 “$SRHOME” 或者命令行参数 “--home” 来改变主目录.

######################################################################
###                    (BaseConfig)  基本配置选项                    ###
######################################################################

# 区块链的 ID
chain_id = "{{ .BaseConfig.ChainID }}"

# 存放所有数据的根目录，包括配置文件、Private Validator 的公私钥、投票权以及节点用于身
# 份验证的私钥，等等一系列内容
home = "{{ .BaseConfig.RootDir }}"

# ABCI应用程序的TCP或UNIX套接字地址，或用SRBFT二进制文件编译的ABCI应用程序的名称.
proxy_app = "{{ .BaseConfig.ProxyApp }}"

# 如果此处设置为 true，那么当节点本地的区块链后面还有很多区块没有被追加上，那么允许此节点通
# 过并行下载区块并验证，来快速同步区块链以保持区块链数据与系统中其他节点的区块链一致
fast_sync = {{ .BaseConfig.FastSyncMode }}

# 数据库后端: goleveldb | cleveldb | boltdb | rocksdb | badgerdb
# 默认情况下，此处选择 “goleveldb”，该数据库由纯 go 语言实现，稳定，最出名的实现方案可在此
# 处查看：https://github.com/syndtr/goleveldb
db_backend = "{{ .BaseConfig.DBBackend }}"

# 存放数据库的目录
db_dir = "{{ js .BaseConfig.DBPath }}"

# 日志的输出级别
log_level = "{{ .BaseConfig.LogLevel }}"

##### 额外的基本配置选项 #####

# 存储初始的 validator 集合以及其他一些元数据，包括区块链ID、区块链创建时间等等
genesis_file = "{{ js .BaseConfig.Genesis }}"

# 存储 Private Validator 的地址、公钥、私钥等信息
priv_validator_key_file = "{{ js .BaseConfig.PrivValidatorKey }}"

# 存储 Private Validator 上一次签名时的状态，包括：height、round、step
priv_validator_state_file = "{{ js .BaseConfig.PrivValidatorState }}"

# SRBFT 用来监听来自外部 Private Validator 进程的 TCP 或 UNIX 套接字地址
priv_validator_laddr = "{{ .BaseConfig.PrivValidatorListenAddr }}"

# 用来存放 P2P 协议中进行身份验证的私钥
node_key_file = "{{ js .BaseConfig.NodeKey }}"


######################################################################
###                           高级配置选项                           ###
######################################################################

#######################################################
###          (RPCConfig)  RPC服务器配置选项            ###
#######################################################
[rpc]

# 用于 RPC 服务器侦听的 TCP 或 UNIX 套接字地址
listen_addr = "{{ .RPC.ListenAddress }}"

# 最大并发连接数(包括 WebSocket)，该值的设定需要考虑到操作系统的限制.
# 最大并发连接数应该小于 {ulimit -Sn} - {MaxNumInboundPeers} - {MaxNumOutboundPeers} - {N of wal, db and other open files}
# 1024 - 40 - 10 - 50 = 924 = ~900
max_open_connections = {{ .RPC.MaxOpenConnections }}

# Maximum number of unique clientIDs that can /subscribe
# If you're using /broadcast_tx_commit, set to the estimated maximum number
# of broadcast_tx_commit calls per block.
max_subscription_clients = {{ .RPC.MaxSubscriptionClients }}

# Maximum number of unique queries a given client can /subscribe to
max_subscriptions_per_client = {{ .RPC.MaxSubscriptionsPerClient }}

# 在 /broadcast_tx_commit 过程中等待tx提交的时间.
# 警告:使用大于 10s 的值将导致全局 HTTP 写超时增加，这适用于所有连接和端点.
timeout_broadcast_tx_commit = "{{ .RPC.TimeoutBroadcastTxCommit }}"

# 请求体的最大大小，以字节为单位
max_body_bytes = {{ .RPC.MaxBodyBytes }}

# 请求头的最大大小，以字节为单位
max_header_bytes = {{ .RPC.MaxHeaderBytes }}


#######################################################
###           (P2PConfig)  P2P协议配置选项            ###
#######################################################
[p2p]

# 监听新连接的地址
listen_addr = "{{ .P2P.ListenAddress }}"

# 与自己保持长久连接的节点，这里用字符串来表示，而不是用列表，节点之间用逗号隔开
persistent_peers = "{{ .P2P.PersistentPeers }}"

# 存放地址簿的路径
addr_book_file = "{{ js .P2P.AddrBook }}"

# 对于私有地址或本地地址，这里应当设为 false，如果不是私有地址或本地地址，那么我们必须要保证地址
# 是可被路由的，因此应当设为 true.
addr_book_strict = {{ .P2P.AddrBookStrict }}

# 最大的入站节点数量，所谓入站节点是指主动与自己建立连接的节点
max_num_inbound_peers = {{ .P2P.MaxNumInboundPeers }}

# 最大的出站节点数量，不包括与其保持持久连接的节点，所谓出站节点是指自己主动建立连接的节点
max_num_outbound_peers = {{ .P2P.MaxNumOutboundPeers }}

# 刷新连接上的消息之间的等待时间
flush_throttle_timeout = "{{ .P2P.FlushThrottleTimeout }}"

# 数据包有效负载的最大大小，以字节为单位
max_packet_msg_payload_size = {{ .P2P.MaxPacketMsgPayloadSize }}

# 建立连接时的握手超时时间
handshake_timeout = "{{ .P2P.HandshakeTimeout }}"

# 建立连接时拨号的超时时间
dial_timeout = "{{ .P2P.DialTimeout }}"


#######################################################
###          (MempoolConfig)  内存池配置选项          ###
#######################################################
[mempool]

recheck = {{ .Mempool.Recheck }}
broadcast = {{ .Mempool.Broadcast }}
wal_dir = "{{ js .Mempool.WalPath }}"

# Maximum number of transactions in the mempool
size = {{ .Mempool.Size }}

# Limit the total size of all txs in the mempool.
# This only accounts for raw transactions (e.g. given 1MB transactions and
# max_txs_bytes=5MB, mempool will only accept 5 transactions).
max_txs_bytes = {{ .Mempool.MaxTxsBytes }}

# Size of the cache (used to filter transactions we saw earlier) in transactions
cache_size = {{ .Mempool.CacheSize }}

# Maximum size of a single transaction.
# NOTE: the max size of a tx transmitted over the network is {max_tx_bytes}.
max_tx_bytes = {{ .Mempool.MaxTxBytes }}

#######################################################
###         Consensus Configuration Options         ###
#######################################################
[consensus]

wal_file = "{{ js .Consensus.WalPath }}"

# How long we wait for a proposal block before prevoting nil
timeout_propose = "{{ .Consensus.TimeoutPropose }}"
# How much timeout_propose increases with each round
timeout_propose_delta = "{{ .Consensus.TimeoutProposeDelta }}"
# How long we wait after receiving +2/3 prevotes for “anything” (ie. not a single block or nil)
timeout_prevote = "{{ .Consensus.TimeoutPrevote }}"
# How much the timeout_prevote increases with each round
timeout_prevote_delta = "{{ .Consensus.TimeoutPrevoteDelta }}"
# How long we wait after receiving +2/3 precommits for “anything” (ie. not a single block or nil)
timeout_precommit = "{{ .Consensus.TimeoutPrecommit }}"
# How much the timeout_precommit increases with each round
timeout_precommit_delta = "{{ .Consensus.TimeoutPrecommitDelta }}"
# How long we wait after committing a block, before starting on the new
# height (this gives us a chance to receive some more precommits, even
# though we already have +2/3).
timeout_commit = "{{ .Consensus.TimeoutCommit }}"

# Make progress as soon as we have all the precommits (as if TimeoutCommit = 0)
skip_timeout_commit = {{ .Consensus.SkipTimeoutCommit }}

# EmptyBlocks mode and possible interval between empty blocks
create_empty_blocks = {{ .Consensus.CreateEmptyBlocks }}
create_empty_blocks_interval = "{{ .Consensus.CreateEmptyBlocksInterval }}"

# Reactor sleep duration parameters
peer_gossip_sleep_duration = "{{ .Consensus.PeerGossipSleepDuration }}"
peer_query_maj23_sleep_duration = "{{ .Consensus.PeerQueryMaj23SleepDuration }}"

# 拜占庭节点所占比例
byzantine_ratio = {{ .Consensus.TestByzantineRatio }}

# 投票签名错误的概率
prob_wrong_sign = {{ .Consensus.ProbWrongSign }}

# true 则表示自己是拜占庭节点，用于论文测试
is_byzantine = {{ .Consensus.IsByzantine }}

evaluation = {{ .Consensus.Evaluation }}

tolerance_delay = {{ .Consensus.ToleranceDelay }}

#######################################################
###   Transaction Indexer Configuration Options     ###
#######################################################
[tx_index]

# What indexer to use for transactions
#
# The application will set which txs to index. In some cases a node operator will be able
# to decide which txs to index based on configuration set in the application.
#
# Options:
#   1) "null"
#   2) "kv" (default) - the simplest possible indexer, backed by key-value storage (defaults to levelDB; see DBBackend).
# 		- When "kv" is chosen "tx.height" and "tx.hash" will always be indexed.
indexer = "{{ .TxIndex.Indexer }}"
`