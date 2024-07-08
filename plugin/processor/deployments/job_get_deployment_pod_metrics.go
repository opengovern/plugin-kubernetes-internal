package deployments

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	"time"
)

type GetDeploymentPodMetricsJob struct {
	processor *Processor
	itemId    string
}

func NewGetDeploymentPodMetricsJob(processor *Processor, itemId string) *GetDeploymentPodMetricsJob {
	return &GetDeploymentPodMetricsJob{
		processor: processor,
		itemId:    itemId,
	}
}
func (j *GetDeploymentPodMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_deployment_pod_metrics_for_%s", j.itemId),
		Description: fmt.Sprintf("Getting metrics for %s (Kubernetes Deployments)", j.itemId),
		MaxRetry:    5,
	}
}
func (j *GetDeploymentPodMetricsJob) Run(ctx context.Context) error {
	deployment, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("deployment not found in the items list")
	}

	cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodOwnerPrefix(ctx, deployment.Namespace, deployment.CurrentReplicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuUsage {
		if deployment.Metrics == nil {
			deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["cpu_usage"] == nil {
			deployment.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		deployment.Metrics["cpu_usage"][podName] = containerMetrics
	}

	cpuThrottling, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodOwnerPrefix(ctx, deployment.Namespace, deployment.CurrentReplicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range cpuThrottling {
		if deployment.Metrics == nil {
			deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["cpu_throttling"] == nil {
			deployment.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		deployment.Metrics["cpu_throttling"][podName] = containerMetrics
	}

	memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodOwnerPrefix(ctx, deployment.Namespace, deployment.CurrentReplicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
	if err != nil {
		return err
	}
	for podName, containerMetrics := range memoryUsage {
		if deployment.Metrics == nil {
			deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["memory_usage"] == nil {
			deployment.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		deployment.Metrics["memory_usage"][podName] = containerMetrics
	}

	for _, replicaSetName := range deployment.HistoricalReplicaSetNames {
		cpuHistoryUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodOwnerPrefix(ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
		if err != nil {
			return err
		}
		for podName, containerMetrics := range cpuHistoryUsage {
			if deployment.Metrics == nil {
				deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if deployment.Metrics["cpu_usage"] == nil {
				deployment.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if _, ok := deployment.Metrics["cpu_usage"][podName]; ok {
				continue
			} else {
				deployment.Metrics["cpu_usage"][podName] = containerMetrics
			}
		}

		cpuThrottlingHistory, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodOwnerPrefix(ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
		if err != nil {
			return err
		}
		for podName, containerMetrics := range cpuThrottlingHistory {
			if deployment.Metrics == nil {
				deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if deployment.Metrics["cpu_throttling"] == nil {
				deployment.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if _, ok := deployment.Metrics["cpu_throttling"][podName]; ok {
				continue
			} else {
				deployment.Metrics["cpu_throttling"][podName] = containerMetrics
			}
		}

		memoryHistoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodOwnerPrefix(ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays, kaytuPrometheus.PodSuffixModeRandom)
		if err != nil {
			return err
		}
		for podName, containerMetrics := range memoryHistoryUsage {
			if deployment.Metrics == nil {
				deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if deployment.Metrics["memory_usage"] == nil {
				deployment.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if _, ok := deployment.Metrics["memory_usage"][podName]; ok {
				continue
			} else {
				deployment.Metrics["memory_usage"][podName] = containerMetrics
			}
		}
	}

	deployment.LazyLoadingEnabled = false
	earliest := time.Now()
	for _, pm := range deployment.Metrics {
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
	deployment.ObservabilityDuration = time.Now().Sub(earliest)

	j.processor.items.Set(deployment.GetID(), deployment)
	j.processor.publishOptimizationItem(deployment.ToOptimizationItem())
	j.processor.UpdateSummary(deployment.GetID())

	if !deployment.Skipped {
		j.processor.jobQueue.Push(NewOptimizeDeploymentJob(j.processor, deployment.GetID()))
	}
	return nil
}
