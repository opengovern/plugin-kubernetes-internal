package deployments

import (
	"context"
	"errors"
	"fmt"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
)

type GetDeploymentPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewGetDeploymentPodMetricsJob(ctx context.Context, processor *Processor, itemId string) *GetDeploymentPodMetricsJob {
	return &GetDeploymentPodMetricsJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *GetDeploymentPodMetricsJob) Id() string {
	return fmt.Sprintf("get_deployment_pod_metrics_for_%s", j.itemId)
}
func (j *GetDeploymentPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for %s (Kubernetes Deployments)", j.itemId)
}
func (j *GetDeploymentPodMetricsJob) Run() error {
	deployment, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("deployment not found in the items list")
	}

	for _, pod := range deployment.Pods {
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

		if deployment.Metrics == nil {
			deployment.Metrics = make(map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}

		if deployment.Metrics["cpu_usage"] == nil {
			deployment.Metrics["cpu_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["cpu_usage"][pod.Name] == nil {
			deployment.Metrics["cpu_usage"][pod.Name] = cpuUsage
		}

		if deployment.Metrics["cpu_throttling"] == nil {
			deployment.Metrics["cpu_throttling"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["cpu_throttling"][pod.Name] == nil {
			deployment.Metrics["cpu_throttling"][pod.Name] = cpuThrottling
		}

		if deployment.Metrics["memory_usage"] == nil {
			deployment.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
		}
		if deployment.Metrics["memory_usage"][pod.Name] == nil {
			deployment.Metrics["memory_usage"][pod.Name] = memoryUsage
		}
	}

	for _, replicaSetName := range deployment.HistoricalReplicaSetNames {
		cpuHistoryUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodPrefix(j.ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays)
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

		cpuThrottlingHistory, err := j.processor.prometheusProvider.GetCpuThrottlingMetricsForPodPrefix(j.ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays)
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

		memoryHistoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodPrefix(j.ctx, deployment.Namespace, replicaSetName, j.processor.observabilityDays)
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
	j.processor.items.Set(deployment.GetID(), deployment)
	j.processor.publishOptimizationItem(deployment.ToOptimizationItem())
	j.processor.UpdateSummary(deployment.GetID())

	if !deployment.Skipped {
		j.processor.jobQueue.Push(NewOptimizeDeploymentJob(j.ctx, j.processor, deployment.GetID()))
	}
	return nil
}
