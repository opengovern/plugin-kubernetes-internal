package pods

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	v1 "k8s.io/api/core/v1"
)

type ListPodsForNamespaceJob struct {
	processor *Processor
	namespace string
	nodes     []shared.KubernetesNode
}

func NewListPodsForNamespaceJob(processor *Processor, namespace string, nodes []shared.KubernetesNode) *ListPodsForNamespaceJob {
	return &ListPodsForNamespaceJob{
		processor: processor,
		namespace: namespace,
		nodes:     nodes,
	}
}

func (j *ListPodsForNamespaceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_pods_for_namespace_kubernetes_%s", j.namespace),
		Description: fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Pods)", j.namespace),
		MaxRetry:    0,
	}
}

func (j *ListPodsForNamespaceJob) Run(ctx context.Context) error {
	pods, err := j.processor.kubernetesProvider.ListPodsInNamespace(ctx, j.namespace, j.processor.selector)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		item := PodItem{
			Pod:                 pod,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         j.processor.defaultPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
			Nodes:               j.nodes,
		}
		if j.processor.nodeSelector != "" {
			if !shared.PodsInNodes([]v1.Pod{item.Pod}, item.Nodes) {
				continue
			}
		}

		if pod.Status.Phase != v1.PodRunning {
			item.Skipped = true
			item.SkipReason = "Pod is not running"
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
		j.processor.jobQueue.Push(NewGetPodMetricsJob(j.processor, item.GetID()))
	}

	return nil
}
