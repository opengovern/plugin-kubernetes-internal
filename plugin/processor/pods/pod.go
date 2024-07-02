package pods

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
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
	"sync/atomic"
)

type ProcessorMode int

const (
	ProcessorModeAll    ProcessorMode = iota
	ProcessorModeOrphan               // Orphan pods are pods that are not managed by any other kaytu supported resources (e.g. deployments, statefulsets, daemonsets)
)

type Processor struct {
	mode ProcessorMode

	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	items                     util.ConcurrentMap[string, PodItem]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	jobQueue                  *sdk.JobQueue
	lazyloadCounter           *atomic.Uint32
	configuration             *kaytu.Configuration
	client                    golang2.OptimizationClient
	namespace                 *string
	selector                  string
	nodeSelector              string
	observabilityDays         int
	kaytuClient               *kaytuAgent.KaytuAgent
	schedulingSim             *simulation.SchedulerService
	clusterNodes              []shared.KubernetesNode

	summary            util.ConcurrentMap[string, shared.ResourceSummary]
	defaultPreferences []*golang.PreferenceItem
}

func NewProcessor(processorConf shared.Configuration, mode ProcessorMode) *Processor {
	r := &Processor{
		mode:                      mode,
		identification:            processorConf.Identification,
		kubernetesProvider:        processorConf.KubernetesProvider,
		prometheusProvider:        processorConf.PrometheusProvider,
		items:                     util.NewConcurrentMap[string, PodItem](),
		publishOptimizationItem:   processorConf.PublishOptimizationItem,
		publishResultSummary:      processorConf.PublishResultSummary,
		publishResultSummaryTable: processorConf.PublishResultSummaryTable,
		jobQueue:                  processorConf.JobQueue,
		lazyloadCounter:           processorConf.LazyloadCounter,
		configuration:             processorConf.Configuration,
		client:                    processorConf.Client,
		namespace:                 processorConf.Namespace,
		selector:                  processorConf.Selector,
		nodeSelector:              processorConf.NodeSelector,
		observabilityDays:         processorConf.ObservabilityDays,
		kaytuClient:               processorConf.KaytuClient,
		defaultPreferences:        processorConf.DefaultPreferences,

		summary: util.NewConcurrentMap[string, shared.ResourceSummary](),
	}

	processorConf.JobQueue.Push(NewListAllNodesJob(r))
	return r
}

func (m *Processor) ClusterNodes() []shared.KubernetesNode {
	return m.clusterNodes
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	v.OptimizationLoading = true
	m.items.Set(id, v)
	v.LazyLoadingEnabled = false
	m.publishOptimizationItem(v.ToOptimizationItem())
	m.jobQueue.Push(NewOptimizePodJob(m, id))
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return &golang.NonInteractiveExport{
		Csv: m.exportCsv(),
	}
}

