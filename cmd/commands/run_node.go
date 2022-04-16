package commands

import (
	"fmt"
	"github.com/spf13/cobra"

	sros "github.com/232425wxy/BFT/libs/os"
	nm "github.com/232425wxy/BFT/node"
)

// AddNodeFlags 在命令行中暴露了一些常见的配置选项，这些是为了方便嵌入SRBFT节点的命令而公开的：
//	- name : variable receiver : usage
//	- "moniker" : config.Moniker : "节点名字"
//	- "priv_validator_laddr" : config.PrivValidatorListenAddr : "套接字地址，用于监听来自外部priv_validator进程的连接"
//	- "fast_sync" : config.FastSyncMode : "区块链快速同步"
//	- "consensus.double_sign_check_height" : config.Consensus.DoubleSignCheckHeight : "在加入共识之前，需要回顾多少块来检查节点的共识投票是否存在"
//	- "proxy_app" : config.ProxyApp : "代理应用的地址，或者以下几种代理应用："
//	   "kvstore" "persistent_kvstore" "counter" "counter_serial"
//	- "rpc.laddr" : config.RPC.ListenAddress : "RPC 的监听地址，例如：127.0.0.1:26657"
//	- "rpc.unsafe" : config.RPC.Unsafe : "启用不安全RPC方法"
func AddNodeFlags(cmd *cobra.Command) {

	// priv val flags
	cmd.Flags().String(
		"priv_validator_laddr",
		config.PrivValidatorListenAddr,
		"socket address to listen on for connections from external priv_validator process")

	// node flags
	cmd.Flags().Bool("fast_sync", config.FastSyncMode, "fast blockchain syncing")

	// abci flags
	cmd.Flags().String(
		"proxy_app",
		config.ProxyApp,
		"proxy app address, or one of: 'kvstore',"+
			" 'persistent_kvstore',"+
			" 'counter',"+
			" 'counter_serial' or 'noop' for local testing.")

	// rpc flags
	cmd.Flags().String("rpc.laddr", config.RPC.ListenAddress, "RPC listen address. Port required")

	// p2p flags
	cmd.Flags().String(
		"p2p.laddr",
		config.P2P.ListenAddress,
		"node listen address. (0.0.0.0:0 means any interface, any port)")
	cmd.Flags().String("p2p.persistent_peers", config.P2P.PersistentPeers, "comma-delimited ID@host:port persistent peers")

	// consensus flags
	cmd.Flags().Bool(
		"consensus.create_empty_blocks",
		config.Consensus.CreateEmptyBlocks,
		"set this to false to only produce blocks when there are txs or when the AppHash changes")
	cmd.Flags().String(
		"consensus.create_empty_blocks_interval", config.Consensus.CreateEmptyBlocksInterval.String(), "the possible interval between empty blocks")

	// db flags
	cmd.Flags().String(
		"db_backend",
		config.DBBackend,
		"database backend: goleveldb | memdb")
	cmd.Flags().String(
		"db_dir",
		config.DBPath,
		"database directory")
	cmd.Flags().Bool("consensus.evaluation", config.Consensus.Evaluation, "Decide whether to conduct a credit assessment")
}

// NewRunNodeCmd returns the command that allows the CLI to start a node.
// It can be used with a custom PrivValidator and in-process ABCI application.
func NewRunNodeCmd(nodeProvider nm.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"node", "run"},
		Short:   "Run the SRBFT node",
		RunE: func(cmd *cobra.Command, args []string) error {

			n, err := nodeProvider(config, logger)
			if err != nil {
				return fmt.Errorf("failed to create node: %w", err)
			}

			if err := n.Start(); err != nil {
				return fmt.Errorf("failed to start node: %w", err)
			}

			logger.Info("Started node", "nodeInfo", n.Switch().NodeInfo())

			// Stop upon receiving SIGTERM or CTRL-C.
			sros.TrapSignal(logger, func() {
				if n.IsRunning() {
					if err := n.Stop(); err != nil {
						logger.Error("unable to stop the node", "error", err)
					}
				}
			})

			// 阻塞在这里
			select {}
		},
	}

	AddNodeFlags(cmd)
	return cmd
}
