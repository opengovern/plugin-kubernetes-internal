package daemonsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/api/core/v1"
)

type OptimizeDaemonsetJob struct {
	processor *Processor
	itemId    string
}

func NewOptimizeDaemonsetJob(processor *Processor, itemId string) *OptimizeDaemonsetJob {
	return &OptimizeDaemonsetJob{
		processor: processor,
		itemId:    itemId,
	}
}

func (j *OptimizeDaemonsetJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_daemonset_%s", j.itemId),
		Description: fmt.Sprintf("Optimizing daemonset %s", j.itemId),
		MaxRetry:    3,
	}
}

func (j *OptimizeDaemonsetJob) Run(ctx context.Context) error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("daemonset not found in items list")
	}
	if item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewListPodsForDaemonsetJob(j.processor, item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	var tolerations []*v1.Toleration
	for _, t := range item.Daemonset.Spec.Template.Spec.Tolerations {
		tolerations = append(tolerations, &t)
	}
	daemonset := golang.KubernetesDaemonset{
		Id:           item.GetID(),
		Name:         item.Daemonset.Name,
		Containers:   nil,
		Affinity:     item.Daemonset.Spec.Template.Spec.Affinity,
		NodeSelector: item.Daemonset.Spec.Template.Spec.NodeSelector,
		Tolerations:  tolerations,
	}
	for _, container := range item.Daemonset.Spec.Template.Spec.Containers {
		daemonset.Containers = append(daemonset.Containers, &golang.KubernetesContainer{
			Name:          container.Name,
			MemoryRequest: container.Resources.Requests.Memory().AsApproximateFloat64(),
			MemoryLimit:   container.Resources.Limits.Memory().AsApproximateFloat64(),
			CpuRequest:    container.Resources.Requests.Cpu().AsApproximateFloat64(),
			CpuLimit:      container.Resources.Limits.Cpu().AsApproximateFloat64(),
		})
	}
	preferencesMap := map[string]*wrapperspb.StringValue{}
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

	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	resp, err := j.processor.client.KubernetesDaemonsetOptimization(grpcCtx, &golang.KubernetesDaemonsetOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Daemonset:      &daemonset,
		Namespace:      item.Namespace,
		Preferences:    preferencesMap,
		Metrics:        metrics,
		Loading:        false,
	})
	if err != nil {
		return err
	}

	item.LazyLoadingEnabled = false
	item.OptimizationLoading = false
	item.Skipped = false
	item.SkipReason = ""
	item.Wastage = resp

	j.processor.items.Set(item.GetID(), item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())
	j.processor.UpdateSummary(item.GetID())
	return nil
}
