package pods

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
)

type PodItem struct {
	Pod                 corev1.Pod
	Namespace           string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string
	//Metrics             map[string][]types2.Datapoint
	//Wastage             kaytu.EC2InstanceWastageResponse
}

func (i PodItem) GetID() string {
	return fmt.Sprintf("%s/%s", i.Pod.Namespace, i.Pod.Name)
}

func (i PodItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	for _, container := range i.Pod.Spec.Containers {
		if container.Resources.Requests.Cpu() != nil {
			v := container.Resources.Requests.Cpu().AsApproximateFloat64()
			if v != 0 {
				if cpuRequest != nil {
					v = *cpuRequest + v
				}
				cpuRequest = &v
			}
		}
		if container.Resources.Limits.Cpu() != nil {
			v := container.Resources.Limits.Cpu().AsApproximateFloat64()
			if v != 0 {
				if cpuLimit != nil {
					v = *cpuLimit + v
				}
				cpuLimit = &v
			}
		}
		if container.Resources.Requests.Memory() != nil {
			v := container.Resources.Requests.Memory().AsApproximateFloat64()
			if v != 0 {
				if memoryRequest != nil {
					v = *memoryRequest + v
				}
				memoryRequest = &v
			}
		}
		if container.Resources.Limits.Memory() != nil {
			v := container.Resources.Limits.Memory().AsApproximateFloat64()
			if v != 0 {
				if memoryLimit != nil {
					v = *memoryLimit + v
				}
				memoryLimit = &v
			}
		}
	}

	oi := &golang.ChartOptimizationItem{
		OverviewChartRow: &golang.ChartRow{
			RowId: i.GetID(),
			Values: map[string]*golang.ChartRowItem{
				"right_arrow": {
					Value: "→",
				},
				"namespace": {
					Value: i.Pod.Namespace,
				},
				"name": {
					Value: i.Pod.Name,
				},
			},
		},
		Preferences:        i.Preferences,
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         nil,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
		Description:        "", // TODO update
		DevicesChartRows:   nil,
		DevicesProperties:  nil,
	}
	if i.SkipReason != "" {
		oi.SkipReason = &wrapperspb.StringValue{Value: i.SkipReason}
	}
	if cpuRequest != nil && *cpuRequest > 0 {
		oi.OverviewChartRow.Values["current_cpu_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f Core", *cpuRequest),
		}
	}
	if cpuLimit != nil && *cpuLimit > 0 {
		oi.OverviewChartRow.Values["current_cpu_limit"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f Core", *cpuLimit),
		}
	}
	if memoryRequest != nil && *memoryRequest > 0 {
		oi.OverviewChartRow.Values["current_memory_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)),
		}
	}
	if memoryLimit != nil && *memoryLimit > 0 {
		oi.OverviewChartRow.Values["current_memory_limit"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)),
		}
	}

	return oi
}
