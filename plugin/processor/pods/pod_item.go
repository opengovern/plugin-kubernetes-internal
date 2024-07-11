package pods

import (
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	"math"
	"strconv"
	"time"
)

type PodItem struct {
	Pod                   corev1.Pod
	Namespace             string
	OptimizationLoading   bool
	Preferences           []*golang.PreferenceItem
	Skipped               bool
	LazyLoadingEnabled    bool
	SkipReason            string
	Metrics               map[string]map[string][]kaytuPrometheus.PromDatapoint // Metric -> Container -> Datapoints
	ObservabilityDuration time.Duration
	Wastage               *golang2.KubernetesPodOptimizationResponse
	Nodes                 []shared.KubernetesNode
	Cost                  float64
}

func (i PodItem) GetID() string {
	return fmt.Sprintf("corev1.pod/%s/%s", i.Pod.Namespace, i.Pod.Name)
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
		cpuRequest, cpuLimit, memoryRequest, memoryLimit := shared.GetContainerRequestLimits(container)

		cpuRequestProperty := golang.Property{
			Key: "CPU Request",
		}
		if cpuRequest != nil {
			row.Values["current_cpu_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", *cpuRequest),
			}
		} else {
			row.Values["current_cpu_request"] = &golang.ChartRowItem{
				Value: "Not configured",
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
		} else {
			row.Values["current_cpu_limit"] = &golang.ChartRowItem{
				Value: "Not configured",
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
		} else {
			row.Values["current_memory_request"] = &golang.ChartRowItem{
				Value: "Not configured",
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
		} else {
			row.Values["current_memory_limit"] = &golang.ChartRowItem{
				Value: "Not configured",
			}
		}
		properties.Properties = append(properties.Properties, &memoryLimitProperty)

		if righSizing != nil && righSizing.Recommended != nil {
			cpuRequestProperty.Current = fmt.Sprintf("%.2f", righSizing.Current.CpuRequest)
			if cpuRequest == nil {
				cpuRequestProperty.Current = "Not configured"
			}
			cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", righSizing.Recommended.CpuRequest)
			if righSizing.CpuTrimmedMean != nil {
				cpuRequestProperty.Average = fmt.Sprintf("avg(tm99): %.2f", righSizing.CpuTrimmedMean.Value)
			}

			cpuLimitProperty.Current = fmt.Sprintf("%.2f", righSizing.Current.CpuLimit)
			if cpuLimit == nil {
				cpuLimitProperty.Current = "Not configured"
			}
			cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", righSizing.Recommended.CpuLimit)
			if righSizing.CpuMax != nil {
				cpuLimitProperty.Average = fmt.Sprintf("max: %.2f", righSizing.CpuMax.Value)
			}

			memoryRequestProperty.Current = shared.SizeByte(righSizing.Current.MemoryRequest)
			if memoryRequest == nil {
				memoryRequestProperty.Current = "Not configured"
			}
			memoryRequestProperty.Recommended = shared.SizeByte(righSizing.Recommended.MemoryRequest)
			if righSizing.MemoryTrimmedMean != nil {
				memoryRequestProperty.Average = "avg(tm99): " + shared.SizeByte(righSizing.MemoryTrimmedMean.Value)
			}
			memoryLimitProperty.Current = shared.SizeByte(righSizing.Current.MemoryLimit)
			if memoryLimit == nil {
				memoryLimitProperty.Current = "Not configured"
			}
			memoryLimitProperty.Recommended = shared.SizeByte(righSizing.Recommended.MemoryLimit)
			if righSizing.MemoryMax != nil {
				memoryLimitProperty.Average = "max: " + shared.SizeByte(righSizing.MemoryMax.Value)
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
			row.Values["x_kaytu_observability_duration"] = &golang.ChartRowItem{
				Value: i.ObservabilityDuration.String(),
			}
		}

		rows = append(rows, &row)
		props[row.RowId] = &properties
	}
	return rows, props
}

func (i PodItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	cpuRequestNotConfigured, cpuLimitNotConfigured, memoryRequestNotConfigured, memoryLimitNotConfigured := false, false, false, false
	for _, container := range i.Pod.Spec.Containers {
		cReq, cLim, mReq, mLim := shared.GetContainerRequestLimits(container)
		if cReq != nil {
			if cpuRequest != nil {
				*cReq = *cpuRequest + *cReq
			}
			cpuRequest = cReq
		} else {
			cpuRequestNotConfigured = true
		}
		if cLim != nil {
			if cpuLimit != nil {
				*cLim = *cpuLimit + *cLim
			}
			cpuLimit = cLim
		} else {
			cpuLimitNotConfigured = true
		}
		if mReq != nil {
			if memoryRequest != nil {
				*mReq = *memoryRequest + *mReq
			}
			memoryRequest = mReq
		} else {
			memoryRequestNotConfigured = true
		}
		if mLim != nil {
			if memoryLimit != nil {
				*mLim = *memoryLimit + *mLim
			}
			memoryLimit = mLim
		} else {
			memoryLimitNotConfigured = true
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

	metrics := i.Metrics
	i.Metrics = nil
	cost := i.Cost
	if math.IsNaN(cost) {
		i.Cost = 0
	}
	kaytuJson, _ := json.Marshal(i)
	i.Cost = cost
	i.Metrics = metrics
	oi := &golang.ChartOptimizationItem{
		OverviewChartRow: &golang.ChartRow{
			RowId: i.GetID(),
			Values: map[string]*golang.ChartRowItem{
				"x_kaytu_right_arrow": {
					Value: "→",
				},
				"namespace": {
					Value: i.Pod.Namespace,
				},
				"name": {
					Value: i.Pod.Name,
				},
				"kubernetes_type": {
					Value:     "Pod",
					SortValue: 4,
				},
				"pod_count": {
					Value: "1",
				},
				"x_kaytu_raw_json": {
					Value: string(kaytuJson),
				},
				"x_kaytu_status": {
					Value: status,
				},
				"x_kaytu_loading": {
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

	if i.Wastage != nil {
		cpuRequestChange := 0.0
		cpuLimitChange := 0.0
		memoryRequestChange := 0.0
		memoryLimitChange := 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			if container.Current != nil && container.Recommended != nil {
				cpuRequestChange += container.Recommended.CpuRequest - container.Current.CpuRequest
				cpuLimitChange += container.Recommended.CpuLimit - container.Current.CpuLimit
				memoryRequestChange += container.Recommended.MemoryRequest - container.Current.MemoryRequest
				memoryLimitChange += container.Recommended.MemoryLimit - container.Current.MemoryLimit
			}
		}

		cpuRequestReductionString := shared.SprintfWithStyle("request: %.2f core", cpuRequestChange, cpuRequestNotConfigured)
		cpuLimitReductionString := shared.SprintfWithStyle("limit: %.2f core", cpuLimitChange, cpuLimitNotConfigured)
		memoryRequestReductionString := shared.SprintfWithStyle(fmt.Sprintf("request: %s", shared.SizeByte(memoryRequestChange)), memoryRequestChange, memoryRequestNotConfigured)
		memoryLimitReductionString := shared.SprintfWithStyle(fmt.Sprintf("limit: %s", shared.SizeByte(memoryLimitChange)), memoryLimitChange, memoryLimitNotConfigured)

		oi.OverviewChartRow.Values["cpu_change"] = &golang.ChartRowItem{
			Value:     cpuRequestReductionString + ", " + cpuLimitReductionString,
			SortValue: cpuRequestChange,
		}
		oi.OverviewChartRow.Values["memory_change"] = &golang.ChartRowItem{
			Value:     memoryRequestReductionString + ", " + memoryLimitReductionString,
			SortValue: memoryRequestChange,
		}
		oi.OverviewChartRow.Values["cost"] = &golang.ChartRowItem{
			Value:     fmt.Sprintf("$%0.2f", i.Cost),
			SortValue: i.Cost,
		}

	}
	return oi
}
