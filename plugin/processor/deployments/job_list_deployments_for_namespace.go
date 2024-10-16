package deployments

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/preferences"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
)

type ListDeploymentsForNamespaceJob struct {
	processor *Processor
	namespace string
	nodes     []shared.KubernetesNode
}

func NewListDeploymentsForNamespaceJob(processor *Processor, namespace string, nodes []shared.KubernetesNode) *ListDeploymentsForNamespaceJob {
	return &ListDeploymentsForNamespaceJob{
		processor: processor,
		namespace: namespace,
		nodes:     nodes,
	}
}

func (j *ListDeploymentsForNamespaceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("list_deployments_for_namespace_kubernetes_%s", j.namespace),
		Description: fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Deployments)", j.namespace),
		MaxRetry:    0,
	}
}

func (j *ListDeploymentsForNamespaceJob) Run(ctx context.Context) error {
	deployments, err := j.processor.kubernetesProvider.ListDeploymentsInNamespace(ctx, j.namespace, j.processor.selector)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		item := DeploymentItem{
			Deployment:          deployment,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultKubernetesPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
			Nodes:               j.nodes,
		}

		if deployment.Status.AvailableReplicas == 0 {
			item.Skipped = true
			item.SkipReason = "no available replicas"
		}
		j.processor.lazyloadCounter.Add(1)
		if j.processor.lazyloadCounter.Load() > uint32(j.processor.configuration.KubernetesLazyLoad) {
			item.LazyLoadingEnabled = true
			item.OptimizationLoading = false
		}
		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())

		if item.LazyLoadingEnabled || !item.OptimizationLoading || item.Skipped {
			fmt.Println("skipping deployment", item.Deployment.Name)
			continue
		}
		j.processor.jobQueue.Push(NewListPodsForDeploymentJob(j.processor, item.GetID()))
	}

	return nil
}
