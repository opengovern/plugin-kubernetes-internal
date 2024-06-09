package statefulsets

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
)

type GetStatefulsetPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewGetStatefulsetPodMetricsJob(ctx context.Context, processor *Processor, itemId string) *GetStatefulsetPodMetricsJob {
	return &GetStatefulsetPodMetricsJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetStatefulsetPodMetricsJob) Id() string {
	return fmt.Sprintf("get_statefulset_pod_metrics_for_%s_%s", j.itemId)
}
func (j *GetStatefulsetPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for %s (Kubernetes Statefulsets)", j.itemId)
}
func (j *GetStatefulsetPodMetricsJob) Run() error {
	statefulset, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("statefulset not found in the items list")
	}

	for _, pod := range statefulset.Pods {
		for _, container := range pod.Spec.Containers {
			cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodContainer(j.ctx, pod.Namespace, pod.Name, container.Name, j.processor.observabilityDays)
			if err != nil {
				return err
			}

			memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodContainer(j.ctx, pod.Namespace, pod.Name, container.Name, j.processor.observabilityDays)
			if err != nil {
				return err
			}

			if statefulset.Metrics == nil {
				statefulset.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}

			if statefulset.Metrics["cpu_usage"] == nil {
				statefulset.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if statefulset.Metrics["cpu_usage"][pod.Name] == nil {
				statefulset.Metrics["cpu_usage"][pod.Name] = make(map[string][]kaytuPrometheus.PromDatapoint)
			}
			statefulset.Metrics["cpu_usage"][pod.Name][container.Name] = cpuUsage

			if statefulset.Metrics["memory_usage"] == nil {
				statefulset.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if statefulset.Metrics["memory_usage"][pod.Name] == nil {
				statefulset.Metrics["memory_usage"][pod.Name] = make(map[string][]kaytuPrometheus.PromDatapoint)
			}
			statefulset.Metrics["memory_usage"][pod.Name][container.Name] = memoryUsage
		}
	}
	statefulset.LazyLoadingEnabled = false
	j.processor.items.Set(statefulset.GetID(), statefulset)
	j.processor.publishOptimizationItem(statefulset.ToOptimizationItem())
	j.processor.UpdateSummary(statefulset.GetID())

	if !statefulset.Skipped {
		j.processor.jobQueue.Push(NewOptimizeStatefulsetJob(j.ctx, j.processor, statefulset.GetID()))
	}
	return nil
}
