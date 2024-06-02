package pods

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	"strconv"
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
	Wastage             *golang2.KubernetesPodOptimizationResponse
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
		var righSizing *golang2.KubernetesContainerRightsizingRecommendation
		if i.Wastage != nil {
			for _, c := range i.Wastage.Rightsizing.ContainerResizing {
				if c.Name == container.Name {
					righSizing = c
				}
			}
		}

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

		if righSizing != nil && righSizing.Recommended != nil {
			cpuRequestProperty.Current = fmt.Sprintf("%.2f", righSizing.Current.CpuRequest)
			cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", righSizing.Recommended.CpuRequest)
			if righSizing.CpuTrimmedMean != nil {
				cpuRequestProperty.Average = fmt.Sprintf("Avg: %.2f", righSizing.CpuTrimmedMean.Value)
			}
			cpuLimitProperty.Current = fmt.Sprintf("%.2f", righSizing.Current.CpuLimit)
			cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", righSizing.Recommended.CpuLimit)
			if righSizing.CpuMax != nil {
				cpuLimitProperty.Average = fmt.Sprintf("Max: %.2f", righSizing.CpuMax.Value)
			}

			memoryRequestProperty.Current = SizeByte(righSizing.Current.MemoryRequest)
			memoryRequestProperty.Recommended = SizeByte(righSizing.Recommended.MemoryRequest)
			if righSizing.MemoryTrimmedMean != nil {
				memoryRequestProperty.Average = "Avg: " + SizeByte(righSizing.MemoryTrimmedMean.Value)
			}
			memoryLimitProperty.Current = SizeByte(righSizing.Current.MemoryLimit)
			memoryLimitProperty.Recommended = SizeByte(righSizing.Recommended.MemoryLimit)
			if righSizing.MemoryMax != nil {
				memoryLimitProperty.Average = "Max: " + SizeByte(righSizing.MemoryMax.Value)
			}

			row.Values["suggested_cpu_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuRequest),
			}
			row.Values["suggested_cpu_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuLimit),
			}
			row.Values["suggested_memory_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", righSizing.Recommended.MemoryRequest/(1024*1024*1024)),
			}
			row.Values["suggested_memory_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", righSizing.Recommended.MemoryLimit/(1024*1024*1024)),
			}
		}

		rows = append(rows, &row)
		props[row.RowId] = &properties
	}
	return rows, props
}

func SizeByte(v float64) string {
	return SizeByte64(float64(v))
}

func SizeByte64(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-%s", SizeByte64(-v))
	}

	if v < 1024 {
		return fmt.Sprintf("%.0f Bytes", v)
	}
	v = v / 1024
	if v < 1024 {
		return fmt.Sprintf("%.1f KB", v)
	}
	v = v / 1024
	if v < 1024 {
		return fmt.Sprintf("%.1f MB", v)
	}
	v = v / 1024
	return fmt.Sprintf("%.1f GB", v)
}

func (i PodItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	var recCpuRequest, recCpuLimit, recMemoryRequest, recMemoryLimit *float64
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

		var righSizing *golang2.KubernetesContainerRightsizingRecommendation
		if i.Wastage != nil {
			for _, c := range i.Wastage.Rightsizing.ContainerResizing {
				if c.Name == container.Name {
					righSizing = c
				}
			}
		}
		if righSizing != nil && righSizing.Recommended != nil {
			if recCpuRequest != nil {
				*recCpuRequest = *recCpuRequest + righSizing.Recommended.CpuRequest
			} else {
				recCpuRequest = &righSizing.Recommended.CpuRequest
			}
			if recCpuLimit != nil {
				*recCpuLimit = *recCpuLimit + righSizing.Recommended.CpuLimit
			} else {
				recCpuLimit = &righSizing.Recommended.CpuLimit
			}
			if recMemoryRequest != nil {
				*recMemoryRequest = *recMemoryRequest + righSizing.Recommended.MemoryRequest
			} else {
				recMemoryRequest = &righSizing.Recommended.MemoryRequest
			}
			if recMemoryLimit != nil {
				*recMemoryLimit = *recMemoryLimit + righSizing.Recommended.MemoryLimit
			} else {
				recMemoryLimit = &righSizing.Recommended.MemoryLimit
			}
		}
	}

	deviceRows, deviceProps := i.Devices()

	status := ""
	if i.Skipped {
		status = fmt.Sprintf("skipped - %s", i.SkipReason)
	} else if i.LazyLoadingEnabled && !i.OptimizationLoading {
		status = "press enter to load"
	} else if i.OptimizationLoading {
		status = "loading"
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
				"status": {
					Value: status,
				},
				"loading": {
					Value: strconv.FormatBool(i.OptimizationLoading),
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
	if recCpuRequest != nil && *recCpuRequest > 0 {
		oi.OverviewChartRow.Values["suggested_cpu_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f Core", *recCpuRequest),
		}
	}
	if recCpuLimit != nil && *recCpuLimit > 0 {
		oi.OverviewChartRow.Values["suggested_cpu_limit"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f Core", *recCpuLimit),
		}
	}
	if recMemoryRequest != nil && *recMemoryRequest > 0 {
		oi.OverviewChartRow.Values["suggested_memory_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *recMemoryRequest/(1024*1024*1024)),
		}
	}
	if recMemoryLimit != nil && *recMemoryLimit > 0 {
		oi.OverviewChartRow.Values["suggested_memory_limit"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *recMemoryLimit/(1024*1024*1024)),
		}
	}

	if i.Wastage != nil {
		cpuRequestReduction := 0.0
		cpuLimitReduction := 0.0
		memoryRequestReduction := 0.0
		memoryLimitReduction := 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			if container.Current != nil && container.Recommended != nil {
				cpuRequestReduction += float64(container.Current.CpuRequest - container.Recommended.CpuRequest)
				cpuLimitReduction += float64(container.Current.CpuLimit - container.Recommended.CpuLimit)
				memoryRequestReduction += float64(container.Current.MemoryRequest - container.Recommended.MemoryRequest)
				memoryLimitReduction += float64(container.Current.MemoryLimit - container.Recommended.MemoryLimit)
			}
		}
		oi.OverviewChartRow.Values["cpu_reduction"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("request: %.2f core, limit: %.2f core", cpuRequestReduction, cpuLimitReduction),
		}
		oi.OverviewChartRow.Values["memory_reduction"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("request: %s, limit: %s", SizeByte64(memoryRequestReduction), SizeByte64(memoryLimitReduction)),
		}
	}

	return oi
}
