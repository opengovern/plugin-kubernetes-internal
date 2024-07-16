package shared

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"time"
)

func GetContainerRequestLimits(container corev1.Container) (cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64) {
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

func GetContainerDeviceRowAndProperties(
	container corev1.Container,
	rightSizing *golang2.KubernetesContainerRightsizingRecommendation,
	namespace string,
	name string,
	podName *string,
	vCpuHoursInPeriod map[string]map[string]float64,
	memoryGBHoursInPeriod map[string]map[string]float64,
	preferences []*golang.PreferenceItem,
	observabilityDuration time.Duration,
) (*golang.ChartRow, *golang.Properties) {
	row := golang.ChartRow{
		RowId:  fmt.Sprintf("%s/%s/%s", namespace, name, container.Name),
		Values: make(map[string]*golang.ChartRowItem),
	}
	if podName != nil {
		row.RowId = fmt.Sprintf("%s/%s/%s/%s", namespace, name, *podName, container.Name)
	}
	properties := golang.Properties{}

	if podName == nil {
		row.Values["name"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s - Overall", container.Name),
		}
		properties.Properties = append(properties.Properties, &golang.Property{
			Key:     "Name",
			Current: row.Values["name"].Value,
		})
	} else {
		row.Values["name"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s - %s", container.Name, *podName),
		}
		properties.Properties = append(properties.Properties, &golang.Property{
			Key:     "Container Name",
			Current: container.Name,
		})
		properties.Properties = append(properties.Properties, &golang.Property{
			Key:     "Pod Name",
			Current: *podName,
		})
	}

	cpuRequest, cpuLimit, memoryRequest, memoryLimit := GetContainerRequestLimits(container)

	cpuRequestProperty := golang.Property{
		Key: "CPU Request",
	}
	if cpuRequest != nil {
		row.Values["current_cpu_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f Core", *cpuRequest),
		}
		cpuRequestProperty.Current = fmt.Sprintf("%.2f", *cpuRequest)
	} else {
		cpuRequestProperty.Current = "Not Configured"
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
	} else {
		cpuLimitProperty.Current = "Not Configured"
	}
	properties.Properties = append(properties.Properties, &cpuLimitProperty)

	memoryRequestProperty := golang.Property{
		Key: "Memory Request",
	}
	if memoryRequest != nil {
		row.Values["current_memory_request"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)),
		}
		memoryRequestProperty.Current = SizeByte(*memoryRequest, false)
	} else {
		memoryRequestProperty.Current = "Not Configured"
	}
	properties.Properties = append(properties.Properties, &memoryRequestProperty)

	memoryLimitProperty := golang.Property{
		Key: "Memory Limit",
	}
	if memoryLimit != nil {
		row.Values["current_memory_limit"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)),
		}
		memoryLimitProperty.Current = SizeByte(*memoryLimit, false)
	} else {
		memoryLimitProperty.Current = "Not Configured"
	}
	properties.Properties = append(properties.Properties, &memoryLimitProperty)
	row.Values["current_cpu"] = &golang.ChartRowItem{
		Value: CpuConfiguration(cpuRequest, cpuLimit),
	}
	row.Values["current_memory"] = &golang.ChartRowItem{
		Value: MemoryConfiguration(memoryRequest, memoryLimit),
	}

	if vCpuHoursInPeriod != nil {
		vCpuHours := 0.0
		if podName == nil {
			for _, pod := range vCpuHoursInPeriod {
				vCpuHours += pod[container.Name]
			}
		} else {
			vCpuHours = vCpuHoursInPeriod[*podName][container.Name]
		}
		cpuHourProperty := golang.Property{
			Key:     "vCPU Hours",
			Average: fmt.Sprintf("%.2f vCpuHour", vCpuHours),
		}
		properties.Properties = append(properties.Properties, &cpuHourProperty)
	}

	if memoryGBHoursInPeriod != nil {
		memoryGBHours := 0.0
		if podName == nil {
			for _, pod := range memoryGBHoursInPeriod {
				memoryGBHours += pod[container.Name]
			}
		} else {
			memoryGBHours = memoryGBHoursInPeriod[*podName][container.Name]
		}
		memoryHourProperty := golang.Property{
			Key:     "Memory GB Hours",
			Average: fmt.Sprintf("%.2f GBHour", memoryGBHours),
		}
		properties.Properties = append(properties.Properties, &memoryHourProperty)
	}

	if rightSizing != nil && rightSizing.Recommended != nil {
		cpuRequestProperty.Recommended = fmt.Sprintf("%.2f (%+.2f)", rightSizing.Recommended.CpuRequest, rightSizing.Recommended.CpuRequest-rightSizing.Current.CpuRequest)
		if rightSizing.CpuTrimmedMean != nil {
			cpuRequestProperty.Average = fmt.Sprintf("avg(tm99): %.2f", rightSizing.CpuTrimmedMean.Value)
		}
		var leaveCPULimitEmpty bool
		for _, i := range preferences {
			if i.Key == "LeaveCPULimitEmpty" {
				f, err := strconv.ParseBool(i.Value.GetValue())
				if err == nil {
					leaveCPULimitEmpty = f
				}
			}
		}
		if !leaveCPULimitEmpty {
			cpuLimitProperty.Recommended = fmt.Sprintf("%.2f (%+.2f)", rightSizing.Recommended.CpuLimit, rightSizing.Recommended.CpuLimit-rightSizing.Current.CpuLimit)
		}
		if rightSizing.CpuMax != nil {
			cpuLimitProperty.Average = fmt.Sprintf("max: %.2f", rightSizing.CpuMax.Value)
		}

		memoryRequestProperty.Recommended = fmt.Sprintf("%s (%s)", SizeByte(rightSizing.Recommended.MemoryRequest, false), SizeByte(rightSizing.Recommended.MemoryRequest-rightSizing.Current.MemoryRequest, true))
		if rightSizing.MemoryTrimmedMean != nil {
			memoryRequestProperty.Average = "avg(tm99): " + SizeByte(rightSizing.MemoryTrimmedMean.Value, false)
		}
		memoryLimitProperty.Recommended = fmt.Sprintf("%s (%s)", SizeByte(rightSizing.Recommended.MemoryLimit, false), SizeByte(rightSizing.Recommended.MemoryLimit-rightSizing.Current.MemoryLimit, true))
		if rightSizing.MemoryMax != nil {
			memoryLimitProperty.Average = "max: " + SizeByte(rightSizing.MemoryMax.Value, false)
		}

		row.Values["suggested_cpu"] = &golang.ChartRowItem{
			Value: CpuConfiguration(&rightSizing.Recommended.CpuRequest, &rightSizing.Recommended.CpuLimit),
		}
		row.Values["suggested_memory"] = &golang.ChartRowItem{
			Value: MemoryConfiguration(&rightSizing.Recommended.MemoryRequest, &rightSizing.Recommended.MemoryLimit),
		}
		row.Values["x_kaytu_observability_duration"] = &golang.ChartRowItem{
			Value: observabilityDuration.String(),
		}
	}

	return &row, &properties
}
