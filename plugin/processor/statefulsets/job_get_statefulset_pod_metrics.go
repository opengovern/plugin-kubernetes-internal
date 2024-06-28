package statefulsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"time"
)

type GetStatefulsetPodMetricsJob struct {
	processor *Processor
	itemId    string
}

func NewGetStatefulsetPodMetricsJob(processor *Processor, itemId string) *GetStatefulsetPodMetricsJob {
	return &GetStatefulsetPodMetricsJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetStatefulsetPodMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_statefulset_pod_metrics_for_%s", j.itemId),
		Description: fmt.Sprintf("Getting metrics for %s (Kubernetes Statefulsets)", j.itemId),
		MaxRetry:    3,
	}
}

func (j *GetStatefulsetPodMetricsJob) Run(ctx context.Context) error {
	statefulset, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("statefulset not found in the items list")
	}

	cpuUsageWithHistory, err := j.processor.prometheusProvider.GetCpuMetricsForPodOwnerPrefix(ctx, statefulset.Namespace, statefulset.Statefulset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeIncremental)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuUsageWithHistory {
		if statefulset.Metrics == nil {
			statefulset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if statefulset.Metrics["cpu_usage"] == nil {
			statefulset.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := statefulset.Metrics["cpu_usage"][podName]; ok {
			continue
		} else {
			statefulset.Metrics["cpu_usage"][podName] = containerMetrics
		}
	}

	cpuThrottlingWithHistory, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodOwnerPrefix(ctx, statefulset.Namespace, statefulset.Statefulset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeIncremental)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuThrottlingWithHistory {
		if statefulset.Metrics == nil {
			statefulset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if statefulset.Metrics["cpu_throttling"] == nil {
			statefulset.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := statefulset.Metrics["cpu_throttling"][podName]; ok {
			continue
		} else {
			statefulset.Metrics["cpu_throttling"][podName] = containerMetrics
		}
	}

	memoryUsageWithHistory, err := j.processor.prometheusProvider.GetMemoryMetricsForPodOwnerPrefix(ctx, statefulset.Namespace, statefulset.Statefulset.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeIncremental)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range memoryUsageWithHistory {
		if statefulset.Metrics == nil {
			statefulset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if statefulset.Metrics["memory_usage"] == nil {
			statefulset.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := statefulset.Metrics["memory_usage"][podName]; ok {
			continue
		} else {
			statefulset.Metrics["memory_usage"][podName] = containerMetrics
		}
	}

	statefulset.LazyLoadingEnabled = false
	earliest := time.Now()
	for _, pm := range []map[string]map[string][]kaytuPrometheus.PromDatapoint{cpuUsageWithHistory, cpuThrottlingWithHistory, memoryUsageWithHistory} {
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
	statefulset.ObservabilityDuration = time.Now().Sub(earliest)

	j.processor.items.Set(statefulset.GetID(), statefulset)
	j.processor.publishOptimizationItem(statefulset.ToOptimizationItem())
	j.processor.UpdateSummary(statefulset.GetID())

	if !statefulset.Skipped {
		j.processor.jobQueue.Push(NewOptimizeStatefulsetJob(j.processor, statefulset.GetID()))
	}
	return nil
}
