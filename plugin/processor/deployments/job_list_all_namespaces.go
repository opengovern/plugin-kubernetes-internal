package deployments

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListAllNamespacesJob struct {
	processor *Processor
	nodes     []shared.KubernetesNode
}

func NewListAllNamespacesJob(processor *Processor, nodes []shared.KubernetesNode) *ListAllNamespacesJob {
	return &ListAllNamespacesJob{
		processor: processor,
		nodes:     nodes,
	}
}
func (j *ListAllNamespacesJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "list_all_namespaces_for_kubernetes_deployments",
		Description: "Listing all available namespaces (Kubernetes Deployments)",
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
		j.processor.jobQueue.Push(NewListDeploymentsForNamespaceJob(j.processor, namespace, j.nodes))
	}
	return nil
}
