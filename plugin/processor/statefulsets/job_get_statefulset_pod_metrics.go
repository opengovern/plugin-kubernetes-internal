package statefulsets

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
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

func (j *GetStatefulsetPodMetricsJob) Id() string {
	return fmt.Sprintf("get_statefulset_pod_metrics_for_%s", j.itemId)
}
func (j *GetStatefulsetPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for %s (Kubernetes Statefulsets)", j.itemId)
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
	j.processor.items.Set(statefulset.GetID(), statefulset)
	j.processor.publishOptimizationItem(statefulset.ToOptimizationItem())
	j.processor.UpdateSummary(statefulset.GetID())

	if !statefulset.Skipped {
		j.processor.jobQueue.Push(NewOptimizeStatefulsetJob(j.processor, statefulset.GetID()))
	}
	return nil
}
