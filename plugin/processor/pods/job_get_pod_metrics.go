package pods

import (
	"context"
	"fmt"
)

type GetPodMetricsJob struct {
	ctx       context.Context
	processor *Processor
	pod       PodItem
}

func NewGetPodMetricsJob(ctx context.Context, processor *Processor, pod PodItem) *GetPodMetricsJob {
	return &GetPodMetricsJob{
		ctx:       ctx,
		processor: processor,
		pod:       pod,
	}
}

func (j *GetPodMetricsJob) Id() string {
	return fmt.Sprintf("get_pod_metrics_for_%s_%s", j.pod.Pod.Namespace, j.pod.Pod.Name)
}
func (j *GetPodMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics for pod %s/%s (Kubernetes Pods)", j.pod.Pod.Namespace, j.pod.Pod.Name)
}
func (j *GetPodMetricsJob) Run() error {
	for _, container := range j.pod.Pod.Spec.Containers {
		_, err := j.processor.prometheusProvider.GetCpuMetricsForPodContainer(j.ctx, j.pod.Pod.Namespace, j.pod.Pod.Name, container.Name)
		if err != nil {
			return err
		}

		_, err = j.processor.prometheusProvider.GetMemoryMetricsForPodContainer(j.ctx, j.pod.Pod.Namespace, j.pod.Pod.Name, container.Name)
		if err != nil {
			return err
		}
		// TODO - store metrics
	}
	return nil
}
