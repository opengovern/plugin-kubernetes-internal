package deployments

import (
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"log"
	"math"
	"strconv"
	"time"
)

type DeploymentItem struct {
	Deployment                appv1.Deployment
	Pods                      []corev1.Pod
	CurrentReplicaSetName     string
	HistoricalReplicaSetNames []string
	Namespace                 string
	OptimizationLoading       bool
	Preferences               []*golang.PreferenceItem
	Skipped                   bool
	LazyLoadingEnabled        bool
	SkipReason                string
	Metrics                   map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint // Metric -> Pod -> Container -> Datapoints
	Wastage                   *golang2.KubernetesDeploymentOptimizationResponse
	Nodes                     []shared.KubernetesNode
	ObservabilityDuration     time.Duration
	Cost                      float64
	VCpuHoursInPeriod         map[string]map[string]float64 // Pod -> Container -> VCpuHours
	MemoryGBHoursInPeriod     map[string]map[string]float64 // Pod -> Container -> MemoryGBHours
}

func (i DeploymentItem) GetID() string {
	return fmt.Sprintf("appv1.deployment/%s/%s", i.Deployment.Namespace, i.Deployment.Name)
}

func (i DeploymentItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var rows []*golang.ChartRow
	props := make(map[string]*golang.Properties)

	for _, container := range i.Deployment.Spec.Template.Spec.Containers {
		var rightSizing *golang2.KubernetesContainerRightsizingRecommendation
		if i.Wastage != nil && i.Wastage.Rightsizing != nil {
			for _, c := range i.Wastage.Rightsizing.ContainerResizing {
				if c.Name == container.Name {
					rightSizing = c
				}
			}
		}

		row := golang.ChartRow{
			RowId:  fmt.Sprintf("%s/%s/%s", i.Deployment.Namespace, i.Deployment.Name, container.Name),
			Values: make(map[string]*golang.ChartRowItem),
		}
		properties := golang.Properties{}

		nameProperty := golang.Property{
			Key: "Name",
		}
		row.Values["name"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s - Overall", container.Name),
		}
		nameProperty.Current = row.Values["name"].Value
		properties.Properties = append(properties.Properties, &nameProperty)

		cpuRequest, cpuLimit, memoryRequest, memoryLimit := shared.GetContainerRequestLimits(container)

		cpuRequestProperty := golang.Property{
			Key: "CPU Request",
		}
		if cpuRequest != nil {
			row.Values["current_cpu_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", *cpuRequest),
			}
			cpuRequestProperty.Current = fmt.Sprintf("%.2f", *cpuRequest)
		}
		properties.Properties = append(properties.Properties, &cpuRequestProperty)

		cpuLimitProperty := golang.Property{
			Key: "CPU Limit",
		}
		if cpuLimit != nil {
			row.Values["current_cpu_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", *cpuLimit),
			}
			cpuLimitProperty.Current = fmt.Sprintf("%.2f", *cpuLimit)
		}
		properties.Properties = append(properties.Properties, &cpuLimitProperty)

		memoryRequestProperty := golang.Property{
			Key: "Memory Request",
		}
		if memoryRequest != nil {
			row.Values["current_memory_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)),
			}
			memoryRequestProperty.Current = shared.SizeByte(*memoryRequest)
		}
		properties.Properties = append(properties.Properties, &memoryRequestProperty)

		memoryLimitProperty := golang.Property{
			Key: "Memory Limit",
		}
		if memoryLimit != nil {
			row.Values["current_memory_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)),
			}
			memoryLimitProperty.Current = shared.SizeByte(*memoryLimit)
		}
		properties.Properties = append(properties.Properties, &memoryLimitProperty)
		row.Values["current_cpu"] = &golang.ChartRowItem{
			Value: shared.CpuConfiguration(cpuRequest, cpuLimit),
		}
		row.Values["current_memory"] = &golang.ChartRowItem{
			Value: shared.MemoryConfiguration(memoryRequest, memoryLimit),
		}

		if i.VCpuHoursInPeriod != nil {
			totalVCpuHours := 0.0
			for _, pod := range i.VCpuHoursInPeriod {
				totalVCpuHours += pod[container.Name]
			}
			cpuHourProperty := golang.Property{
				Key:     "vCPU Hours",
				Average: fmt.Sprintf("%.2f vCpuHour", totalVCpuHours),
			}
			properties.Properties = append(properties.Properties, &cpuHourProperty)
		}

		if i.MemoryGBHoursInPeriod != nil {
			totalMemoryGBHours := 0.0
			for _, pod := range i.MemoryGBHoursInPeriod {
				totalMemoryGBHours += pod[container.Name]
			}
			memoryHourProperty := golang.Property{
				Key:     "Memory GB Hours",
				Average: fmt.Sprintf("%.2f GBHour", totalMemoryGBHours),
			}
			properties.Properties = append(properties.Properties, &memoryHourProperty)
		}

		if rightSizing != nil && rightSizing.Recommended != nil {
			cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuRequest)
			if rightSizing.CpuTrimmedMean != nil {
				cpuRequestProperty.Average = fmt.Sprintf("avg(tm99): %.2f", rightSizing.CpuTrimmedMean.Value)
			}

			var leaveCPULimitEmpty bool
			for _, i := range i.Preferences {
				if i.Key == "LeaveCPULimitEmpty" {
					f, err := strconv.ParseBool(i.Value.GetValue())
					if err == nil {
						leaveCPULimitEmpty = f
					}
				}
			}
			if !leaveCPULimitEmpty {
				cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuLimit)
			}

			if rightSizing.CpuMax != nil {
				cpuLimitProperty.Average = fmt.Sprintf("max: %.2f", rightSizing.CpuMax.Value)
			}

			memoryRequestProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryRequest)
			if rightSizing.MemoryTrimmedMean != nil {
				memoryRequestProperty.Average = "avg(tm99): " + shared.SizeByte(rightSizing.MemoryTrimmedMean.Value)
			}
			memoryLimitProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryLimit)
			if rightSizing.MemoryMax != nil {
				memoryLimitProperty.Average = "max: " + shared.SizeByte(rightSizing.MemoryMax.Value)
			}

			row.Values["suggested_cpu"] = &golang.ChartRowItem{
				Value: shared.CpuConfiguration(&rightSizing.Recommended.CpuRequest, &rightSizing.Recommended.CpuLimit),
			}
			row.Values["suggested_memory"] = &golang.ChartRowItem{
				Value: shared.MemoryConfiguration(&rightSizing.Recommended.MemoryRequest, &rightSizing.Recommended.MemoryLimit),
			}
			row.Values["x_kaytu_observability_duration"] = &golang.ChartRowItem{
				Value: i.ObservabilityDuration.String(),
			}
		}

		rows = append(rows, &row)
		props[row.RowId] = &properties
	}

	for _, pod := range i.Pods {
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

			row := golang.ChartRow{
				RowId:  fmt.Sprintf("%s/%s/%s/%s", pod.Namespace, i.Deployment.Name, pod.Name, container.Name),
				Values: make(map[string]*golang.ChartRowItem),
			}
			properties := golang.Properties{}

			row.Values["name"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%s - %s", container.Name, pod.Name),
			}
			properties.Properties = append(properties.Properties, &golang.Property{
				Key:     "Container Name",
				Current: container.Name,
			})
			properties.Properties = append(properties.Properties, &golang.Property{
				Key:     "Pod Name",
				Current: pod.Name,
			})

			cpuRequest, cpuLimit, memoryRequest, memoryLimit := shared.GetContainerRequestLimits(container)

			cpuRequestProperty := golang.Property{
				Key: "CPU Request",
			}
			if cpuRequest != nil {
				row.Values["current_cpu_request"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f Core", *cpuRequest),
				}
				cpuRequestProperty.Current = fmt.Sprintf("%.2f", *cpuRequest)
			}
			properties.Properties = append(properties.Properties, &cpuRequestProperty)

			cpuLimitProperty := golang.Property{
				Key: "CPU Limit",
			}
			if cpuLimit != nil {
				row.Values["current_cpu_limit"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f Core", *cpuLimit),
				}
				cpuLimitProperty.Current = fmt.Sprintf("%.2f", *cpuLimit)
			}
			properties.Properties = append(properties.Properties, &cpuLimitProperty)

			memoryRequestProperty := golang.Property{
				Key: "Memory Request",
			}
			if memoryRequest != nil {
				row.Values["current_memory_request"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)),
				}
				memoryRequestProperty.Current = shared.SizeByte(*memoryRequest)
			}
			properties.Properties = append(properties.Properties, &memoryRequestProperty)

			memoryLimitProperty := golang.Property{
				Key: "Memory Limit",
			}
			if memoryLimit != nil {
				row.Values["current_memory_limit"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)),
				}
				memoryLimitProperty.Current = shared.SizeByte(*memoryLimit)
			}
			properties.Properties = append(properties.Properties, &memoryLimitProperty)
			row.Values["current_cpu"] = &golang.ChartRowItem{
				Value: shared.CpuConfiguration(cpuRequest, cpuLimit),
			}
			row.Values["current_memory"] = &golang.ChartRowItem{
				Value: shared.MemoryConfiguration(memoryRequest, memoryLimit),
			}

			if i.VCpuHoursInPeriod != nil && i.VCpuHoursInPeriod[pod.Name] != nil {
				cpuHourProperty := golang.Property{
					Key:     "vCPU Hours",
					Average: fmt.Sprintf("%.2f vCpuHour", i.VCpuHoursInPeriod[pod.Name][container.Name]),
				}
				properties.Properties = append(properties.Properties, &cpuHourProperty)
			}
			if i.MemoryGBHoursInPeriod != nil && i.MemoryGBHoursInPeriod[pod.Name] != nil {
				memoryHourProperty := golang.Property{
					Key:     "Memory GB Hours",
					Average: fmt.Sprintf("%.2f GBHour", i.MemoryGBHoursInPeriod[pod.Name][container.Name]),
				}
				properties.Properties = append(properties.Properties, &memoryHourProperty)
			}

			if rightSizing != nil && rightSizing.Recommended != nil {
				cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuRequest)
				if rightSizing.CpuTrimmedMean != nil {
					cpuRequestProperty.Average = fmt.Sprintf("avg(tm99): %.2f", rightSizing.CpuTrimmedMean.Value)
				}
				cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuLimit)
				if rightSizing.CpuMax != nil {
					cpuLimitProperty.Average = fmt.Sprintf("max: %.2f", rightSizing.CpuMax.Value)
				}

				memoryRequestProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryRequest)
				if rightSizing.MemoryTrimmedMean != nil {
					memoryRequestProperty.Average = "avg(tm99): " + shared.SizeByte(rightSizing.MemoryTrimmedMean.Value)
				}
				memoryLimitProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryLimit)
				if rightSizing.MemoryMax != nil {
					memoryLimitProperty.Average = "max: " + shared.SizeByte(rightSizing.MemoryMax.Value)
				}

				row.Values["suggested_cpu"] = &golang.ChartRowItem{
					Value: shared.CpuConfiguration(&rightSizing.Recommended.CpuRequest, &rightSizing.Recommended.CpuLimit),
				}
				row.Values["suggested_memory"] = &golang.ChartRowItem{
					Value: shared.MemoryConfiguration(&rightSizing.Recommended.MemoryRequest, &rightSizing.Recommended.MemoryLimit),
				}
				row.Values["x_kaytu_observability_duration"] = &golang.ChartRowItem{
					Value: i.ObservabilityDuration.String(),
				}
			}

			rows = append(rows, &row)
			props[row.RowId] = &properties
		}
	}

	return rows, props
}

func (i DeploymentItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	cpuRequestNotConfigured, cpuLimitNotConfigured, memoryRequestNotConfigured, memoryLimitNotConfigured := false, false, false, false
	for _, container := range i.Deployment.Spec.Template.Spec.Containers {
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
					Value: i.Deployment.Namespace,
				},
				"name": {
					Value: i.Deployment.Name,
				},
				"kubernetes_type": {
					Value:     "Deployment",
					SortValue: 2,
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
		if i.Deployment.Spec.Replicas != nil {
			cpuRequestChange = cpuRequestChange * float64(*i.Deployment.Spec.Replicas)
			cpuLimitChange = cpuLimitChange * float64(*i.Deployment.Spec.Replicas)
			memoryRequestChange = memoryRequestChange * float64(*i.Deployment.Spec.Replicas)
			memoryLimitChange = memoryLimitChange * float64(*i.Deployment.Spec.Replicas)
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
