package pods

import (
	"context"
)

type ListAllNamespacesJob struct {
	ctx       context.Context
	processor *Processor
}

func NewListAllNamespacesJob(ctx context.Context, processor *Processor) *ListAllNamespacesJob {
	return &ListAllNamespacesJob{
		ctx:       ctx,
		processor: processor,
	}
}

func (j *ListAllNamespacesJob) Id() string {
	return "list_all_namespaces_for_kubernetes_pods"
}
func (j *ListAllNamespacesJob) Description() string {
	return "Listing all available namespaces (Kubernetes Pods)"
}
func (j *ListAllNamespacesJob) Run() error {
	namespaces, err := j.processor.provider.ListAllNamespaces(j.ctx)
	if err != nil {
		return err
	}
	for _, namespace := range namespaces {
		if namespace.Name == "kube-system" {
			continue
		}
		j.processor.jobQueue.Push(NewListPodsForNamespaceJob(j.ctx, j.processor, namespace.Name))
	}
	return nil
}
