package deployments

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
	return "list_all_namespaces_for_kubernetes_deployments"
}
func (j *ListAllNamespacesJob) Description() string {
	return "Listing all available namespaces (Kubernetes Deployments)"
}
func (j *ListAllNamespacesJob) Run() error {
	namespaces, err := j.processor.kubernetesProvider.ListAllNamespaces(j.ctx)
	if err != nil {
		return err
	}
	for _, namespace := range namespaces {
		if namespace.Name == "kube-system" {
			continue
		}
		j.processor.jobQueue.Push(NewListDeploymentsForNamespaceJob(j.ctx, j.processor, namespace.Name))
	}
	return nil
}
