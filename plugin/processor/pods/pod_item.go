package pods

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
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
	Metrics             map[string]map[string][]kaytuPrometheus.PromDatapoint // Metric -> Container -> Datapoints
	//Wastage             kaytu.EC2InstanceWastageResponse
}

func (i PodItem) GetID() string {
	return fmt.Sprintf("%s/%s", i.Pod.Namespace, i.Pod.Name)
}

func getContainerRequestLimits(container corev1.Container) (cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64) {
	if container.Resources.Requests.Cpu() != nil {
		v := container.Resources.Requests.Cpu().AsApproximateFloat64()
		if v != 0 {
			cpuRequest = &v
		}
	}
	if container.Resources.Limits.Cpu() != nil {
		v := container.Resources.Limits.Cpu().AsApproximateFloat64()
		if v != 0 {
			cpuLimit = &v
		}
	}
	if container.Resources.Requests.Memory() != nil {
		v := container.Resources.Requests.Memory().AsApproximateFloat64()
		if v != 0 {
			memoryRequest = &v
		}
	}
	if container.Resources.Limits.Memory() != nil {
		v := container.Resources.Limits.Memory().AsApproximateFloat64()
		if v != 0 {
			memoryLimit = &v
		}
	}
	return cpuRequest, cpuLimit, memoryRequest, memoryLimit
}

func (i PodItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var rows []*golang.ChartRow
	props := make(map[string]*golang.Properties)
	for _, container := range i.Pod.Spec.Containers {
		row := golang.ChartRow{
			RowId:  fmt.Sprintf("%s/%s/%s", i.Pod.Namespace, i.Pod.Name, container.Name),
			Values: make(map[string]*golang.ChartRowItem),
		}
		properties := golang.Properties{}

		row.Values["name"] = &golang.ChartRowItem{
			Value: container.Name,
		}
		cpuRequest, cpuLimit, memoryRequest, memoryLimit := getContainerRequestLimits(container)

		cpuRequestProperty := golang.Property{
			Key: "CPU Request",
		}
		if cpuRequest != nil {
			row.Values["current_cpu_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", *cpuRequest),
			}
		}
		properties.Properties = append(properties.Properties, &cpuRequestProperty)

		cpuLimitProperty := golang.Property{
			Key: "CPU Limit",
		}
		if cpuLimit != nil {
			row.Values["current_cpu_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", *cpuLimit),
			}

		}
		properties.Properties = append(properties.Properties, &cpuLimitProperty)

		memoryRequestProperty := golang.Property{
			Key: "Memory Request",
		}
		if memoryRequest != nil {
			row.Values["current_memory_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)),
			}
		}
		properties.Properties = append(properties.Properties, &memoryRequestProperty)

		memoryLimitProperty := golang.Property{
			Key: "Memory Limit",
		}
		if memoryLimit != nil {
			row.Values["current_memory_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)),
			}
		}
		properties.Properties = append(properties.Properties, &memoryLimitProperty)

		rows = append(rows, &row)
		props[row.RowId] = &properties
	}
	return rows, props
}

func (i PodItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	for _, container := range i.Pod.Spec.Containers {
		cReq, cLim, mReq, mLim := getContainerRequestLimits(container)
		if cReq != nil {
			if cpuRequest != nil {
				*cReq = *cpuRequest + *cReq
			}
			cpuRequest = cReq
		}
		if cLim != nil {
			if cpuLimit != nil {
				*cLim = *cpuLimit + *cLim
			}
			cpuLimit = cLim
		}
		if mReq != nil {
			if memoryRequest != nil {
				*mReq = *memoryRequest + *mReq
			}
			memoryRequest = mReq
		}
		if mLim != nil {
			if memoryLimit != nil {
				*mLim = *memoryLimit + *mLim
			}
			memoryLimit = mLim
		}
	}

	deviceRows, deviceProps := i.Devices()

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
		DevicesChartRows:   deviceRows,
		DevicesProperties:  deviceProps,
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
