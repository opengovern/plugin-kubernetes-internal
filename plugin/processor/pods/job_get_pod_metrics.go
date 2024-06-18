package pods

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"log"
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
	log.Printf("------- job %s - starting", j.Id())
	pod, ok := j.processor.items.Get(j.podId)
	if !ok {
		return errors.New("pod not found in items list")
	}

	log.Printf("------- job %s - getting cpu metrics", j.Id())
	cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPod(j.ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	log.Printf("------- job %s - getting cpu throttling metrics", j.Id())
	cpuThrottling, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPod(j.ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	log.Printf("------- job %s - getting memory metrics", j.Id())
	memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPod(j.ctx, pod.Pod.Namespace, pod.Pod.Name, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	log.Printf("------- job %s - setting metrics", j.Id())
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

	log.Printf("------- job %s - setting pod item", j.Id())
	j.processor.items.Set(pod.GetID(), pod)
	log.Printf("------- job %s - publishing optimization item", j.Id())
	j.processor.publishOptimizationItem(pod.ToOptimizationItem())
	log.Printf("------- job %s - updating summary", j.Id())
	j.processor.UpdateSummary(pod.GetID())

	if !pod.Skipped {
		log.Printf("------- job %s - pushing new optimize pod job", j.Id())
		j.processor.jobQueue.Push(NewOptimizePodJob(j.ctx, j.processor, pod.GetID()))
	}

	log.Printf("------- job %s - done", j.Id())
	return nil
}
