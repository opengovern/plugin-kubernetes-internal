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
		for _, container := range pod.Spec.Containers {
			cpuUsage, err := j.processor.prometheusProvider.GetCpuMetricsForPodContainer(j.ctx, pod.Namespace, pod.Name, container.Name, j.processor.observabilityDays)
			if err != nil {
				return err
			}

			memoryUsage, err := j.processor.prometheusProvider.GetMemoryMetricsForPodContainer(j.ctx, pod.Namespace, pod.Name, container.Name, j.processor.observabilityDays)
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
				deployment.Metrics["cpu_usage"][pod.Name] = make(map[string][]kaytuPrometheus.PromDatapoint)
			}
			deployment.Metrics["cpu_usage"][pod.Name][container.Name] = cpuUsage

			if deployment.Metrics["memory_usage"] == nil {
				deployment.Metrics["memory_usage"] = make(map[string]map[string][]kaytuPrometheus.PromDatapoint)
			}
			if deployment.Metrics["memory_usage"][pod.Name] == nil {
				deployment.Metrics["memory_usage"][pod.Name] = make(map[string][]kaytuPrometheus.PromDatapoint)
			}
			deployment.Metrics["memory_usage"][pod.Name][container.Name] = memoryUsage
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
