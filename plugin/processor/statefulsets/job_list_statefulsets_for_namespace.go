package statefulsets

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListStatefulsetsForNamespaceJob struct {
	processor *Processor
	namespace string
	nodes     []shared.KubernetesNode
}

func NewListStatefulsetsForNamespaceJob(processor *Processor, namespace string, nodes []shared.KubernetesNode) *ListStatefulsetsForNamespaceJob {
	return &ListStatefulsetsForNamespaceJob{
		processor: processor,
		namespace: namespace,
		nodes:     nodes,
	}
}

func (j *ListStatefulsetsForNamespaceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_statefulsets_for_namespace_kubernetes_%s", j.namespace),
		Description: fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Statefulsets)", j.namespace),
		MaxRetry:    0,
	}
}
func (j *ListStatefulsetsForNamespaceJob) Run(ctx context.Context) error {
	statefulsets, err := j.processor.kubernetesProvider.ListStatefulsetsInNamespace(ctx, j.namespace, j.processor.selector)
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
			Nodes:               j.nodes,
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
		j.processor.jobQueue.Push(NewListPodsForStatefulsetJob(j.processor, item.GetID()))
	}

	return nil
}
