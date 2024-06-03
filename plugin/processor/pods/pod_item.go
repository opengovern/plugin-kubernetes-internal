package pods

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

var (
	unchangedStyle     = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeeeee"))
	increaseStyle      = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#00ee00"))
	decreaseStyle      = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#ee0000"))
	notConfiguredStyle = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeee00"))
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

			memoryRequestProperty.Current = shared.SizeByte(righSizing.Current.MemoryRequest)
			memoryRequestProperty.Recommended = shared.SizeByte(righSizing.Recommended.MemoryRequest)
			if righSizing.MemoryTrimmedMean != nil {
				memoryRequestProperty.Average = "Avg: " + shared.SizeByte(righSizing.MemoryTrimmedMean.Value)
			}
			memoryLimitProperty.Current = shared.SizeByte(righSizing.Current.MemoryLimit)
			memoryLimitProperty.Recommended = shared.SizeByte(righSizing.Recommended.MemoryLimit)
			if righSizing.MemoryMax != nil {
				memoryLimitProperty.Average = "Max: " + shared.SizeByte(righSizing.MemoryMax.Value)
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

		cpuRequestReductionString := SprintfWithStyle("request: %.2f core", cpuRequestChange, cpuRequestNotConfigured)
		cpuLimitReductionString := SprintfWithStyle("limit: %.2f core", cpuLimitChange, cpuLimitNotConfigured)
		memoryRequestReductionString := SprintfWithStyle(fmt.Sprintf("request: %s", shared.SizeByte(memoryRequestChange)), memoryRequestChange, memoryRequestNotConfigured)
		memoryLimitReductionString := SprintfWithStyle(fmt.Sprintf("limit: %s", shared.SizeByte(memoryLimitChange)), memoryLimitChange, memoryLimitNotConfigured)

		oi.OverviewChartRow.Values["cpu_change"] = &golang.ChartRowItem{
			Value: cpuRequestReductionString + ", " + cpuLimitReductionString,
		}
		oi.OverviewChartRow.Values["memory_change"] = &golang.ChartRowItem{
			Value: memoryRequestReductionString + ", " + memoryLimitReductionString,
		}

	}
	return oi
}

func SprintfWithStyle(format string, value float64, notConfigured bool) string {
	str := format
	if strings.Contains(format, "%") {
		str = fmt.Sprintf(format, value)
	}
	if notConfigured {
		str = notConfiguredStyle.Render(str)
	} else if value < 0 {
		str = decreaseStyle.Render(str)
	} else if value > 0 {
		str = increaseStyle.Render(str)
	} else {
		str = unchangedStyle.Render(str)
	}
	return str
}

func (i PodItem) UpdateSummary(p *Processor) {
	if i.Wastage != nil {
		cpuRequestChange := 0.0
		cpuLimitChange := 0.0
		memoryRequestChange := 0.0
		memoryLimitChange := 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			if container.Current != nil && container.Recommended != nil {
				cpuRequestChange += float64(container.Recommended.CpuRequest - container.Current.CpuRequest)
				cpuLimitChange += float64(container.Recommended.CpuLimit - container.Current.CpuLimit)
				memoryRequestChange += float64(container.Recommended.MemoryRequest - container.Current.MemoryRequest)
				memoryLimitChange += float64(container.Recommended.MemoryLimit - container.Current.MemoryLimit)
			}
		}

		p.summaryMutex.Lock()
		p.summary[i.GetID()] = PodSummary{
			CPURequestChange:    cpuRequestChange,
			CPULimitChange:      cpuLimitChange,
			MemoryRequestChange: memoryRequestChange,
			MemoryLimitChange:   memoryLimitChange,
		}
		p.summaryMutex.Unlock()

	}
	p.publishResultSummary(p.ResultsSummary())
}
