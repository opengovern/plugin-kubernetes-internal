package jobs

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
)

type ListAllNamespacesJob struct {
	processor *Processor
}

func NewListAllNamespacesJob(processor *Processor) *ListAllNamespacesJob {
	return &ListAllNamespacesJob{
		processor: processor,
	}
}

func (j *ListAllNamespacesJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "list_all_namespaces_for_kubernetes_jobs",
		Description: "Listing all available namespaces (Kubernetes Jobs)",
		MaxRetry:    0,
	}
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
		j.processor.jobQueue.Push(NewListJobsForNamespaceJob(j.processor, namespace))
	}
	return nil
}
