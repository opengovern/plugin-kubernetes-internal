package deployments

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
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
	//Wastage             *golang2.KubernetesPodOptimizationResponse
}

func (i DeploymentItem) GetID() string {
	return fmt.Sprintf("appv1.deployment/%s/%s", i.Deployment.Namespace, i.Deployment.Name)
}

func (i DeploymentItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var cpuRequest, cpuLimit, memoryRequest, memoryLimit *float64
	//var recCpuRequest, recCpuLimit, recMemoryRequest, recMemoryLimit *float64
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

		//var rightSizing *golang2.KubernetesContainerRightsizingRecommendation
		//if i.Wastage != nil {
		//	for _, c := range i.Wastage.Rightsizing.ContainerResizing {
		//		if c.Name == container.Name {
		//			rightSizing = c
		//		}
		//	}
		//}
		//if rightSizing != nil && rightSizing.Recommended != nil {
		//	if recCpuRequest != nil {
		//		*recCpuRequest = *recCpuRequest + rightSizing.Recommended.CpuRequest
		//	} else {
		//		recCpuRequest = &rightSizing.Recommended.CpuRequest
		//	}
		//	if recCpuLimit != nil {
		//		*recCpuLimit = *recCpuLimit + rightSizing.Recommended.CpuLimit
		//	} else {
		//		recCpuLimit = &rightSizing.Recommended.CpuLimit
		//	}
		//	if recMemoryRequest != nil {
		//		*recMemoryRequest = *recMemoryRequest + rightSizing.Recommended.MemoryRequest
		//	} else {
		//		recMemoryRequest = &rightSizing.Recommended.MemoryRequest
		//	}
		//	if recMemoryLimit != nil {
		//		*recMemoryLimit = *recMemoryLimit + rightSizing.Recommended.MemoryLimit
		//	} else {
		//		recMemoryLimit = &rightSizing.Recommended.MemoryLimit
		//	}
		//}
	}

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
			},
		},
		Preferences:        i.Preferences,
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         nil,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
		Description:        "", // TODO update
		//DevicesChartRows:   deviceRows,
		//DevicesProperties:  deviceProps,
	}
	if i.SkipReason != "" {
		oi.SkipReason = &wrapperspb.StringValue{Value: i.SkipReason}
	}

	// TODO show cpu & memory change from wastage response

	return oi
}
