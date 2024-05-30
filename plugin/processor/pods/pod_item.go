package pods

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
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

func (i PodItem) ToOptimizationItem() *golang.OptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	for _, container := range i.Pod.Spec.Containers {
		if container.Resources.Requests.Cpu() != nil {
			v := container.Resources.Requests.Cpu().AsApproximateFloat64()
			if cpuRequest != nil {
				v = *cpuRequest + v
			}
			cpuRequest = &v
		}
		if container.Resources.Limits.Cpu() != nil {
			v := container.Resources.Limits.Cpu().AsApproximateFloat64()
			if cpuLimit != nil {
				v = *cpuLimit + v
			}
			cpuLimit = &v
		}
		if container.Resources.Requests.Memory() != nil {
			v := container.Resources.Requests.Memory().AsApproximateFloat64()
			if memoryRequest != nil {
				v = *memoryRequest + v
			}
			memoryRequest = &v
		}
		if container.Resources.Limits.Memory() != nil {
			v := container.Resources.Limits.Memory().AsApproximateFloat64()
			if memoryLimit != nil {
				v = *memoryLimit + v
			}
			memoryLimit = &v
		}
	}

	reqLimitStr := ""
	if cpuRequest != nil {
		reqLimitStr += fmt.Sprintf("%.2fc ", *cpuRequest)
	} else {
		reqLimitStr += "na "
	}
	if memoryRequest != nil {
		reqLimitStr += fmt.Sprintf("%.2fm ", *memoryRequest/(1024*1024*1024))
	} else {
		reqLimitStr += "na "
	}
	if len(reqLimitStr) > 0 {
		reqLimitStr += "/ "
	}
	if cpuLimit != nil {
		reqLimitStr += fmt.Sprintf("%.2fc", *cpuLimit)
	} else {
		reqLimitStr += "na "
	}
	if memoryLimit != nil {
		reqLimitStr += fmt.Sprintf("%.2fm", *memoryLimit/(1024*1024*1024))
	} else {
		reqLimitStr += "na"
	}

	oi := &golang.OptimizationItem{
		Id:                 i.Pod.Name,
		ResourceType:       reqLimitStr,
		Region:             i.Namespace,
		Devices:            nil,
		Preferences:        i.Preferences,
		Description:        "", // TODO update
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         i.SkipReason,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}

	return oi
}
