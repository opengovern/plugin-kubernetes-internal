package jobs

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
	return "list_all_namespaces_for_kubernetes_jobs"
}
func (j *ListAllNamespacesJob) Description() string {
	return "Listing all available namespaces (Kubernetes Jobs)"
}
func (j *ListAllNamespacesJob) Run() error {
	var namespaces []string
	if j.processor.namespace != nil &&
		*j.processor.namespace != "" {
		namespaces = []string{*j.processor.namespace}
	} else {
		nss, err := j.processor.kubernetesProvider.ListAllNamespaces(j.ctx)
		if err != nil {
			return err
		}

		for _, ns := range nss {
			namespaces = append(namespaces, ns.Name)
		}
	}
	for _, namespace := range namespaces {
		if namespace == "kube-system" {
			continue
		}
		j.processor.jobQueue.Push(NewListJobsForNamespaceJob(j.ctx, j.processor, namespace))
	}
	return nil
}