func (m *Processor) exportCsv() []*golang.CSVRow {
	headers := []string{
		"Namespace", "Pod Name", "Container Name", "Current CPU Request", "Current CPU Limit", "Current Memory Request",
		"Current Memory Limit",
		"Suggested CPU Request", "Suggested CPU Limit", "Suggested Memory Request", "Suggested Memory Limit",
		"CPU Request Change", "CPU Limit Change", "Memory Request Change", "Memory Limit Change",
		"Justification", "Additional Details",
	}
	var rows []*golang.CSVRow
	rows = append(rows, &golang.CSVRow{Row: headers})

	m.summary.Range(func(name string, value shared.ResourceSummary) bool {
		pod, ok := m.items.Get(name)
		if !ok {
			return true
		}
		for _, container := range pod.Pod.Spec.Containers {
			var row []string

			row = append(row, pod.Pod.Namespace, pod.Pod.Name, container.Name)

			var righSizing *golang2.KubernetesContainerRightsizingRecommendation
			if pod.Wastage != nil {
				for _, c := range pod.Wastage.Rightsizing.ContainerResizing {
					if c.Name == container.Name {
						righSizing = c
					}
				}
			}
			cpuRequest, cpuLimit, memoryRequest, memoryLimit := shared.GetContainerRequestLimits(container)

			if cpuRequest != nil {
				row = append(row, fmt.Sprintf("%.2f Core", *cpuRequest))
			} else {
				row = append(row, "Not configured")
			}
			if cpuLimit != nil {
				row = append(row, fmt.Sprintf("%.2f Core", *cpuLimit))
			} else {
				row = append(row, "Not configured")
			}
			if memoryRequest != nil {
				row = append(row, fmt.Sprintf("%.2f GB", *memoryRequest/(1024*1024*1024)))
			} else {
				row = append(row, "Not configured")
			}
			if memoryLimit != nil {
				row = append(row, fmt.Sprintf("%.2f GB", *memoryLimit/(1024*1024*1024)))
			} else {
				row = append(row, "Not configured")
			}

			var additionalDetails []string
			if righSizing != nil && righSizing.Recommended != nil {
				row = append(row, fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuRequest),
					fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuLimit),
					fmt.Sprintf("%.2f GB", righSizing.Recommended.MemoryRequest/(1024*1024*1024)),
					fmt.Sprintf("%.2f GB", righSizing.Recommended.MemoryLimit/(1024*1024*1024)))

				if cpuRequest != nil {
					row = append(row, fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuRequest-*cpuRequest))
				} else {
					row = append(row, "Not configured")
				}
				if cpuLimit != nil {
					row = append(row, fmt.Sprintf("%.2f Core", righSizing.Recommended.CpuLimit-*cpuLimit))
				} else {
					row = append(row, "Not configured")
				}
				if memoryRequest != nil {
					row = append(row, fmt.Sprintf("%s", shared.SizeByte(righSizing.Recommended.MemoryRequest-*memoryRequest)))
				} else {
					row = append(row, "Not configured")
				}
				if memoryLimit != nil {
					row = append(row, fmt.Sprintf("%s", shared.SizeByte(righSizing.Recommended.MemoryLimit-*memoryLimit)))
				} else {
					row = append(row, "Not configured")
				}

				row = append(row, righSizing.Description)

				cpuTrimmedMean := 0.0
				cpuMax := 0.0
				memoryTrimmedMean := 0.0
				memoryMax := 0.0
				if righSizing.CpuTrimmedMean != nil {
					cpuTrimmedMean = righSizing.CpuTrimmedMean.Value
				}
				if righSizing.CpuMax != nil {
					cpuMax = righSizing.CpuMax.Value
				}
				if righSizing.MemoryTrimmedMean != nil {
					memoryTrimmedMean = righSizing.MemoryTrimmedMean.Value
				}
				if righSizing.MemoryMax != nil {
					memoryMax = righSizing.MemoryMax.Value
				}

				additionalDetails = append(additionalDetails,
					fmt.Sprintf("CPU Usage:: Avg: %s - Max: %s",
						fmt.Sprintf("%.2f", cpuTrimmedMean),
						fmt.Sprintf("%.2f", cpuMax)))
				additionalDetails = append(additionalDetails,
					fmt.Sprintf("Memory Usage:: Avg: %s - Max: %s",
						fmt.Sprintf("%.2f", memoryTrimmedMean/(1024*1024*1024)),
						fmt.Sprintf("%.2f", memoryMax/(1024*1024*1024))))
				row = append(row, strings.Join(additionalDetails, "---"))
			}
			rows = append(rows, &golang.CSVRow{Row: row})
		}
		return true
	})

	return rows
}

func (m *Processor) GetSummaryMap() *util.ConcurrentMap[string, shared.ResourceSummary] {
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
			for _, podContainer := range i.Pod.Spec.Containers {
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

		m.summary.Set(i.GetID(), shared.ResourceSummary{
			ReplicaCount:            1,
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
		})
		if m.schedulingSim != nil {
			for idx, c := range i.Pod.Spec.Containers {
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

						i.Pod.Spec.Containers[idx] = c
					}
				}
			}

			m.schedulingSim.AddPod(i.Pod)
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
