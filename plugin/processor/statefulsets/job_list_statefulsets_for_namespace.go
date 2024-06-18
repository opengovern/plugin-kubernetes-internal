package statefulsets

import (
	"context"
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
)

type ListStatefulsetsForNamespaceJob struct {
	ctx       context.Context
	processor *Processor
	namespace string
}

func NewListStatefulsetsForNamespaceJob(ctx context.Context, processor *Processor, namespace string) *ListStatefulsetsForNamespaceJob {
	return &ListStatefulsetsForNamespaceJob{
		ctx:       ctx,
		processor: processor,
		namespace: namespace,
	}
}

func (j *ListStatefulsetsForNamespaceJob) Id() string {
	return fmt.Sprintf("list_statefulsets_for_namespace_kubernetes_%s", j.namespace)
}
func (j *ListStatefulsetsForNamespaceJob) Description() string {
	return fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Statefulsets)", j.namespace)
}
func (j *ListStatefulsetsForNamespaceJob) Run() error {
	statefulsets, err := j.processor.kubernetesProvider.ListStatefulsetsInNamespace(j.ctx, j.namespace)
	if err != nil {
		return err
	}

	for _, statefulset := range statefulsets {
		item := StatefulsetItem{
			Statefulset:         statefulset,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultStatefulsetsPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
		}

		if statefulset.Status.AvailableReplicas == 0 {
			item.Skipped = true
			item.SkipReason = "no available replicas"
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
		j.processor.jobQueue.Push(NewListPodsForStatefulsetJob(j.ctx, j.processor, item.GetID()))
	}

	return nil
}
