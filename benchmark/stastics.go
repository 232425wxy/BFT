package main

import (
	"BFT/libs/json"
	srrpc "BFT/rpc/client/http"
	"BFT/types"
	"context"
	"fmt"
	"time"
)

type statistics struct {
	Client           *srrpc.HTTP
	startHeight      int
	startTime        time.Time
	endTime          time.Time
	txsThroughput    int
	blocksThroughput int
	interval         float64
}

func newStastics(client *srrpc.HTTP, startHeight, duration int, startTime time.Time) *statistics {
	return &statistics{
		Client:           client,
		startHeight:      startHeight,
		startTime:        startTime,
		endTime:          startTime.Add(time.Duration(duration*2) * time.Second),
		txsThroughput:    0,
		blocksThroughput: 0,
	}
}

func (s *statistics) getBlockMetas() []*types.BlockMeta {
	info, err := s.Client.BlockchainInfo(context.Background(), 4, 0)
	if err != nil {
		panic("get block meta failed")
	}

	latestHeight := info.LastHeight
	blockMetas := make([]*types.BlockMeta, 0)
	for _, bm := range info.BlockMetas {
		blockMetas = append(blockMetas, bm)
	}
	fmt.Println("---------------------------------------------------------")
	fmt.Println("LatestHeight:", latestHeight)
	fmt.Println("---------------------------------------------------------")

	for len(blockMetas) < int(latestHeight) - 4 {
		info, err = s.Client.BlockchainInfo(context.Background(), int64(4), blockMetas[len(blockMetas)-1].Header.Height-1)
		for _, bm := range info.BlockMetas {
			blockMetas = append(blockMetas, bm)
		}
	}

	if len(blockMetas) != int(latestHeight)-3 {
		panic(fmt.Sprintf("get wrong number blockMetas, expected %d actual %d", latestHeight-2, len(blockMetas)))
	}
	return blockMetas
}

func (s *statistics) calc() {
	time.Sleep(time.Second * time.Duration(duration / 50))
	blockMetas := s.getBlockMetas()

	s.interval = blockMetas[0].Header.Time.Sub(blockMetas[len(blockMetas)-1].Header.Time).Seconds()

	for _, b := range blockMetas {
		s.blocksThroughput += 1
		s.txsThroughput += b.NumTxs
	}

	s.interval -= float64(s.blocksThroughput) * 1.0

	logger.Infow("时间间隔", "interval", s.interval)
}

func (s *statistics) printStatistics() {
	result, err := json.Marshal(struct {
		TxsThroughput    float64 `json:"txs_per_sec_throughput"`
		BlocksThroughput float64 `json:"blocks_per_sec_throughput"`
		Delay float64 `json:"delay"`
	}{
		TxsThroughput:    float64(s.txsThroughput) / s.interval,
		BlocksThroughput: float64(s.blocksThroughput) / s.interval,
		Delay: float64(1)/(float64(s.blocksThroughput) / s.interval),
	})
	if err != nil {
		printErrorAndExit("marshal result failed", err)
	}

	logger.Infow(string(result), "interval", fmt.Sprintf("%.2fs", s.interval))
}