package daemonsets

import (
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/opengovern/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/opengovern/plugin-kubernetes-internal/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"log"
	"math"
	"strconv"
	"time"
)

type DaemonsetItem struct {
	Daemonset             appv1.DaemonSet
	Pods                  []corev1.Pod
	Namespace             string
	OptimizationLoading   bool
	Preferences           []*golang.PreferenceItem
	Skipped               bool
	LazyLoadingEnabled    bool
	SkipReason            string
	Metrics               map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint // Metric -> Pod -> Container -> Datapoints
	Wastage               *golang2.KubernetesDaemonsetOptimizationResponse
	Nodes                 []shared.KubernetesNode
	ObservabilityDuration time.Duration
	Cost                  float64
	VCpuHoursInPeriod     map[string]map[string]float64 // Pod -> Container -> VCpuHours
	MemoryGBHoursInPeriod map[string]map[string]float64 // Pod -> Container -> MemoryGBHours
}

func (i DaemonsetItem) GetID() string {
	return fmt.Sprintf("appv1.daemonset/%s/%s", i.Daemonset.Namespace, i.Daemonset.Name)
}

func (i DaemonsetItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var rows []*golang.ChartRow
	props := make(map[string]*golang.Properties)

	for _, container := range i.Daemonset.Spec.Template.Spec.Containers {
		var rightSizing *golang2.KubernetesContainerRightsizingRecommendation
		if i.Wastage != nil && i.Wastage.Rightsizing != nil {
			for _, c := range i.Wastage.Rightsizing.ContainerResizing {
				if c.Name == container.Name {
					rightSizing = c
				}
			}
		}

		row, properties := shared.GetContainerDeviceRowAndProperties(container, rightSizing, i.Daemonset.Namespace, i.Daemonset.Name, nil, i.VCpuHoursInPeriod, i.MemoryGBHoursInPeriod, i.Preferences, i.ObservabilityDuration)
		rows = append(rows, row)
		props[row.RowId] = properties
	}

	for _, pod := range i.Pods {
		pod := pod
		var podRs *golang2.KubernetesPodRightsizingRecommendation
		var podRsFound = false
		if i.Wastage != nil && i.Wastage.Rightsizing != nil {
			podRs, podRsFound = i.Wastage.Rightsizing.PodContainerResizing[pod.Name]
		}
		for _, container := range pod.Spec.Containers {
			var rightSizing *golang2.KubernetesContainerRightsizingRecommendation
			if i.Wastage != nil && podRsFound {
				for _, c := range podRs.ContainerResizing {
					if c.Name == container.Name {
						rightSizing = c
					}
				}
			}

			row, properties := shared.GetContainerDeviceRowAndProperties(container, rightSizing, i.Daemonset.Namespace, i.Daemonset.Name, &pod.Name, i.VCpuHoursInPeriod, i.MemoryGBHoursInPeriod, i.Preferences, i.ObservabilityDuration)
			rows = append(rows, row)
			props[row.RowId] = properties
		}
	}

	return rows, props
}

func (i DaemonsetItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	cpuRequestNotConfigured, cpuLimitNotConfigured, memoryRequestNotConfigured, memoryLimitNotConfigured := false, false, false, false
	for _, container := range i.Daemonset.Spec.Template.Spec.Containers {
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
	kaytuJson, err := json.Marshal(i)
	if err != nil {
		log.Printf("failed to marshal kaytu json: %v", err)
	}
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
					Value: i.Daemonset.Namespace,
				},
				"name": {
					Value: i.Daemonset.Name,
				},
				"kubernetes_type": {
					Value:     "DaemonSet",
					SortValue: 1,
				},
				"x_kaytu_status": {
					Value: status,
				},
				"x_kaytu_loading": {
					Value: strconv.FormatBool(i.OptimizationLoading),
				},
				"x_kaytu_raw_json": {
					Value: string(kaytuJson),
				},
				"pod_count": {
					Value:     strconv.Itoa(len(i.Pods)),
					SortValue: float64(len(i.Pods)),
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
	if i.Pods == nil {
		oi.OverviewChartRow.Values["pod_count"] = &golang.ChartRowItem{
			Value: "N/A",
		}
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

		cpuRequestChange = cpuRequestChange * float64(i.Daemonset.Status.CurrentNumberScheduled)
		cpuLimitChange = cpuLimitChange * float64(i.Daemonset.Status.CurrentNumberScheduled)
		memoryRequestChange = memoryRequestChange * float64(i.Daemonset.Status.CurrentNumberScheduled)
		memoryLimitChange = memoryLimitChange * float64(i.Daemonset.Status.CurrentNumberScheduled)

		cpuRequestReductionString := shared.SprintfWithStyle("request: %+.2f core", cpuRequestChange, cpuRequestNotConfigured)
		cpuLimitReductionString := shared.SprintfWithStyle("limit: %+.2f core", cpuLimitChange, cpuLimitNotConfigured)
		memoryRequestReductionString := shared.SprintfWithStyle(fmt.Sprintf("request: %s", shared.SizeByte(memoryRequestChange, true)), memoryRequestChange, memoryRequestNotConfigured)
		memoryLimitReductionString := shared.SprintfWithStyle(fmt.Sprintf("limit: %s", shared.SizeByte(memoryLimitChange, true)), memoryLimitChange, memoryLimitNotConfigured)

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
