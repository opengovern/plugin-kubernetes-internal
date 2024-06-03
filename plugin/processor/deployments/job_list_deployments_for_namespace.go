package deployments

import (
	"context"
	"fmt"
	"github.com/kaytu-io/plugin-kubernetes/plugin/preferences"
)

type ListDeploymentsForNamespaceJob struct {
	ctx       context.Context
	processor *Processor
	namespace string
}

func NewListDeploymentsForNamespaceJob(ctx context.Context, processor *Processor, namespace string) *ListDeploymentsForNamespaceJob {
	return &ListDeploymentsForNamespaceJob{
		ctx:       ctx,
		processor: processor,
		namespace: namespace,
	}
}

func (j *ListDeploymentsForNamespaceJob) Id() string {
	return fmt.Sprintf("list_deployments_for_namespace_kubernetes_%s", j.namespace)
}
func (j *ListDeploymentsForNamespaceJob) Description() string {
	return fmt.Sprintf("Listing all pods in namespace %s (Kubernetes Deployments)", j.namespace)
}
func (j *ListDeploymentsForNamespaceJob) Run() error {
	deployments, err := j.processor.kubernetesProvider.ListDeploymentsInNamespace(j.ctx, j.namespace)
	if err != nil {
		return err
	}

	for _, deployment := range deployments {
		item := DeploymentItem{
			Deployment:          deployment,
			Namespace:           j.namespace,
			OptimizationLoading: true,
			Preferences:         preferences.DefaultDeploymentsPreferences,
			Skipped:             false,
			LazyLoadingEnabled:  false,
		}

		if deployment.Status.AvailableReplicas == 0 {
			item.Skipped = true
			item.SkipReason = "no available replicas"
		}

		j.processor.items.Set(item.GetID(), item)
		j.processor.jobQueue.Push(NewListPodsForDeploymentJob(j.ctx, j.processor, item.GetID()))
	}

	return nil
}
