package deployments

import (
	"context"
	"errors"
	"fmt"
)

type ListPodsForDeploymentJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewListPodsForDeploymentJob(ctx context.Context, processor *Processor, itemId string) *ListPodsForDeploymentJob {
	return &ListPodsForDeploymentJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *ListPodsForDeploymentJob) Id() string {
	return fmt.Sprintf("list_pods_for_deployment_kubernetes_%s", j.itemId)
}
func (j *ListPodsForDeploymentJob) Description() string {
	return fmt.Sprintf("Listing all pods for deployment %s (Kubernetes Deployments)", j.itemId)
}
func (j *ListPodsForDeploymentJob) Run() error {
	var err error
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("deployment not found in the items list")
	}

	item.Pods, err = j.processor.kubernetesProvider.ListDeploymentPods(j.ctx, item.Deployment)
	if err != nil {
		return err
	}

	item.LazyLoadingEnabled = false
	j.processor.items.Set(j.itemId, item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	j.processor.jobQueue.Push(NewGetDeploymentPodMetricsJob(j.ctx, j.processor, item.GetID()))
	return nil
}
