package deployments

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListPodsForDeploymentJob struct {
	processor *Processor
	itemId    string
}

func NewListPodsForDeploymentJob(processor *Processor, itemId string) *ListPodsForDeploymentJob {
	return &ListPodsForDeploymentJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *ListPodsForDeploymentJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_pods_for_deployment_kubernetes_%s", j.itemId),
		Description: fmt.Sprintf("Listing all pods for deployment %s (Kubernetes Deployments)", j.itemId),
		MaxRetry:    0,
	}
}

func (j *ListPodsForDeploymentJob) Run(ctx context.Context) error {
	var err error
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("deployment not found in the items list")
	}

	item.Pods, item.HistoricalReplicaSetNames, err = j.processor.kubernetesProvider.ListDeploymentPodsAndHistoricalReplicaSets(ctx, item.Deployment, j.processor.observabilityDays)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	item.LazyLoadingEnabled = false
	if j.processor.nodeSelector != "" {
		if !shared.PodsInNodes(item.Pods, item.Nodes) {
			item.Skipped = true
			item.SkipReason = "not in selected nodes"
			j.processor.items.Set(j.itemId, item)
			j.processor.publishOptimizationItem(item.ToOptimizationItem())
			return nil
		}
	}

	j.processor.items.Set(j.itemId, item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	j.processor.jobQueue.Push(NewGetDeploymentPodMetricsJob(j.processor, item.GetID()))
	return nil
}
