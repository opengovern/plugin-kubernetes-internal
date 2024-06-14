package daemonsets

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
)

type GetDaemonsetPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewGetDaemonsetPodMetricsJob(ctx context.Context, processor *Processor, itemId string) *GetDaemonsetPodMetricsJob {
	return &GetDaemonsetPodMetricsJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetDaemonsetPodMetricsJob) Id() string {
	return fmt.Sprintf("get_daemonset_pod_metrics_for_%s", j.itemId)
}
func (j *GetDaemonsetPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for %s (Kubernetes Daemonsets)", j.itemId)
}
func (j *GetDaemonsetPodMetricsJob) Run() error {
	daemonset, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("daemonset not found in the items list")
	}

	for _, pod := range daemonset.Pods {
		cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPod(j.ctx, pod.Namespace, pod.Name, j.processor.observabilityDays)
		if err != nil {
			return err
		}

		cpuThrottling, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPod(j.ctx, pod.Namespace, pod.Name, j.processor.observabilityDays)
		if err != nil {
			return err
		}

		memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPod(j.ctx, pod.Namespace, pod.Name, j.processor.observabilityDays)
		if err != nil {
			return err
		}

		if daemonset.Metrics == nil {
			daemonset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}

		if daemonset.Metrics["cpu_usage"] == nil {
			daemonset.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["cpu_usage"][pod.Name] == nil {
			daemonset.Metrics["cpu_usage"][pod.Name] = cpuUsage
		}

		if daemonset.Metrics["cpu_throttling"] == nil {
			daemonset.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["cpu_throttling"][pod.Name] == nil {
			daemonset.Metrics["cpu_throttling"][pod.Name] = cpuThrottling
		}

		if daemonset.Metrics["memory_usage"] == nil {
			daemonset.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if daemonset.Metrics["memory_usage"][pod.Name] == nil {
			daemonset.Metrics["memory_usage"][pod.Name] = memoryUsage
		}
	}
	daemonset.LazyLoadingEnabled = false
	j.processor.items.Set(daemonset.GetID(), daemonset)
	j.processor.publishOptimizationItem(daemonset.ToOptimizationItem())
	j.processor.UpdateSummary(daemonset.GetID())

	if !daemonset.Skipped {
		j.processor.jobQueue.Push(NewOptimizeDaemonsetJob(j.ctx, j.processor, daemonset.GetID()))
	}
	return nil
}
