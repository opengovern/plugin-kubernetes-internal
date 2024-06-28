package daemonsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"time"
)

type GetDaemonsetPodMetricsJob struct {
	processor *Processor
	itemId    string
}

func NewGetDaemonsetPodMetricsJob(processor *Processor, itemId string) *GetDaemonsetPodMetricsJob {
	return &GetDaemonsetPodMetricsJob{
		processor: processor,
		itemId:    itemId,
	}
}
func (j *GetDaemonsetPodMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_daemonset_pod_metrics_for_%s", j.itemId),
		Description: fmt.Sprintf("Getting metrics for %s (Kubernetes Daemonsets)", j.itemId),
		MaxRetry:    3,
	}
}

func (j *GetDaemonsetPodMetricsJob) Run(ctx context.Context) error {
	daemonset, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("daemonset not found in the items list")
	}

	cpuUsageWithHistory, err := j.processor.prometheusProvider.GetCpuMetricsForPodOwnerPrefix(ctx, daemonset.Namespace, daemonset.Daemonset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuUsageWithHistory {
		if daemonset.Metrics == nil {
			daemonset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["cpu_usage"] == nil {
			daemonset.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := daemonset.Metrics["cpu_usage"][podName]; ok {
			continue
		} else {
			daemonset.Metrics["cpu_usage"][podName] = containerMetrics
		}
	}

	cpuThrottlingWithHistory, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodOwnerPrefix(ctx, daemonset.Namespace, daemonset.Daemonset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuThrottlingWithHistory {
		if daemonset.Metrics == nil {
			daemonset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["cpu_throttling"] == nil {
			daemonset.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := daemonset.Metrics["cpu_throttling"][podName]; ok {
			continue
		} else {
			daemonset.Metrics["cpu_throttling"][podName] = containerMetrics
		}
	}

	memoryUsageWithHistory, err := j.processor.prometheusProvider.GetMemoryMetricsForPodOwnerPrefix(ctx, daemonset.Namespace, daemonset.Daemonset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range memoryUsageWithHistory {
		if daemonset.Metrics == nil {
			daemonset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["memory_usage"] == nil {
			daemonset.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := daemonset.Metrics["memory_usage"][podName]; ok {
			continue
		} else {
			daemonset.Metrics["memory_usage"][podName] = containerMetrics
		}
	}

	daemonset.LazyLoadingEnabled = false
	earliest := time.Now()
	for _, pm := range daemonset.Metrics {
		for _, kvs := range pm {
			for _, v := range kvs {
				for _, m := range v {
					if m.Timestamp.Before(earliest) {
						earliest = m.Timestamp
					}
				}
			}
		}
	}
	daemonset.ObservabilityDuration = time.Now().Sub(earliest)

	j.processor.items.Set(daemonset.GetID(), daemonset)
	j.processor.publishOptimizationItem(daemonset.ToOptimizationItem())
	j.processor.UpdateSummary(daemonset.GetID())

	if !daemonset.Skipped {
		j.processor.jobQueue.Push(NewOptimizeDaemonsetJob(j.processor, daemonset.GetID()))
	}
	return nil
}
