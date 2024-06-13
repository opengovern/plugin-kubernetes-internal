package jobs

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
)

type GetJobPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewGetJobPodMetricsJob(ctx context.Context, processor *Processor, itemId string) *GetJobPodMetricsJob {
	return &GetJobPodMetricsJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetJobPodMetricsJob) Id() string {
	return fmt.Sprintf("get_job_pod_metrics_for_%s", j.itemId)
}
func (j *GetJobPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for %s (Kubernetes Jobs)", j.itemId)
}
func (j *GetJobPodMetricsJob) Run() error {
	job, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("job not found in the items list")
	}

	for _, pod := range job.Pods {
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

		if job.Metrics == nil {
			job.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}

		if job.Metrics["cpu_usage"] == nil {
			job.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["cpu_usage"][pod.Name] == nil {
			job.Metrics["cpu_usage"][pod.Name] = cpuUsage
		}

		if job.Metrics["cpu_throttling"] == nil {
			job.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["cpu_throttling"][pod.Name] == nil {
			job.Metrics["cpu_throttling"][pod.Name] = cpuThrottling
		}

		if job.Metrics["memory_usage"] == nil {
			job.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["memory_usage"][pod.Name] == nil {
			job.Metrics["memory_usage"][pod.Name] = memoryUsage
		}
	}
	job.LazyLoadingEnabled = false
	j.processor.items.Set(job.GetID(), job)
	j.processor.publishOptimizationItem(job.ToOptimizationItem())
	j.processor.UpdateSummary(job.GetID())

	if !job.Skipped {
		j.processor.jobQueue.Push(NewOptimizeJobJob(j.ctx, j.processor, job.GetID()))
	}
	return nil
}
