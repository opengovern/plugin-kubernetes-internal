package pods

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
)

type GetPodMetricsJob struct {
	processor *Processor
	podId     string
}

func NewGetPodMetricsJob(processor *Processor, podId string) *GetPodMetricsJob {
	return &GetPodMetricsJob{
		processor: processor,
		podId:     podId,
	}
}

func (j *GetPodMetricsJob) Id() string {
	return fmt.Sprintf("get_pod_metrics_for_%s", j.podId)
}
func (j *GetPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for pod %s (Kubernetes Pods)", j.podId)
}
func (j *GetPodMetricsJob) Run(ctx context.Context) error {
	pod, ok := j.processor.items.Get(j.podId)
	if !ok {
		return errors.New("pod not found in items list")
	}

	cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPod(ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	cpuThrottling, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPod(ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPod(ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	if pod.Metrics == nil {
		pod.Metrics = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
	}

	if pod.Metrics["cpu_usage"] == nil {
		pod.Metrics["cpu_usage"] = cpuUsage
	}

	if pod.Metrics["cpu_throttling"] == nil {
		pod.Metrics["cpu_throttling"] = cpuThrottling
	}

	if pod.Metrics["memory_usage"] == nil {
		pod.Metrics["memory_usage"] = memoryUsage
	}

	pod.LazyLoadingEnabled = false

	j.processor.items.Set(pod.GetID(), pod)
	j.processor.publishOptimizationItem(pod.ToOptimizationItem())
	j.processor.UpdateSummary(pod.GetID())

	if !pod.Skipped {
		j.processor.jobQueue.Push(NewOptimizePodJob(j.processor, pod.GetID()))
	}

	return nil
}
