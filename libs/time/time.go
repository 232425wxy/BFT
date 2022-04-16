package time

import (
	"sort"
	"time"
)

func Now() time.Time {
	return Canonical(time.Now())
}

func Canonical(t time.Time) time.Time {
	return t.Round(time.Nanosecond)
}

// WeightedTime 它用于计算中值。
type WeightedTime struct {
	Time   time.Time
	Weight int64
}

// NewWeightedTime 用给定的 time 和权重实例化一个 WeightedTime
func NewWeightedTime(time time.Time, weight int64) *WeightedTime {
	return &WeightedTime{
		Time:   time,
		Weight: weight,
	}
}

// WeightedMedian 为给定的 WeightedTime 数组和总投票权计算加权中值时间。
func WeightedMedian(weightedTimes []*WeightedTime, totalVotingPower int64) (res time.Time) {
	median := totalVotingPower / 2 // 总投票权的一半

	// 对 weightedTimes 进行排序，排序规则是：谁的 time 小，谁排在前面
	sort.Slice(weightedTimes, func(i, j int) bool {
		if weightedTimes[i] == nil {
			// 如果 weightedTimes[i] 为 nil，则 j 排在前面
			return false
		}
		if weightedTimes[j] == nil {
			// 如果 j 为 nil，则 i 排在前面
			return true
		}
		// 如果 i 的 time 小于 j 的 time，则 i 排在前面
		return weightedTimes[i].Time.UnixNano() < weightedTimes[j].Time.UnixNano()
	})

	for _, weightedTime := range weightedTimes {
		if weightedTime != nil {
			if median <= weightedTime.Weight {
				res = weightedTime.Time
				break
			}
			median -= weightedTime.Weight
		}
	}
	return
}
