package statefulsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListPodsForStatefulsetJob struct {
	processor *Processor
	itemId    string
}

func NewListPodsForStatefulsetJob(processor *Processor, itemId string) *ListPodsForStatefulsetJob {
	return &ListPodsForStatefulsetJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *ListPodsForStatefulsetJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_pods_for_statefulset_kubernetes_%s", j.itemId),
		Description: fmt.Sprintf("Listing all pods for statefulset %s (Kubernetes Statefulsets)", j.itemId),
		MaxRetry:    0,
	}
}

func (j *ListPodsForStatefulsetJob) Run(ctx context.Context) error {
	var err error
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("statefulset not found in the items list")
	}

	item.Pods, err = j.processor.kubernetesProvider.ListStatefulsetPods(ctx, item.Statefulset)
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

	j.processor.jobQueue.Push(NewGetStatefulsetPodMetricsJob(j.processor, item.GetID()))
	return nil
}
