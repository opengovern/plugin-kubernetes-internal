package pods

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	util "github.com/kaytu-io/plugin-kubernetes/utils"
)

type Processor struct {
	kubernetesProvider      *kaytuKubernetes.Kubernetes
	prometheusProvider      *kaytuPrometheus.Prometheus
	items                   util.ConcurrentMap[string, PodItem]
	publishOptimizationItem func(item *golang.OptimizationItem)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	lazyloadCounter         *sdk.SafeCounter
}

func NewProcessor(
	ctx context.Context,
	kubernetesProvider *kaytuKubernetes.Kubernetes,
	prometheusProvider *kaytuPrometheus.Prometheus,
	publishOptimizationItem func(item *golang.OptimizationItem),
	kaytuAcccessToken string,
	jobQueue *sdk.JobQueue,
	lazyloadCounter *sdk.SafeCounter,
) *Processor {
	r := &Processor{
		kubernetesProvider:      kubernetesProvider,
		prometheusProvider:      prometheusProvider,
		items:                   util.NewMap[string, PodItem](),
		publishOptimizationItem: publishOptimizationItem,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		lazyloadCounter:         lazyloadCounter,
	}
	jobQueue.Push(NewListAllNamespacesJob(ctx, r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	m.items.Set(id, v)
	//m.jobQueue.Push(NewOptimizeEC2InstanceJob(m, v))
}
