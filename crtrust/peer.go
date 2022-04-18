package crtrust

import (
	"fmt"
	"github.com/232425wxy/BFT/crypto"
	"github.com/232425wxy/BFT/gossip"
	"math"
	"time"
)

type peer struct {
	ID gossip.ID
	Pubkey crypto.PubKey
	goods float64
	bads float64
	trust float64
	weight float64
}

// ------------------------------------------------------------------

type system struct {
	recommendations map[*peer]float64
	similarities map[string]map[string]float64
	globalT map[string]float64
	tok *time.Ticker
}

func arccot(x float64) float64 {
	return math.Pi/2 - math.Atan(x)
}

func (sys *system) evaluateRoutine() {
	for {
		select {
		case <-sys.tok.C:
			for p, _ := range sys.recommendations {
				sys.recommendations[p] = arccot(2*p.bads - p.goods)/math.Pi
			}
		}
	}
}

// 计算相似度
func (sys *system) globalTrust(collect map[string]*LocalEvaluation) Updates {
	// 1. 计算节点之间推荐意见的相似度
	for c1, local1 := range collect {
		for c2, local2 := range collect {
			if c1 == c2 {
				sys.similarities[c1][c2] = 1.0
				continue
			}
			numerator, denominator1, denominator2 := 0.0, 0.0, 0.0
			for id, _ := range local1.PeerEval {
				// 此处遍历可以得到两个节点对其他节点的推荐意见
				numerator += local1.PeerEval[id] * local2.PeerEval[id]
				denominator1 += local1.PeerEval[id]*local1.PeerEval[id]
				denominator2 += local2.PeerEval[id]*local2.PeerEval[id]
			}
			sys.similarities[c1][c2] = numerator / (math.Pow(denominator1, 0.5)*math.Pow(denominator2, 0.5))
		}
	}

	// 2. 计算每个节点的推荐权重，如果一个节点对全网其他节点的推荐意见较为一致，说明该节点是好节点的概率比较高
	for p, _ := range sys.recommendations {
		p_sim := sys.similarities[string(p.ID)]
		p.weight = 0.0
		for _, sim := range p_sim {
			p.weight += sim / float64(len(p_sim))
		}
		fmt.Printf("节点%s的信任权重是：%v\n", p.ID, p.weight)
	}

	// 3. 对全网节点的推荐意见做归一化处理
	for _, evals := range collect {
		sumi := 0.0
		for _, eval := range evals.PeerEval {
			sumi += eval
		}
		for id, _ := range evals.PeerEval {
			evals.PeerEval[id] = evals.PeerEval[id] / sumi
		}
	}

	// 4. 开始计算全局信任值
	for {
		c_T := make(map[string]float64)
		for id, T := range sys.globalT {
			c_T[id] = T
		}
		for id, _ := range sys.globalT {
			trust := 0.0
			for id2, _ := range sys.globalT {
				trust = trust + sys.globalT[id2] * sys.similarities[id][id2] * collect[id2].PeerEval[id] * sys.weight(id)
			}
			sys.globalT[id] = trust
		}
		diff := 0.0
		for id, t1 := range sys.globalT {
			diff += math.Abs(t1 - c_T[id])
		}
		if diff < 0.00000001 {
			break
		}
	}

	// 5. 获得全网节点全局信任值
	updates := make([]*Update, 0)
	sumi := 0.0
	for _, t := range sys.globalT {
		sumi += t
	}
	for id, t := range sys.globalT {
		updates = append(updates, &Update{
			Id:      id,
			Pubkey: sys.pubKey(id),
			Trust:   t / sumi,
		})
	}
	return updates
}

func (sys *system) weight(id string) float64 {
	for p, _ := range sys.recommendations {
		if string(p.ID) == id {
			return p.weight
		}
	}
	panic("not found weight!!!")
}

func (sys *system) pubKey(id string) crypto.PubKey {
	for p, _ := range sys.recommendations {
		if string(p.ID) == id {
			return p.Pubkey
		}
	}
	panic("not found Address!!!")
}