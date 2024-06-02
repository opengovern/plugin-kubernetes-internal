package pods

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes/plugin/kaytu"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes/utils"
)

type Processor struct {
	kubernetesProvider      *kaytuKubernetes.Kubernetes
	prometheusProvider      *kaytuPrometheus.Prometheus
	items                   util.ConcurrentMap[string, PodItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	lazyloadCounter         *sdk.SafeCounter
	identification          map[string]string
	configuration           *kaytu.Configuration
	client                  golang2.OptimizationClient
}

func NewProcessor(ctx context.Context, kubernetesProvider *kaytuKubernetes.Kubernetes, prometheusProvider *kaytuPrometheus.Prometheus, publishOptimizationItem func(item *golang.ChartOptimizationItem), kaytuAcccessToken string, jobQueue *sdk.JobQueue, configuration *kaytu.Configuration, client golang2.OptimizationClient) *Processor {
	r := &Processor{
		kubernetesProvider:      kubernetesProvider,
		prometheusProvider:      prometheusProvider,
		items:                   util.NewMap[string, PodItem](),
		publishOptimizationItem: publishOptimizationItem,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		lazyloadCounter:         &sdk.SafeCounter{},
		configuration:           configuration,
		client:                  client,
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
