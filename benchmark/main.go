package main

import (
	"BFT/libs/log"
	httprpc "BFT/rpc/client/http"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var (
	logger            log.CRLogger
	host              = "localhost:36657"
	hostname          = "sagaya"
	tx_size           = 16
	broadcastTxMethod = "commit"
	duration          = 10
)

var rootCmd = cobra.Command{
	Use:   "bench",
	Short: "Used to test the throughput of the consensus algorithm",
}

func printErrorAndExit(reason string, err error) {
	logger.Errorw(reason, "error", err)
	os.Exit(1)
}

func main() {
	logger = log.NewCRLogger("DEBUG")

	rootCmd.Flags().StringVar(&host, "host", host, "ip:port,ip:port")
	rootCmd.Flags().StringVar(&hostname, "hostname", hostname, "host name")
	rootCmd.Flags().IntVar(&tx_size, "tx_size", tx_size, "The size of a transaction in bytes")
	rootCmd.Flags().IntVar(&duration, "duration", duration, "Benchmark test time")
	rootCmd.Flags().StringVar(&broadcastTxMethod, "broadcast_tx_method", broadcastTxMethod, "Broadcast method: [async, sync, commit]")
	err := rootCmd.Execute()

	if err != nil {
		printErrorAndExit("rootCmd executes failed", err)
	}
	if broadcastTxMethod != "async" && broadcastTxMethod != "sync" && broadcastTxMethod != "commit" {
		printErrorAndExit(fmt.Sprintf("broadcastTxMethod %q doesn't equal to 'async' or 'sync' or 'commit'", broadcastTxMethod), nil)
	}

	broadcastTxMethod = "broadcast_tx_" + broadcastTxMethod

	client, err := httprpc.New(host, "/websocket")
	if err != nil {
		printErrorAndExit("new client failed", err)
	}

	status, err := client.Status()
	if err != nil {
		printErrorAndExit("get client.Status failed", err)
	}
	latestHeight := status.SyncInfo.LatestBlockHeight // 最近的区块高度
	logger.Info("Latest block height", "height", latestHeight)

	var start time.Time
	bom := newBomber(host, broadcastTxMethod, duration)

	bom.start(&start)
	s := newStastics(client, int(latestHeight), 16, start)

	s.calc()
	s.printStatistics()
}
