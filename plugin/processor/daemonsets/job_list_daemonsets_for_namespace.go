package daemonsets

import (
	"context"
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
)

type ListDaemonsetsForNamespaceJob struct {
	ctx       context.Context
	processor *Processor
	namespace string
}

func NewListDaemonsetsForNamespaceJob(ctx context.Context, processor *Processor, namespace string) *ListDaemonsetsForNamespaceJob {
	return &ListDaemonsetsForNamespaceJob{
		ctx:       ctx,
		processor: processor,
		namespace: namespace,
	}
}

func (j *ListDaemonsetsForNamespaceJob) Id() string {
	return fmt.Sprintf("list_daemonsets_for_namespace_kubernetes_%s", j.namespace)
}
func (j *ListDaemonsetsForNamespaceJob) Description() string {
	return fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Daemonsets)", j.namespace)
}
func (j *ListDaemonsetsForNamespaceJob) Run() error {
	daemonsets, err := j.processor.kubernetesProvider.ListDaemonsetsInNamespace(j.ctx, j.namespace)
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
		}

		if daemonset.Status.CurrentNumberScheduled == 0 {
			item.Skipped = true
			item.SkipReason = "no available replicas"
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
		j.processor.jobQueue.Push(NewListPodsForDaemonsetJob(j.ctx, j.processor, item.GetID()))
	}

	return nil
}
