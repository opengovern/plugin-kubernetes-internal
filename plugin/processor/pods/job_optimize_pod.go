package pods

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type OptimizePodJob struct {
	ctx       context.Context
	processor *Processor
	item      PodItem
}

func NewOptimizePodJob(ctx context.Context, processor *Processor, item PodItem) *OptimizePodJob {
	return &OptimizePodJob{
		ctx:       ctx,
		processor: processor,
		item:      item,
	}
}

func (j *OptimizePodJob) Id() string {
	return fmt.Sprintf("optimize_pod_cluster_%s", j.item.Pod.Name)
}
func (j *OptimizePodJob) Description() string {
	return fmt.Sprintf("Optimizing %s", j.item.Pod.Name)
}
func (j *OptimizePodJob) Run() error {
	if j.item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetPodMetricsJob(j.ctx, j.processor, j.item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	pod := golang.KubernetesPod{
		Id:   j.item.Pod.Name,
		Name: j.item.Pod.Name,
	}
	for _, container := range j.item.Pod.Spec.Containers {
		pod.Containers = append(pod.Containers, &golang.KubernetesContainer{
			Name:          container.Name,
			MemoryRequest: float32(container.Resources.Requests.Memory().AsApproximateFloat64()),
			MemoryLimit:   float32(container.Resources.Limits.Memory().AsApproximateFloat64()),
			CpuRequest:    float32(container.Resources.Requests.Cpu().AsApproximateFloat64()),
			CpuLimit:      float32(container.Resources.Limits.Cpu().AsApproximateFloat64()),
		})
	}
	preferencesMap := map[string]*wrappers.StringValue{}
	for k, v := range preferences.Export(j.item.Preferences) {
		preferencesMap[k] = nil
		if v != nil {
			preferencesMap[k] = wrapperspb.String(*v)
		}
	}
	metrics := map[string]*golang.KubernetesContainerMetrics{}
	for metricId, containerMetrics := range j.item.Metrics {
		for containerId, datapoints := range containerMetrics {
			if metrics[containerId] == nil {
				metrics[containerId] = &golang.KubernetesContainerMetrics{}
			}

			if metricId == "cpu_usage" {
				v := metrics[containerId]
				for _, dp := range datapoints {
					if v.Cpu == nil {
						v.Cpu = map[string]float32{}
					}
					v.Cpu[dp.Timestamp.Format("2006-05-04 15:02:01")] = float32(dp.Value)
				}
			} else if metricId == "memory_usage" {
				v := metrics[containerId]
				for _, dp := range datapoints {
					if v.Memory == nil {
						v.Memory = map[string]float32{}
					}
					v.Memory[dp.Timestamp.Format("2006-05-04 15:02:01")] = float32(dp.Value)
				}
			}
		}
	}

	grpcCtx := metadata.NewOutgoingContext(j.ctx, metadata.Pairs("workspace-name", "kaytu"))
	resp, err := j.processor.client.KubernetesPodOptimization(grpcCtx, &golang.KubernetesPodOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Pod:            &pod,
		Namespace:      j.item.Namespace,
		Preferences:    preferencesMap,
		Metrics:        metrics,
		Loading:        false,
	})
	if err != nil {
		return err
	}

	j.item = PodItem{
		Pod:                 j.item.Pod,
		Namespace:           j.item.Namespace,
		LazyLoadingEnabled:  false,
		OptimizationLoading: false,
		Preferences:         j.item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Wastage:             resp,
	}
	j.processor.items.Set(j.item.Pod.Name, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	return nil
}
