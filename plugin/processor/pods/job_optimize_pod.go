package pods

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/kaytu"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
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

	resp, err := kaytu.PodRequest(kaytu.KubernetesPodWastageRequest{
		RequestId:      &reqID,
		CliVersion:     &version.VERSION,
		Identification: j.processor.identification,
		Pod:            j.item.Pod,
		Namespace:      j.item.Namespace,
		Preferences:    preferences.Export(j.item.Preferences),
		Metrics:        j.item.Metrics,
		Loading:        false,
	}, j.processor.kaytuAcccessToken)
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
