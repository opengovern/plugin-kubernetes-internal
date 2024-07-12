package daemonsets

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/nodes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sync/atomic"
)

type Processor struct {
	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	items                     utils.ConcurrentMap[string, DaemonsetItem]
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
	clusterNodes              []shared.KubernetesNode

	summary       utils.ConcurrentMap[string, shared.ResourceSummary]
	NodeProcessor *nodes.Processor
}

func NewProcessor(processorConf shared.Configuration, nodeProcessor *nodes.Processor) *Processor {
	r := &Processor{
		identification:            processorConf.Identification,
		kubernetesProvider:        processorConf.KubernetesProvider,
		prometheusProvider:        processorConf.PrometheusProvider,
		items:                     utils.NewConcurrentMap[string, DaemonsetItem](),
		publishOptimizationItem:   processorConf.PublishOptimizationItem,
		publishResultSummary:      processorConf.PublishResultSummary,
		publishResultSummaryTable: processorConf.PublishResultSummaryTable,
		jobQueue:                  processorConf.JobQueue,
		lazyloadCounter:           processorConf.LazyloadCounter,
		configuration:             processorConf.Configuration,
		client:                    processorConf.Client,
		kaytuClient:               processorConf.KaytuClient,
		namespace:                 processorConf.Namespace,
		selector:                  processorConf.Selector,
		nodeSelector:              processorConf.NodeSelector,
		observabilityDays:         processorConf.ObservabilityDays,
		defaultPreferences:        processorConf.DefaultPreferences,
		NodeProcessor:             nodeProcessor,

		summary: utils.NewConcurrentMap[string, shared.ResourceSummary](),
	}
	if nodeProcessor != nil {
		nodeProcessor.GetKubernetesNodes()
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
	m.jobQueue.Push(NewOptimizeDaemonsetJob(m, id))
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}

func (m *Processor) GetSummaryMap() *utils.ConcurrentMap[string, shared.ResourceSummary] {
	return &m.summary
}

func (m *Processor) UpdateSummary(itemId string) {
	var removableNodes []shared.KubernetesNode

	i, ok := m.items.Get(itemId)
	if ok && i.Wastage != nil {
		cpuRequestChange, totalCpuRequest := 0.0, 0.0
		cpuLimitChange, totalCpuLimit := 0.0, 0.0
		memoryRequestChange, totalMemoryRequest := 0.0, 0.0
		memoryLimitChange, totalMemoryLimit := 0.0, 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			var pContainer corev1.Container
			for _, podContainer := range i.Daemonset.Spec.Template.Spec.Containers {
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

		ds := shared.ResourceSummary{
			ReplicaCount:            i.Daemonset.Status.CurrentNumberScheduled,
			CPURequestDownSizing:    min(0, cpuRequestChange),
			CPURequestUpSizing:      max(0, cpuRequestChange),
			TotalCPURequest:         totalCpuRequest,
			CPULimitDownSizing:      min(0, cpuLimitChange),
			CPULimitUpSizing:        max(0, cpuLimitChange),
			TotalCPULimit:           totalCpuLimit,
			MemoryRequestUpSizing:   max(0, memoryRequestChange),
			MemoryRequestDownSizing: min(0, memoryRequestChange),
			TotalMemoryRequest:      totalMemoryRequest,
			MemoryLimitUpSizing:     max(0, memoryLimitChange),
			MemoryLimitDownSizing:   min(0, memoryLimitChange),
			TotalMemoryLimit:        totalMemoryLimit,
		}

		m.summary.Set(i.GetID(), ds)
		if m.schedulingSim != nil {
			i.Daemonset = *i.Daemonset.DeepCopy()
			for idx, c := range i.Daemonset.Spec.Template.Spec.Containers {
				for _, container := range i.Wastage.Rightsizing.ContainerResizing {
					if container.Name != c.Name {
						continue
					}

					if container.Recommended != nil {
						c.Resources.Requests = map[corev1.ResourceName]resource.Quantity{}
						c.Resources.Limits = map[corev1.ResourceName]resource.Quantity{}

						c.Resources.Requests[corev1.ResourceCPU] = resource.Quantity{}
						c.Resources.Requests.Cpu().SetMilli(int64(container.Recommended.CpuRequest * 1000))

						c.Resources.Limits[corev1.ResourceCPU] = resource.Quantity{}
						c.Resources.Limits.Cpu().SetMilli(int64(container.Recommended.CpuLimit * 1000))

						c.Resources.Requests[corev1.ResourceMemory] = resource.Quantity{}
						c.Resources.Requests.Memory().Set(int64(container.Recommended.MemoryRequest))

						c.Resources.Limits[corev1.ResourceMemory] = resource.Quantity{}
						c.Resources.Limits.Memory().Set(int64(container.Recommended.MemoryLimit))

						i.Daemonset.Spec.Template.Spec.Containers[idx] = c
					}
				}
			}

			m.schedulingSim.AddDaemonSet(i.Daemonset)
			nodes, err := m.schedulingSim.Simulate()
			if err != nil {
				fmt.Println("failed to simulate due to", err)
			} else {
				removableNodes = nodes
			}
		}
	}
	rs, _ := shared.GetAggregatedResultsSummary(&m.summary)
	m.publishResultSummary(rs)
	rst, _ := shared.GetAggregatedResultsSummaryTable(&m.summary, m.clusterNodes, removableNodes)
	m.publishResultSummaryTable(rst)
}

func (m *Processor) SetSchedulingSim(sim *simulation.SchedulerService) {
	m.schedulingSim = sim
}
