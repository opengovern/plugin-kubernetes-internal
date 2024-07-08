package pods

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"time"
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

func (j *GetPodMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_pod_metrics_for_%s", j.podId),
		Description: fmt.Sprintf("Getting metrics for pod %s (Kubernetes Pods)", j.podId),
		MaxRetry:    5,
	}
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
	earliest := time.Now()
	for _, pm := range []map[string][]kaytuPrometheus.PromDatapoint{cpuUsage, cpuThrottling, memoryUsage} {
		for _, dps := range pm {
			for _, m := range dps {
				if m.Timestamp.Before(earliest) {
					earliest = m.Timestamp
				}
			}
		}
	}
	pod.ObservabilityDuration = time.Now().Sub(earliest)

	j.processor.items.Set(pod.GetID(), pod)
	j.processor.publishOptimizationItem(pod.ToOptimizationItem())
	j.processor.UpdateSummary(pod.GetID())

	if !pod.Skipped {
		j.processor.jobQueue.Push(NewOptimizePodJob(j.processor, pod.GetID()))
	}

	return nil
}
