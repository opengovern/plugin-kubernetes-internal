package statefulsets

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/api/core/v1"
	"time"
)

type OptimizeStatefulsetJob struct {
	processor *Processor
	itemId    string
}

func NewOptimizeStatefulsetJob(processor *Processor, itemId string) *OptimizeStatefulsetJob {
	return &OptimizeStatefulsetJob{
		processor: processor,
		itemId:    itemId,
	}
}
func (j *OptimizeStatefulsetJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_statefulset_%s", j.itemId),
		Description: fmt.Sprintf("Optimizing statefulset %s", j.itemId),
		MaxRetry:    5,
	}
}

func (j *OptimizeStatefulsetJob) Run(ctx context.Context) error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("statefulset not found in items list")
	}
	if item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewListPodsForStatefulsetJob(j.processor, item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	var tolerations []*v1.Toleration
	for _, t := range item.Statefulset.Spec.Template.Spec.Tolerations {
		tolerations = append(tolerations, &t)
	}
	statefulset := golang.KubernetesStatefulset{
		Id:           item.GetID(),
		Name:         item.Statefulset.Name,
		Containers:   nil,
		Replicas:     *item.Statefulset.Spec.Replicas,
		Affinity:     item.Statefulset.Spec.Template.Spec.Affinity,
		NodeSelector: item.Statefulset.Spec.Template.Spec.NodeSelector,
		Tolerations:  tolerations,
		Labels:       item.Statefulset.Labels,
	}
	for _, container := range item.Statefulset.Spec.Template.Spec.Containers {
		statefulset.Containers = append(statefulset.Containers, &golang.KubernetesContainer{
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
	resp, err := j.processor.client.KubernetesStatefulsetOptimization(grpcCtx, &golang.KubernetesStatefulsetOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Statefulset:    &statefulset,
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

	nodeCost := map[string]float64{}
	nodeCPU := map[string]float64{}
	nodeMemory := map[string]float64{}
	for _, p := range item.Pods {
		if j.processor.nodeProcessor != nil {
			for _, n := range j.processor.nodeProcessor.GetKubernetesNodes() {
				if n.Name == p.Spec.NodeName {
					if n.Cost != nil {
						nodeCost[p.Name] = *n.Cost
						nodeCPU[p.Name] = n.VCores
						nodeMemory[p.Name] = n.Memory * simulation.GB
					}
					break
				}
			}
		}
	}

	observabilityPeriod := time.Duration(j.processor.observabilityDays*24) * time.Hour
	totalCost := 0.0
	for pod, podMetrics := range item.Metrics["cpu_usage"] {
		if nodeCPU[pod] > 0 {
			for _, containerDatapoints := range podMetrics {
				totalCost += nodeCost[pod] * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeCPU[pod])
			}
		}
	}
	for pod, podMetrics := range item.Metrics["memory_usage"] {
		if nodeMemory[pod] > 0 {
			for _, containerDatapoints := range podMetrics {
				totalCost += nodeCost[pod] * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeMemory[pod])
			}
		}
	}
	item.Cost = totalCost

	j.processor.items.Set(item.GetID(), item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())
	j.processor.UpdateSummary(item.GetID())
	return nil
}
