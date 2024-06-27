package daemonsets

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
	corev1 "k8s.io/api/core/v1"
)

type ListDaemonsetsForNamespaceJob struct {
	processor *Processor
	namespace string
	nodes     []corev1.Node
}

func NewListDaemonsetsForNamespaceJob(processor *Processor, namespace string, nodes []corev1.Node) *ListDaemonsetsForNamespaceJob {
	return &ListDaemonsetsForNamespaceJob{
		processor: processor,
		namespace: namespace,
		nodes:     nodes,
	}
}
func (j *ListDaemonsetsForNamespaceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_daemonsets_for_namespace_kubernetes_%s", j.namespace),
		Description: fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Daemonsets)", j.namespace),
		MaxRetry:    0,
	}
}

func (j *ListDaemonsetsForNamespaceJob) Run(ctx context.Context) error {
	daemonsets, err := j.processor.kubernetesProvider.ListDaemonsetsInNamespace(ctx, j.namespace, j.processor.selector)
	if err != nil {
		return err
	}

	for _, daemonset := range daemonsets {
		item := DaemonsetItem{
			Daemonset:           daemonset,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultDaemonsetsPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
			Nodes:               j.nodes,
		}

		if daemonset.Status.CurrentNumberScheduled == 0 {
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
		j.processor.jobQueue.Push(NewListPodsForDaemonsetJob(j.processor, item.GetID()))
	}

	return nil
}
