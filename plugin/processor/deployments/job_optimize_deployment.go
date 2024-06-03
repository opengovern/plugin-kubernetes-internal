package deployments

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type OptimizeDeploymentJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewOptimizeDeploymentJob(ctx context.Context, processor *Processor, itemId string) *OptimizeDeploymentJob {
	return &OptimizeDeploymentJob{
		ctx:       ctx,
		processor: processor,
		itemId:    itemId,
	}
}

func (j *OptimizeDeploymentJob) Id() string {
	return fmt.Sprintf("optimize_deployment_%s", j.itemId)
}
func (j *OptimizeDeploymentJob) Description() string {
	return fmt.Sprintf("Optimizing deployment %s", j.itemId)
}
func (j *OptimizeDeploymentJob) Run() error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("deployment not found in items list")
	}
	if item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetDeploymentPodMetricsJob(j.ctx, j.processor, item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	deployment := golang.KubernetesDeployment{
		Name:       item.Deployment.Name,
		Containers: nil,
	}
	for _, container := range item.Deployment.Spec.Template.Spec.Containers {
		deployment.Containers = append(deployment.Containers, &golang.KubernetesContainer{
			Name:          container.Name,
			MemoryRequest: container.Resources.Requests.Memory().AsApproximateFloat64(),
			MemoryLimit:   container.Resources.Limits.Memory().AsApproximateFloat64(),
			CpuRequest:    container.Resources.Requests.Cpu().AsApproximateFloat64(),
			CpuLimit:      container.Resources.Limits.Cpu().AsApproximateFloat64(),
		})
	}
	preferencesMap := map[string]*wrappers.StringValue{}
	for k, v := range preferences.Export(item.Preferences) {
		preferencesMap[k] = nil
		if v != nil {
			preferencesMap[k] = wrapperspb.String(*v)
		}
	}
	metrics := make(map[string]*golang.KubernetesPodMetrics)
	for metricId, podMetrics := range item.Metrics {
		for podId, containerMetrics := range podMetrics {
			if metrics[podId] == nil {
				metrics[podId] = &golang.KubernetesPodMetrics{
					Metrics: make(map[string]*golang.KubernetesContainerMetrics),
				}
			}
			for containerId, datapoints := range containerMetrics {
				if metrics[podId].Metrics[containerId] == nil {
					metrics[podId].Metrics[containerId] = &golang.KubernetesContainerMetrics{
						Cpu:    nil,
						Memory: nil,
					}
				}
				v := metrics[podId].Metrics[containerId]
				switch metricId {
				case "cpu_usage":
					for _, dp := range datapoints {
						if v.Cpu == nil {
							v.Cpu = map[string]float64{}
						}
						v.Cpu[dp.Timestamp.Format("2006-01-02 15:04:05")] = dp.Value
					}
				case "memory_usage":
					for _, dp := range datapoints {
						if v.Memory == nil {
							v.Memory = map[string]float64{}
						}
						v.Memory[dp.Timestamp.Format("2006-01-02 15:04:05")] = dp.Value
					}
				}
				metrics[podId].Metrics[containerId] = v
			}
		}
	}

	grpcCtx := metadata.NewOutgoingContext(j.ctx, metadata.Pairs("workspace-name", "kaytu"))
	resp, err := j.processor.client.KubernetesDeploymentOptimization(grpcCtx, &golang.KubernetesDeploymentOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Deployment:     &deployment,
		Namespace:      item.Namespace,
		Preferences:    preferencesMap,
		Metrics:        metrics,
		Loading:        false,
	})
	if err != nil {
		return err
	}

	item.Wastage = resp
	item.OptimizationLoading = false
	item.LazyLoadingEnabled = false
	item.Skipped = false

	j.processor.items.Set(item.GetID(), item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())
	return nil
}
