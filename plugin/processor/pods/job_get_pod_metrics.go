package pods

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
)

type GetPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	podId     string
}

func NewGetPodMetricsJob(ctx context.Context, processor *Processor, podId string) *GetPodMetricsJob {
	return &GetPodMetricsJob{
		ctx:       ctx,
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
func (j *GetPodMetricsJob) Run() error {
	pod, ok := j.processor.items.Get(j.podId)
	if !ok {
		return errors.New("pod not found in items list")
	}
	for _, container := range pod.Pod.Spec.Containers {
		cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodContainer(j.ctx, pod.Pod.Namespace, pod.Pod.Name, container.Name)
		if err != nil {
			return err
		}

		memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodContainer(j.ctx, pod.Pod.Namespace, pod.Pod.Name, container.Name)
		if err != nil {
			return err
		}

		if pod.Metrics == nil {
			pod.Metrics = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if pod.Metrics["cpu_usage"] == nil {
			pod.Metrics["cpu_usage"] = make(map[string][]kaytuPrometheus.PromDatapoint)
		}
		pod.Metrics["cpu_usage"][container.Name] = cpuUsage

		if pod.Metrics["memory_usage"] == nil {
			pod.Metrics["memory_usage"] = make(map[string][]kaytuPrometheus.PromDatapoint)
		}
		pod.Metrics["memory_usage"][container.Name] = memoryUsage
	}

	j.processor.items.Set(pod.GetID(), pod)
	j.processor.publishOptimizationItem(pod.ToOptimizationItem())
	return nil
}
