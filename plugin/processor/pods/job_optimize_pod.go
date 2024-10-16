package pods

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/simulation"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	v1 "k8s.io/api/core/v1"
	"time"
)

type OptimizePodJob struct {
	processor *Processor
	itemId    string
}

func NewOptimizePodJob(processor *Processor, item string) *OptimizePodJob {
	return &OptimizePodJob{
		processor: processor,
		itemId:    item,
	}
}
func (j *OptimizePodJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_pod_cluster_%s", j.itemId),
		Description: fmt.Sprintf("Optimizing pod %s", j.itemId),
		MaxRetry:    5,
	}
}

func (j *OptimizePodJob) Run(ctx context.Context) error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("pod not found in items list")
	}
	if item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetPodMetricsJob(j.processor, item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	var tolerations []*v1.Toleration
	for _, t := range item.Pod.Spec.Tolerations {
		tolerations = append(tolerations, &t)
	}
	var ownerRefs []string
	for _, ow := range item.Pod.ObjectMeta.OwnerReferences {
		ownerRefs = append(ownerRefs, ow.Kind)
	}

	pod := golang.KubernetesPod{
		Id:           item.Pod.Name,
		Name:         item.Pod.Name,
		Containers:   nil,
		Affinity:     item.Pod.Spec.Affinity,
		NodeSelector: item.Pod.Spec.NodeSelector,
		Tolerations:  tolerations,
		Labels:       item.Pod.Labels,
		OwnerKind:    ownerRefs,
	}

	for _, container := range item.Pod.Spec.Containers {
		pod.Containers = append(pod.Containers, &golang.KubernetesContainer{
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
	metrics := map[string]*golang.KubernetesContainerMetrics{}
	for metricId, containerMetrics := range item.Metrics {
		for containerId, datapoints := range containerMetrics {
			if metrics[containerId] == nil {
				metrics[containerId] = &golang.KubernetesContainerMetrics{}
			}

			if metricId == "cpu_usage" {
				v := metrics[containerId]
				for _, dp := range datapoints {
					if v.Cpu == nil {
						v.Cpu = map[string]float64{}
					}
					v.Cpu[dp.Timestamp.Format("2006-01-02 15:04:05")] = dp.Value
				}
				metrics[containerId] = v
			} else if metricId == "memory_usage" {
				v := metrics[containerId]
				for _, dp := range datapoints {
					if v.Memory == nil {
						v.Memory = map[string]float64{}
					}
					v.Memory[dp.Timestamp.Format("2006-01-02 15:04:05")] = dp.Value
				}
				metrics[containerId] = v
			}
		}
	}

	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	resp, err := j.processor.client.KubernetesPodOptimization(grpcCtx, &golang.KubernetesPodOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Pod:            &pod,
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

	nodeCost := 0.0
	nodeCPU := 0.0
	nodeMemory := 0.0
	if j.processor.nodeProcessor != nil {
		for _, n := range j.processor.nodeProcessor.GetKubernetesNodes() {
			if n.Name == item.Pod.Spec.NodeName {
				if n.Cost != nil {
					nodeCost = *n.Cost
					nodeCPU = n.VCores
					nodeMemory = n.Memory * simulation.GB
				}
				break
			}
		}
	}

	observabilityPeriod := time.Duration(j.processor.observabilityDays*24) * time.Hour
	totalCost := 0.0
	for containerName, containerDatapoints := range item.Metrics["memory_usage"] {
		v := shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod)
		if nodeMemory > 0 {
			totalCost += nodeCost * 0.5 * (v / nodeMemory)
		}
		if item.MemoryGBHoursInPeriod == nil {
			item.MemoryGBHoursInPeriod = make(map[string]float64)
		}
		item.MemoryGBHoursInPeriod[containerName] = v / simulation.GB * observabilityPeriod.Hours()
	}
	for containerName, containerDatapoints := range item.Metrics["cpu_usage"] {
		v := shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod)
		if nodeCPU > 0 {
			totalCost += nodeCost * 0.5 * (v / nodeCPU)
		}
		if item.VCpuHoursInPeriod == nil {
			item.VCpuHoursInPeriod = make(map[string]float64)
		}
		item.VCpuHoursInPeriod[containerName] = v * observabilityPeriod.Hours()
	}
	item.Cost = totalCost

	j.processor.items.Set(item.GetID(), item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())
	j.processor.UpdateSummary(item.GetID())

	return nil
}
