package pods

import (
	"context"
	"fmt"
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
	return "list_pods_for_namespace_kubernetes"
}
func (j *ListPodsForNamespaceJob) Description() string {
	return "Listing all pods in namespace (Kubernetes Pods)"
}
func (j *ListPodsForNamespaceJob) Run() error {
	pods, err := j.processor.provider.ListPodsInNamespace(j.ctx, j.namespace)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		item := PodItem{
			Pod:                 pod,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         nil,
			Skipped:             true,
			LazyLoadingEnabled:  false,
			SkipReason:          "WIP",
		}

		// TODO: metrics and lazy loading

		j.processor.items.Set(fmt.Sprintf("%s/%s", pod.Namespace, pod.Name), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
	}
	return nil
}
