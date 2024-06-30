package deployments

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListAllNodesJob struct {
	processor *Processor
}

func NewListAllNodesJob(processor *Processor) *ListAllNodesJob {
	return &ListAllNodesJob{
		processor: processor,
	}
}
func (j *ListAllNodesJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "list_all_nodes_for_kubernetes_deployments",
		Description: "Listing all available nodes (Kubernetes Deployments)",
		MaxRetry:    0,
	}
}

func (j *ListAllNodesJob) Run(ctx context.Context) error {
	nodes, err := j.processor.kubernetesProvider.ListAllNodes(ctx, j.processor.nodeSelector)
	if err != nil {
		return err
	}

	var knodes []shared.KubernetesNode
	for _, node := range nodes {
		knodes = append(knodes, shared.KubernetesNode{
			Name: node.Name,
			Cost: nil,
		})
	}

	if j.processor.kaytuClient.IsEnabled() {
		j.processor.jobQueue.Push(NewDownloadKaytuAgentReportJob(j.processor, knodes))
	} else {
		j.processor.jobQueue.Push(NewListAllNamespacesJob(j.processor, knodes))
	}
	return nil
}
