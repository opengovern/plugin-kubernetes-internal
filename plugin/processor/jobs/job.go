package jobs

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	corev1 "k8s.io/api/core/v1"
	"sync/atomic"
)

type Processor struct {
	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	items                     util.ConcurrentMap[string, JobItem]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	jobQueue                  *sdk.JobQueue
	lazyloadCounter           *atomic.Uint32
	configuration             *kaytu.Configuration
	client                    golang2.OptimizationClient
	kaytuClient               *kaytuAgent.KaytuAgent
	namespace                 *string
	selector                  string
	nodeSelector              string
	observabilityDays         int
	defaultPreferences        []*golang.PreferenceItem
	schedulingSim             *simulation.SchedulerService

	summary util.ConcurrentMap[string, shared.ResourceSummary]
}

func NewProcessor(processorConf shared.Configuration) *Processor {
	r := &Processor{
		identification:            processorConf.Identification,
		kubernetesProvider:        processorConf.KubernetesProvider,
		prometheusProvider:        processorConf.PrometheusProvider,
		items:                     util.NewConcurrentMap[string, JobItem](),
		publishOptimizationItem:   processorConf.PublishOptimizationItem,
		publishResultSummary:      processorConf.PublishResultSummary,
		publishResultSummaryTable: processorConf.PublishResultSummaryTable,
		jobQueue:                  processorConf.JobQueue,
		lazyloadCounter:           processorConf.LazyloadCounter,
		configuration:             processorConf.Configuration,
		kaytuClient:               processorConf.KaytuClient,
		client:                    processorConf.Client,
		namespace:                 processorConf.Namespace,
		selector:                  processorConf.Selector,
		nodeSelector:              processorConf.NodeSelector,
		observabilityDays:         processorConf.ObservabilityDays,
		defaultPreferences:        processorConf.DefaultPreferences,

		summary: util.NewConcurrentMap[string, shared.ResourceSummary](),
	}
	processorConf.JobQueue.Push(NewListAllNodesJob(r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	v.OptimizationLoading = true
	m.items.Set(id, v)
	v.LazyLoadingEnabled = false
	m.publishOptimizationItem(v.ToOptimizationItem())
	m.jobQueue.Push(NewOptimizeJobJob(m, id))
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}

func (m *Processor) GetSummaryMap() *util.ConcurrentMap[string, shared.ResourceSummary] {
	return &m.summary
}

func (m *Processor) UpdateSummary(itemId string) {
	i, ok := m.items.Get(itemId)
	if ok && i.Wastage != nil {
		cpuRequestChange, totalCpuRequest := 0.0, 0.0
		cpuLimitChange, totalCpuLimit := 0.0, 0.0
		memoryRequestChange, totalMemoryRequest := 0.0, 0.0
		memoryLimitChange, totalMemoryLimit := 0.0, 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			var pContainer corev1.Container
			for _, podContainer := range i.Job.Spec.Template.Spec.Containers {
				if podContainer.Name == container.Name {
					pContainer = podContainer
				}
			}
			cpuRequest, cpuLimit, memoryRequest, memoryLimit := shared.GetContainerRequestLimits(pContainer)
			if container.Current != nil && container.Recommended != nil {
				if cpuRequest != nil {
					totalCpuRequest += container.Current.CpuRequest
					cpuRequestChange += container.Recommended.CpuRequest - container.Current.CpuRequest
				}
				if cpuLimit != nil {
					totalCpuLimit += container.Current.CpuLimit
					cpuLimitChange += container.Recommended.CpuLimit - container.Current.CpuLimit
				}
				if memoryRequest != nil {
					totalMemoryRequest += container.Current.MemoryRequest
					memoryRequestChange += container.Recommended.MemoryRequest - container.Current.MemoryRequest
				}
				if memoryLimit != nil {
					totalMemoryLimit += container.Current.MemoryLimit
					memoryLimitChange += container.Recommended.MemoryLimit - container.Current.MemoryLimit
				}
			}
		}

		js := shared.ResourceSummary{
			ReplicaCount:        1,
			CPURequestChange:    cpuRequestChange,
			TotalCPURequest:     totalCpuRequest,
			CPULimitChange:      cpuLimitChange,
			TotalCPULimit:       totalCpuLimit,
			MemoryRequestChange: memoryRequestChange,
			TotalMemoryRequest:  totalMemoryRequest,
			MemoryLimitChange:   memoryLimitChange,
			TotalMemoryLimit:    totalMemoryLimit,
		}
		if i.Job.Spec.Parallelism != nil {
			js.ReplicaCount = *i.Job.Spec.Parallelism
		}

		m.summary.Set(i.GetID(), js)
		m.schedulingSim.AddJob(i)
		nodes, err := m.schedulingSim.Simulate()
		if err != nil {
			fmt.Println("failed to simulate due to", err)
		} else {
			cpu, memory := 0.0, 0.0
			for _, n := range nodes {
				cpu += n.VCores
				memory += n.Memory
			}
			fmt.Println("savings: cpu:", cpu, "memory:", memory)
		}
	}
	rs, _ := shared.GetAggregatedResultsSummary(&m.summary)
	m.publishResultSummary(rs)
	rst, _ := shared.GetAggregatedResultsSummaryTable(&m.summary)
	m.publishResultSummaryTable(rst)
}

func (m *Processor) SetSchedulingSim(sim *simulation.SchedulerService) {
	m.schedulingSim = sim
}
