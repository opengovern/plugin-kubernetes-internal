package deployments

import (
	"context"
)

type ListAllNamespacesJob struct {
	processor *Processor
}

func NewListAllNamespacesJob(processor *Processor) *ListAllNamespacesJob {
	return &ListAllNamespacesJob{
		processor: processor,
	}
}

func (j *ListAllNamespacesJob) Id() string {
	return "list_all_namespaces_for_kubernetes_deployments"
}
func (j *ListAllNamespacesJob) Description() string {
	return "Listing all available namespaces (Kubernetes Deployments)"
}
func (j *ListAllNamespacesJob) Run(ctx context.Context) error {
	var namespaces []string
	if j.processor.namespace != nil &&
		*j.processor.namespace != "" {
		namespaces = []string{*j.processor.namespace}
	} else {
		nss, err := j.processor.kubernetesProvider.ListAllNamespaces(ctx)
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
		j.processor.jobQueue.Push(NewListDeploymentsForNamespaceJob(j.processor, namespace))
	}
	return nil
}
