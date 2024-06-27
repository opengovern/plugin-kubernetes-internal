package jobs

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	corev1 "k8s.io/api/core/v1"
)

type ListJobsForNamespaceJob struct {
	processor *Processor
	namespace string
	nodes     []corev1.Node
}

func NewListJobsForNamespaceJob(processor *Processor, namespace string, nodes []corev1.Node) *ListJobsForNamespaceJob {
	return &ListJobsForNamespaceJob{
		processor: processor,
		namespace: namespace,
		nodes:     nodes,
	}
}

func (j *ListJobsForNamespaceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_jobs_for_namespace_kubernetes_%s", j.namespace),
		Description: fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Jobs)", j.namespace),
		MaxRetry:    0,
	}
}
func (j *ListJobsForNamespaceJob) Run(ctx context.Context) error {
	jobs, err := j.processor.kubernetesProvider.ListJobsInNamespace(ctx, j.namespace, j.processor.selector)
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
			Nodes:               j.nodes,
		}
		if j.processor.nodeSelector != "" {
			if !shared.PodsInNodes(item.Pods, item.Nodes) {
				continue
			}
		}

		if job.Status.Active+job.Status.Succeeded+job.Status.Failed == 0 {
			item.Skipped = true
			item.SkipReason = "no replicas"
		}
		j.processor.lazyloadCounter.Add(1)
		if j.processor.lazyloadCounter.Load() > uint32(j.processor.configuration.KubernetesLazyLoad) {
			item.LazyLoadingEnabled = true
			item.OptimizationLoading = false
		}
		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())

		if item.LazyLoadingEnabled || !item.OptimizationLoading || item.Skipped {
			continue
		}
		j.processor.jobQueue.Push(NewListPodsForJobJob(j.processor, item.GetID()))
	}

	return nil
}
