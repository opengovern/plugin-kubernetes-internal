package pods

import (
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
