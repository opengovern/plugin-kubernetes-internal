package pods

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

type OptimizePodJob struct {
	ctx       context.Context
	processor *Processor
	itemId    string
}

func NewOptimizePodJob(ctx context.Context, processor *Processor, item string) *OptimizePodJob {
	return &OptimizePodJob{
		ctx:       ctx,
		processor: processor,
		itemId:    item,
	}
}

func (j *OptimizePodJob) Id() string {
	return fmt.Sprintf("optimize_pod_cluster_%s", j.itemId)
}
func (j *OptimizePodJob) Description() string {
	return fmt.Sprintf("Optimizing pod %s", j.itemId)
}
func (j *OptimizePodJob) Run() error {
	item, ok := j.processor.items.Get(j.itemId)
	if !ok {
		return errors.New("pod not found in items list")
	}
	if item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetPodMetricsJob(j.ctx, j.processor, item.GetID()))
		return nil
	}

	reqID := uuid.New().String()

	pod := golang.KubernetesPod{
		Id:   item.Pod.Name,
		Name: item.Pod.Name,
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
	preferencesMap := map[string]*wrappers.StringValue{}
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

	grpcCtx := metadata.NewOutgoingContext(j.ctx, metadata.Pairs("workspace-name", "kaytu"))
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

	item = PodItem{
		Pod:                 item.Pod,
		Namespace:           item.Namespace,
		LazyLoadingEnabled:  false,
		OptimizationLoading: false,
		Preferences:         item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Wastage:             resp,
	}
	j.processor.items.Set(item.GetID(), item)
	j.processor.publishOptimizationItem(item.ToOptimizationItem())
	item.UpdateSummary(j.processor)
	return nil
}
