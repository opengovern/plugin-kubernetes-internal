package pods

import (
	"context"
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes/plugin/preferences"
	v1 "k8s.io/api/core/v1"
)

type ListPodsForNamespaceJob struct {
	ctx       context.Context
	processor *Processor
	namespace string
}

func NewListPodsForNamespaceJob(ctx context.Context, processor *Processor, namespace string) *ListPodsForNamespaceJob {
	return &ListPodsForNamespaceJob{
		ctx:       ctx,
		processor: processor,
		namespace: namespace,
	}
}

func (j *ListPodsForNamespaceJob) Id() string {
	return fmt.Sprintf("list_pods_for_namespace_kubernetes_%s", j.namespace)
}
func (j *ListPodsForNamespaceJob) Description() string {
	return fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Pods)", j.namespace)
}
func (j *ListPodsForNamespaceJob) Run() error {
	pods, err := j.processor.kubernetesProvider.ListPodsInNamespace(j.ctx, j.namespace)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		item := PodItem{
			Pod:                 pod,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultPodsPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
		}

		if pod.Status.Phase != v1.PodRunning {
			item.Skipped = true
			item.SkipReason = "Pod is not running"
		}

		j.processor.lazyloadCounter.Increment()
		if j.processor.lazyloadCounter.Get() > j.processor.configuration.KubernetesLazyLoad {
			item.LazyLoadingEnabled = true
			item.OptimizationLoading = false
		}

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		item.UpdateSummary(j.processor)
	}

	for _, pod := range pods {
		i, ok := j.processor.items.Get(fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		if ok && (i.LazyLoadingEnabled || !i.OptimizationLoading || i.Skipped) {
			continue
		}

		j.processor.jobQueue.Push(NewGetPodMetricsJob(j.ctx, j.processor, i.GetID()))
	}

	return nil
}
