package shared

import (
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"sort"
	"time"
)

func MetricAverageOverObservabilityPeriod(dp []kaytuPrometheus.PromDatapoint, observabilityPeriod time.Duration) float64 {
	if len(dp) == 0 {
		return 0
	}
	if len(dp) == 1 {
		return dp[0].Value
	}

	minDuration := time.Hour
	var sorted []kaytuPrometheus.PromDatapoint
	for _, d := range dp {
		sorted = append(sorted, d)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	for i := 0; i < len(sorted)-1; i++ {
		a := sorted[i].Timestamp
		b := sorted[i+1].Timestamp

		duration := b.Sub(a)
		if duration.Milliseconds() < minDuration.Milliseconds() && duration != 0 {
			minDuration = duration
		}
	}

	total := 0.0
	for _, d := range dp {
		total += d.Value
	}
	totalDPCount := float64(observabilityPeriod) / float64(minDuration)
	avg := total / totalDPCount

	return avg
}
