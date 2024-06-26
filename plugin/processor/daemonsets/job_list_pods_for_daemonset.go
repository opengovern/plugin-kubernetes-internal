package daemonsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
)

type ListPodsForDaemonsetJob struct {
	processor *Processor
	itemId    string
}

func NewListPodsForDaemonsetJob(processor *Processor, itemId string) *ListPodsForDaemonsetJob {
	return &ListPodsForDaemonsetJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *ListPodsForDaemonsetJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_pods_for_daemonset_kubernetes_%s", j.itemId),
		Description: fmt.Sprintf("Listing all pods for daemonset %s (Kubernetes Daemonsets)", j.itemId),
		MaxRetry:    0,
	}
}
func (j *ListPodsForDaemonsetJob) Run(ctx context.Context) error {
	var err error
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("daemonset not found in the items list")
	}

	item.Pods, err = j.processor.kubernetesProvider.ListDaemonsetPods(ctx, item.Daemonset)
	if err != nil {
		return err
	}

	item.LazyLoadingEnabled = false
	j.processor.items.Set(j.itemId, item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	j.processor.jobQueue.Push(NewGetDaemonsetPodMetricsJob(j.processor, item.GetID()))
	return nil
}
