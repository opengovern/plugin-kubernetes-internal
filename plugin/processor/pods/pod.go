package pods

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes/plugin/kaytu"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes/utils"
	"sync"
)

type Processor struct {
	identification          map[string]string
	kubernetesProvider      *kaytuKubernetes.Kubernetes
	prometheusProvider      *kaytuPrometheus.Prometheus
	items                   util.ConcurrentMap[string, PodItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	publishResultSummary    func(summary *golang.ResultSummary)
	summary                 map[string]PodSummary
	summaryMutex            sync.RWMutex
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	lazyloadCounter         *sdk.SafeCounter
	configuration           *kaytu.Configuration
	client                  golang2.OptimizationClient
}

func NewProcessor(ctx context.Context, identification map[string]string, kubernetesProvider *kaytuKubernetes.Kubernetes, prometheusProvider *kaytuPrometheus.Prometheus, publishOptimizationItem func(item *golang.ChartOptimizationItem), publishResultSummary func(summary *golang.ResultSummary), kaytuAcccessToken string, jobQueue *sdk.JobQueue, configuration *kaytu.Configuration, client golang2.OptimizationClient) *Processor {
	r := &Processor{
		identification:          identification,
		kubernetesProvider:      kubernetesProvider,
		prometheusProvider:      prometheusProvider,
		items:                   util.NewMap[string, PodItem](),
		publishOptimizationItem: publishOptimizationItem,
		publishResultSummary:    publishResultSummary,
		summary:                 map[string]PodSummary{},
		summaryMutex:            sync.RWMutex{},
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
	v.OptimizationLoading = true
	m.items.Set(id, v)
	m.jobQueue.Push(NewOptimizePodJob(context.Background(), m, id))
}

func (m *Processor) ResultsSummary() *golang.ResultSummary {
	summary := &golang.ResultSummary{}
	var cpuRequestChanges float64
	var cpuLimitChanges float64
	var memoryRequestChanges float64
	var memoryLimitChanges float64
	for _, item := range m.summary {
		cpuRequestChanges += item.CPURequestChange
		cpuLimitChanges += item.CPULimitChange
		memoryRequestChanges += item.MemoryRequestChange
		memoryLimitChanges += item.MemoryLimitChange
	}
	summary.Message = fmt.Sprintf("Overal changes: CPU request: %.2f core, CPU limit: %.2f core, Memory request: %s, Memory limit: %s", cpuRequestChanges, cpuLimitChanges, SizeByte64(memoryRequestChanges), SizeByte64(memoryLimitChanges))
	return summary
}
