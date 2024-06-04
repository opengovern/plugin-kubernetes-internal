package deployments

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

type DeploymentItem struct {
	Deployment          appv1.Deployment
	Pods                []corev1.Pod
	Namespace           string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string
	Metrics             map[string]map[string]map[string][]kaytuPrometheus.PromDatapoint // Metric -> Pod -> Container -> Datapoints
	Wastage             *golang2.KubernetesDeploymentOptimizationResponse
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

		row.Values["name"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s Overall", container.Name),
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

		if rightSizing != nil && rightSizing.Recommended != nil {
			cpuRequestProperty.Current = fmt.Sprintf("%.2f", rightSizing.Current.CpuRequest)
			cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuRequest)
			if rightSizing.CpuTrimmedMean != nil {
				cpuRequestProperty.Average = fmt.Sprintf("Avg: %.2f", rightSizing.CpuTrimmedMean.Value)
			}
			cpuLimitProperty.Current = fmt.Sprintf("%.2f", rightSizing.Current.CpuLimit)
			cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuLimit)
			if rightSizing.CpuMax != nil {
				cpuLimitProperty.Average = fmt.Sprintf("Max: %.2f", rightSizing.CpuMax.Value)
			}

			memoryRequestProperty.Current = shared.SizeByte(rightSizing.Current.MemoryRequest)
			memoryRequestProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryRequest)
			if rightSizing.MemoryTrimmedMean != nil {
				memoryRequestProperty.Average = "Avg: " + shared.SizeByte(rightSizing.MemoryTrimmedMean.Value)
			}
			memoryLimitProperty.Current = shared.SizeByte(rightSizing.Current.MemoryLimit)
			memoryLimitProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryLimit)
			if rightSizing.MemoryMax != nil {
				memoryLimitProperty.Average = "Max: " + shared.SizeByte(rightSizing.MemoryMax.Value)
			}

			row.Values["suggested_cpu_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", rightSizing.Recommended.CpuRequest),
			}
			row.Values["suggested_cpu_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f Core", rightSizing.Recommended.CpuLimit),
			}
			row.Values["suggested_memory_request"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", rightSizing.Recommended.MemoryRequest/(1024*1024*1024)),
			}
			row.Values["suggested_memory_limit"] = &golang.ChartRowItem{
				Value: fmt.Sprintf("%.2f GB", rightSizing.Recommended.MemoryLimit/(1024*1024*1024)),
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
				Value: fmt.Sprintf("%s/%s", pod.Name, container.Name),
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

			if rightSizing != nil && rightSizing.Recommended != nil {
				cpuRequestProperty.Current = fmt.Sprintf("%.2f", rightSizing.Current.CpuRequest)
				cpuRequestProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuRequest)
				if rightSizing.CpuTrimmedMean != nil {
					cpuRequestProperty.Average = fmt.Sprintf("Avg: %.2f", rightSizing.CpuTrimmedMean.Value)
				}
				cpuLimitProperty.Current = fmt.Sprintf("%.2f", rightSizing.Current.CpuLimit)
				cpuLimitProperty.Recommended = fmt.Sprintf("%.2f", rightSizing.Recommended.CpuLimit)
				if rightSizing.CpuMax != nil {
					cpuLimitProperty.Average = fmt.Sprintf("Max: %.2f", rightSizing.CpuMax.Value)
				}

				memoryRequestProperty.Current = shared.SizeByte(rightSizing.Current.MemoryRequest)
				memoryRequestProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryRequest)
				if rightSizing.MemoryTrimmedMean != nil {
					memoryRequestProperty.Average = "Avg: " + shared.SizeByte(rightSizing.MemoryTrimmedMean.Value)
				}
				memoryLimitProperty.Current = shared.SizeByte(rightSizing.Current.MemoryLimit)
				memoryLimitProperty.Recommended = shared.SizeByte(rightSizing.Recommended.MemoryLimit)
				if rightSizing.MemoryMax != nil {
					memoryLimitProperty.Average = "Max: " + shared.SizeByte(rightSizing.MemoryMax.Value)
				}

				row.Values["suggested_cpu_request"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f Core", rightSizing.Recommended.CpuRequest),
				}
				row.Values["suggested_cpu_limit"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f Core", rightSizing.Recommended.CpuLimit),
				}
				row.Values["suggested_memory_request"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f GB", rightSizing.Recommended.MemoryRequest/(1024*1024*1024)),
				}
				row.Values["suggested_memory_limit"] = &golang.ChartRowItem{
					Value: fmt.Sprintf("%.2f GB", rightSizing.Recommended.MemoryLimit/(1024*1024*1024)),
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
	var recCpuRequest, recCpuLimit, recMemoryRequest, recMemoryLimit *float64
	for _, container := range i.Deployment.Spec.Template.Spec.Containers {
		cReq, cLim, mReq, mLim := shared.GetContainerRequestLimits(container)
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

		var rightSizing *golang2.KubernetesContainerRightsizingRecommendation
		if i.Wastage != nil {
			for _, c := range i.Wastage.Rightsizing.ContainerResizing {
				if c.Name == container.Name {
					rightSizing = c
				}
			}
		}
		if rightSizing != nil && rightSizing.Recommended != nil {
			if recCpuRequest != nil {
				*recCpuRequest = *recCpuRequest + rightSizing.Recommended.CpuRequest
			} else {
				recCpuRequest = &rightSizing.Recommended.CpuRequest
			}
			if recCpuLimit != nil {
				*recCpuLimit = *recCpuLimit + rightSizing.Recommended.CpuLimit
			} else {
				recCpuLimit = &rightSizing.Recommended.CpuLimit
			}
			if recMemoryRequest != nil {
				*recMemoryRequest = *recMemoryRequest + rightSizing.Recommended.MemoryRequest
			} else {
				recMemoryRequest = &rightSizing.Recommended.MemoryRequest
			}
			if recMemoryLimit != nil {
				*recMemoryLimit = *recMemoryLimit + rightSizing.Recommended.MemoryLimit
			} else {
				recMemoryLimit = &rightSizing.Recommended.MemoryLimit
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
					Value: i.Deployment.Namespace,
				},
				"name": {
					Value: i.Deployment.Name,
				},
				"status": {
					Value: status,
				},
				"loading": {
					Value: strconv.FormatBool(i.OptimizationLoading),
				},
				"pod_count": {
					Value: strconv.Itoa(len(i.Pods)),
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
		oi.OverviewChartRow.Values["cpu_change"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("request: %.2f core, limit: %.2f core", cpuRequestChange, cpuLimitChange),
		}
		oi.OverviewChartRow.Values["memory_change"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("request: %s, limit: %s", shared.SizeByte64(memoryRequestChange), shared.SizeByte64(memoryLimitChange)),
		}
	}

	return oi
}
