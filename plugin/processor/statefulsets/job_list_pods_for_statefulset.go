package statefulsets

import (
	"context"
	"errors"
	"fmt"
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

func (j *ListPodsForStatefulsetJob) Id() string {
	return fmt.Sprintf("list_pods_for_statefulset_kubernetes_%s", j.itemId)
}
func (j *ListPodsForStatefulsetJob) Description() string {
	return fmt.Sprintf("Listing all pods for statefulset %s (Kubernetes Statefulsets)", j.itemId)
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
	j.processor.items.Set(j.itemId, item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	j.processor.jobQueue.Push(NewGetStatefulsetPodMetricsJob(j.processor, item.GetID()))
	return nil
}
