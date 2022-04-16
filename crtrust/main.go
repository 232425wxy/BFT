package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

var chars = []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

type node struct {
	similarities map[*node]float64  // 和其他节点之间的相似度
	recomds map[*node]float64  // 对其他节点的推荐意见
	_type string  // 节点类型
	trust float64
	id [10]byte
	weight float64
}

type system struct {
	nodes []*node
	totalNodeNum int
	R [][]float64
	T []float64
	threshold float64
}


// --------------------------------------------------------------------------- 工具函数

// 产生100个局部评价评价
func evaluate(_type string) float64 {
	var start, end float64
	switch _type {
	case "h2h", "m2m":
		start, end = 0.7, 0.99
	case "h2m", "m2h":
		start, end = 0.3, 0.4
	}
	precious := (end - start) / float64(100)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return float64(r.Intn(100)) * precious + start
}

// 获得推荐意见的秩
func rank(evaluations map[*node]float64) map[*node]int {
	internal := func(evaluation float64, n *node) int {
		rank := 1
		for n2, eval := range evaluations {
			if evaluation > eval {
				rank++
			} else if evaluation == eval {
				if compareID(n.id, n2.id) {
					rank++
				}
			}
		}
		return rank
	}
	result := make(map[*node]int)
	for n , eval := range evaluations {
		result[n] = internal(eval, n)
	}
	return result
}

func similar(recomds1, recomds2 map[*node]int) float64 {
	if len(recomds1) != len(recomds2) {
		panic("推荐意见长度不一致")
	}
	n := len(recomds1)
	diff := 0.0
	for n, _ := range recomds1 {
		diff = diff + math.Pow(float64(recomds1[n]-recomds2[n]), 2.0)
	}
	return (2.0 - 6.0 * diff / (math.Pow(float64(n), 3.0) - float64(n))) / 2.0
}

// 四舍五入函数
func sishewuru(f float64) int {
	xs := f - math.Floor(f)
	if xs >= 0.5 {
		return int(math.Ceil(f))
	} else {
		return int(math.Floor(f))
	}
}

// 为节点生成id
func generateID() [10]byte {
	id := [10]byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		id[i] = chars[r.Intn(len(chars))]
	}
	return id
}

// 比较两个id大小
func compareID(id1, id2 [10]byte) bool {
	for i := 0; i <len(id1); i++ {
		if id1[i] == id2[i] {
			continue
		} else if id1[i] > id2[i] {
			return true
		} else {
			return false
		}
	}
	return false
}

func avg(nodes []*node) {
	honest := 0.0
	h := 1.0
	normalMalicious := 0.0
	nm := 1.0
	cheatMalicious := 0.0
	cm := 1.0
	spyMalicious := 0.0
	sm := 1.0
	for _, n := range nodes {
		if n._type == "honest" {
			honest += n.trust
			h += 1.0
		} else if n._type == "normalMalicious" {
			normalMalicious += n.trust
			nm += 1.0
		} else if n._type == "cheatMalicious" {
			cheatMalicious += n.trust
			cm += 1.0
		} else if n._type == "spyMalicious" {
			spyMalicious += n.trust
			sm += 1.0
		}
	}
	sum := honest / h + normalMalicious / nm + cheatMalicious / cm + spyMalicious / sm
	fmt.Printf("Good：%.8f \n IM：%.8f \n MC：%.8f \n MS：%.8f \n", honest / (h*sum), normalMalicious / (nm*sum), cheatMalicious / (cm*sum), spyMalicious / (sm*sum))
	fmt.Println("-----------------------------------------------------")
}

// ---------------------------------------------------------------------------

