package nodes

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"strings"
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
		ID:          "list_all_nodes_for_kubernetes_nodes",
		Description: "Listing all available nodes (Kubernetes Nodes)",
		MaxRetry:    0,
	}
}

func (j *ListAllNodesJob) Run(ctx context.Context) error {
	defer j.processor.nodesReady.Done()
	nodes, err := j.processor.kubernetesProvider.ListAllNodes(ctx, j.processor.nodeSelector)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		item := NodeItem{
			Node:        node,
			ClusterType: ClusterTypeUnknown,
		}

	clusterTypeLoop:
		for labelKey, _ := range node.Labels {
			labelKey := strings.ToLower(labelKey)
			switch {
			case strings.HasPrefix(labelKey, "eks.amazonaws.com/"):
				item.ClusterType = ClusterTypeAwsEks
				break clusterTypeLoop
			case strings.HasPrefix(labelKey, "kubernetes.azure.com/"):
				item.ClusterType = ClusterTypeAzureAks
				break clusterTypeLoop
				// TODO @Arta GCP case
			}
		}

		j.processor.items.Set(item.GetID(), item)

		switch item.ClusterType {
		case ClusterTypeAwsEks, ClusterTypeAzureAks, ClusterTypeGoogleGke:
			j.processor.jobQueue.Push(NewGetNodeCostJob(j.processor, item.GetID()))
		default:
			item.Skipped = true
			item.SkipReason = "Unknown cluster type"
			j.processor.items.Set(item.GetID(), item)
		}
	}

	return nil
}
