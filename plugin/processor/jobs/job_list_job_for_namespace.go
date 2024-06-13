package jobs

import (
	"context"
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
)

type ListJobsForNamespaceJob struct {
	ctx       context.Context
	processor *Processor
	namespace string
}

func NewListJobsForNamespaceJob(ctx context.Context, processor *Processor, namespace string) *ListJobsForNamespaceJob {
	return &ListJobsForNamespaceJob{
		ctx:       ctx,
		processor: processor,
		namespace: namespace,
	}
}

func (j *ListJobsForNamespaceJob) Id() string {
	return fmt.Sprintf("list_jobs_for_namespace_kubernetes_%s", j.namespace)
}
func (j *ListJobsForNamespaceJob) Description() string {
	return fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Jobs)", j.namespace)
}
func (j *ListJobsForNamespaceJob) Run() error {
	jobs, err := j.processor.kubernetesProvider.ListJobsInNamespace(j.ctx, j.namespace)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		item := JobItem{
			Job:                 job,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultJobsPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
		}

		if job.Status.Active+job.Status.Succeeded+job.Status.Failed == 0 {
			item.Skipped = true
			item.SkipReason = "no replicas"
		}
		j.processor.lazyloadCounter.Increment()
		if j.processor.lazyloadCounter.Get() > j.processor.configuration.KubernetesLazyLoad {
			item.LazyLoadingEnabled = true
			item.OptimizationLoading = false
		}
		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())

		if item.LazyLoadingEnabled || !item.OptimizationLoading || item.Skipped {
			continue
		}
		j.processor.jobQueue.Push(NewListPodsForJobJob(j.ctx, j.processor, item.GetID()))
	}

	return nil
}
