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

	cpuUsageWithHistory, err := j.processor.prometheusProvider.GetCpuMetricsForPodOwnerPrefix(j.ctx, job.Namespace, job.Job.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuUsageWithHistory {
		if job.Metrics == nil {
			job.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["cpu_usage"] == nil {
			job.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := job.Metrics["cpu_usage"][podName]; ok {
			continue
		} else {
			job.Metrics["cpu_usage"][podName] = containerMetrics
		}
	}

	cpuThrottlingWithHistory, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodOwnerPrefix(j.ctx, job.Namespace, job.Job.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuThrottlingWithHistory {
		if job.Metrics == nil {
			job.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["cpu_throttling"] == nil {
			job.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := job.Metrics["cpu_throttling"][podName]; ok {
			continue
		} else {
			job.Metrics["cpu_throttling"][podName] = containerMetrics
		}
	}

	memoryUsageWithHistory, err := j.processor.prometheusProvider.GetMemoryMetricsForPodOwnerPrefix(j.ctx, job.Namespace, job.Job.Name, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range memoryUsageWithHistory {
		if job.Metrics == nil {
			job.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if job.Metrics["memory_usage"] == nil {
			job.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if _, ok := job.Metrics["memory_usage"][podName]; ok {
			continue
		} else {
			job.Metrics["memory_usage"][podName] = containerMetrics
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
