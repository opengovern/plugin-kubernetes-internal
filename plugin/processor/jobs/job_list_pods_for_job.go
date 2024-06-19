package jobs

import (
	"context"
	"errors"
	"fmt"
)

type ListPodsForJobJob struct {
	processor *Processor
	itemId    string
}

func NewListPodsForJobJob(processor *Processor, itemId string) *ListPodsForJobJob {
	return &ListPodsForJobJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *ListPodsForJobJob) Id() string {
	return fmt.Sprintf("list_pods_for_job_kubernetes_%s", j.itemId)
}
func (j *ListPodsForJobJob) Description() string {
	return fmt.Sprintf("Listing all pods for job %s (Kubernetes Jobs)", j.itemId)
}
func (j *ListPodsForJobJob) Run(ctx context.Context) error {
	var err error
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("job not found in the items list")
	}

	item.Pods, err = j.processor.kubernetesProvider.ListJobPods(ctx, item.Job)
	if err != nil {
		return err
	}

	item.LazyLoadingEnabled = false
	j.processor.items.Set(j.itemId, item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	j.processor.jobQueue.Push(NewGetJobPodMetricsJob(j.processor, item.GetID()))
	return nil
}