func setup(totalNodeNum int, e, e1, e2, e3 float64, threshold float64) {
	sys := new(system)
	honestNum := totalNodeNum - sishewuru(e * float64(totalNodeNum))
	normalMaliciousNum := sishewuru(e * e1 * float64(totalNodeNum))
	cheatMaliciousNum := sishewuru(e * e2 * float64(totalNodeNum))
	spyMaliciousNum := sishewuru(e * e3 * float64(totalNodeNum))
	fmt.Printf("产生了%d个正常节点 %d个普通恶意节点 %d个合谋恶意节点 %d个恶意间谍节点，节点总数为%d\n", honestNum, normalMaliciousNum, cheatMaliciousNum, spyMaliciousNum, honestNum+normalMaliciousNum+cheatMaliciousNum+spyMaliciousNum)

	sys.nodes = make([]*node, totalNodeNum)
	sys.totalNodeNum = totalNodeNum
	sys.threshold = threshold

	for i := 0; i < honestNum; i++ {
		n := &node{
			id: generateID(),
			similarities: make(map[*node]float64),
			recomds: make(map[*node]float64),
			_type: "honest",
			trust: 1.0,
			weight: 0.0,
		}
		sys.nodes[i] = n
	}

	for i := honestNum; i < totalNodeNum - cheatMaliciousNum - spyMaliciousNum; i++ {
		n := &node{
			id: generateID(),
			similarities: make(map[*node]float64),
			recomds: make(map[*node]float64),
			_type: "normalMalicious",
			trust: 1.0,
			weight: 0.0,
		}
		sys.nodes[i] = n
	}

	for i := honestNum+normalMaliciousNum; i < totalNodeNum - spyMaliciousNum; i++ {
		n := &node{
			id: generateID(),
			similarities: make(map[*node]float64),
			recomds: make(map[*node]float64),
			_type: "cheatMalicious",
			trust: 1.0,
			weight: 0.0,
		}
		sys.nodes[i] = n
	}

	for i := honestNum+normalMaliciousNum+cheatMaliciousNum; i < totalNodeNum; i++ {
		n := &node{
			id: generateID(),
			similarities: make(map[*node]float64),
			recomds: make(map[*node]float64),
			_type: "spyMalicious",
			trust: 1.0,
			weight: 0.0,
		}
		sys.nodes[i] = n
	}

	// 给其他节点推荐意见
	for i := 0; i < sys.totalNodeNum; i++ {
		for j := 0; j < sys.totalNodeNum; j++ {
			if sys.nodes[i].id == sys.nodes[j].id {
				sys.nodes[i].recomds[sys.nodes[j]] = 0
				continue
			}
			// 诚实节点给其他节点推荐意见
			if sys.nodes[i]._type == "honest" {
				if sys.nodes[j]._type == "honest" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2h")
				} else if sys.nodes[j]._type == "normalMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2m")
				} else if sys.nodes[j]._type == "cheatMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2m")
				} else if sys.nodes[j]._type == "spyMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2h")
				} else {
					panic("未知种类节点")
				}
			}

			// 普通恶意节点给其他节点推荐意见
			if sys.nodes[i]._type == "normalMalicious" {
				if sys.nodes[j]._type == "honest" || sys.nodes[j]._type == "spyMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2h")
				} else if sys.nodes[j]._type == "normalMalicious" || sys.nodes[j]._type == "cheatMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("h2m")
				} else {
					panic("未知种类节点")
				}
			}

			// 合谋恶意节点给其他节点推荐意见
			if sys.nodes[i]._type == "cheatMalicious" {
				if sys.nodes[j]._type == "honest" || sys.nodes[j]._type == "spyMalicious" || sys.nodes[j]._type == "normalMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("m2h")
				} else if sys.nodes[j]._type == "cheatMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("m2m")
				} else {
					panic("未知的节点类型")
				}
			}

			// 恶意间谍节点给其他节点推荐意见
			if sys.nodes[i]._type == "spyMalicious" {
				if sys.nodes[j]._type == "honest" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("m2h")
				} else if sys.nodes[j]._type == "normalMalicious" || sys.nodes[j]._type == "cheatMalicious" || sys.nodes[j]._type == "spyMalicious" {
					sys.nodes[i].recomds[sys.nodes[j]] = evaluate("m2m")
				}
			}
		}
	}

	// 获取节点之间的推荐意见相似度
	for i := 0; i < sys.totalNodeNum; i++ {
		ranki := rank(sys.nodes[i].recomds)
		for j := 0; j <sys.totalNodeNum; j++ {
			rankj := rank(sys.nodes[j].recomds)
			sys.nodes[i].similarities[sys.nodes[j]] = similar(ranki, rankj)
		}
	}

	// 计算每个节点的推荐权重
	for i := 0; i < sys.totalNodeNum; i++ {
		for n, s := range sys.nodes[i].similarities {
			if n.id == sys.nodes[i].id {
				continue
			}
			sys.nodes[i].weight = sys.nodes[i].weight + s / float64(sys.totalNodeNum)
		}
	}

	// 对每个节点的推荐做归一化处理
	for i := 0; i < sys.totalNodeNum; i++ {
		sum := 0.0
		for _, r := range sys.nodes[i].recomds {
			sum += r
		}
		for n, _ := range sys.nodes[i].recomds {
			sys.nodes[i].recomds[n] = sys.nodes[i].recomds[n] / (2 * sum)
		}
	}

	// 开始计算全局信任值
	for {
		c_nodes := make([]*node, len(sys.nodes))
		for i := 0; i < sys.totalNodeNum; i++ {
			c_nodes[i] = &node{
				id: sys.nodes[i].id,
				trust: sys.nodes[i].trust,
			}
		}
		for _, n := range sys.nodes {
			trust := 0.0
			for _, n2 := range sys.nodes {
				trust = trust + n2.trust * n2.recomds[n] * n.similarities[n2]
			}
			n.trust = trust
		}
		avg(sys.nodes)
		diff := 0.0
		for i := 0; i < sys.totalNodeNum; i++ {
			diff += math.Abs(sys.nodes[i].trust - c_nodes[i].trust)
		}
		if diff < sys.threshold {
			break
		}
	}
}

func main() {
	setup(100, 0.33, 0., 1., 0., 0.000000001)}
//h := 0.0
//hnm := 0.0
//hcm := 0.0
//hsm := 0.0
//for n, s := range sys.nodes[0].similarities {
//if n._type == "honest" {
//h += s
//} else if n._type == "normalMalicious" {
//hnm += s
//} else if n._type == "cheatMalicious" {
//hcm += s
//} else if n._type == "spyMalicious" {
//hsm += s
//}
//}
//fmt.Printf("h:%.4f hnm:%.4f hcm:%.4f hsm:%.4f\n", h / float64(honestNum), hnm / float64(normalMaliciousNum), hcm / float64(cheatMaliciousNum), hsm / float64(spyMaliciousNum))