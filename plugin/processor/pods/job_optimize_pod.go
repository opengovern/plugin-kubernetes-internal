package pods

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"log"
	"time"
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
	log.Printf("-------- job %s - starting", j.Id())
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

	log.Printf("-------- job %s - populating containers", j.Id())
	for _, container := range item.Pod.Spec.Containers {
		pod.Containers = append(pod.Containers, &golang.KubernetesContainer{
			Name:          container.Name,
			MemoryRequest: container.Resources.Requests.Memory().AsApproximateFloat64(),
			MemoryLimit:   container.Resources.Limits.Memory().AsApproximateFloat64(),
			CpuRequest:    container.Resources.Requests.Cpu().AsApproximateFloat64(),
			CpuLimit:      container.Resources.Limits.Cpu().AsApproximateFloat64(),
		})
	}

	log.Printf("-------- job %s - populating preferences", j.Id())
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

	log.Printf("-------- job %s - sending request", j.Id())
	grpcCtx := metadata.NewOutgoingContext(j.ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, time.Minute)
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
	log.Printf("-------- job %s - request done", j.Id())
	if err != nil {
		return err
	}
	log.Printf("-------- job %s - response received with no error", j.Id())

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

	log.Printf("-------- job %s - updating item", j.Id())
	j.processor.items.Set(item.GetID(), item)

	log.Printf("-------- job %s - publishing optimization item", j.Id())
	j.processor.publishOptimizationItem(item.ToOptimizationItem())

	log.Printf("-------- job %s - updating summary", j.Id())
	j.processor.UpdateSummary(item.GetID())

	log.Printf("-------- job %s - done", j.Id())
	return nil
}
