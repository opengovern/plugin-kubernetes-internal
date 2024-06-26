package pods

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"sync/atomic"
)

const MB = 1024 * 1024

type Processor struct {
	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	items                     util.ConcurrentMap[string, PodItem]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	jobQueue                  *sdk.JobQueue
	lazyloadCounter           atomic.Uint32
	configuration             *kaytu.Configuration
	client                    golang2.OptimizationClient
	namespace                 *string
	observabilityDays         int
	kaytuClient               *kaytuAgent.KaytuAgent

	summary            util.ConcurrentMap[string, PodSummary]
	defaultPreferences []*golang.PreferenceItem
}

func NewProcessor(
	identification map[string]string,
	kubernetesProvider *kaytuKubernetes.Kubernetes,
	prometheusProvider *kaytuPrometheus.Prometheus,
	kaytuClient *kaytuAgent.KaytuAgent,
	publishOptimizationItem func(item *golang.ChartOptimizationItem),
	publishResultSummary func(summary *golang.ResultSummary),
	publishResultSummaryTable func(summary *golang.ResultSummaryTable),
	jobQueue *sdk.JobQueue,
	configuration *kaytu.Configuration,
	client golang2.OptimizationClient,
	namespace *string,
	observabilityDays int,
	defaultPreferences []*golang.PreferenceItem,
) *Processor {
	r := &Processor{
		identification:            identification,
		kubernetesProvider:        kubernetesProvider,
		prometheusProvider:        prometheusProvider,
		items:                     util.NewMap[string, PodItem](),
		publishOptimizationItem:   publishOptimizationItem,
		publishResultSummary:      publishResultSummary,
		publishResultSummaryTable: publishResultSummaryTable,
		jobQueue:                  jobQueue,
		lazyloadCounter:           atomic.Uint32{},
		configuration:             configuration,
		client:                    client,
		namespace:                 namespace,
		observabilityDays:         observabilityDays,
		kaytuClient:               kaytuClient,
		defaultPreferences:        defaultPreferences,

		summary: util.NewMap[string, PodSummary](),
	}

	if kaytuClient.IsEnabled() {
		jobQueue.Push(NewDownloadKaytuAgentReportJob(r))
	} else {
		jobQueue.Push(NewListAllNamespacesJob(r))
	}
	return r
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

	m.summary.Range(func(name string, value PodSummary) bool {
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

func (m *Processor) ResultsSummary() *golang.ResultSummary {
	summary := &golang.ResultSummary{}
	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64

	m.summary.Range(func(key string, item PodSummary) bool {
		cpuRequestChanges += item.CPURequestChange
		cpuLimitChanges += item.CPULimitChange
		memoryRequestChanges += item.MemoryRequestChange
		memoryLimitChanges += item.MemoryLimitChange

		totalCpuRequest += item.TotalCPURequest
		totalCpuLimit += item.TotalCPULimit
		totalMemoryRequest += item.TotalMemoryRequest
		totalMemoryLimit += item.TotalMemoryLimit

		return true
	})

	summary.Message = fmt.Sprintf("Overall changes: CPU request: %.2f of %.2f core, CPU limit: %.2f of %.2f core, Memory request: %s of %s, Memory limit: %s of %s", cpuRequestChanges, totalCpuRequest, cpuLimitChanges, totalCpuLimit, shared.SizeByte64(memoryRequestChanges), shared.SizeByte64(totalMemoryRequest), shared.SizeByte64(memoryLimitChanges), shared.SizeByte64(totalMemoryLimit))
	return summary
}

func (m *Processor) ResultsSummaryTable() *golang.ResultSummaryTable {
	summaryTable := &golang.ResultSummaryTable{}
	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	m.summary.Range(func(key string, item PodSummary) bool {
		cpuRequestChanges += item.CPURequestChange
		cpuLimitChanges += item.CPULimitChange
		memoryRequestChanges += item.MemoryRequestChange
		memoryLimitChanges += item.MemoryLimitChange

		totalCpuRequest += item.TotalCPURequest
		totalCpuLimit += item.TotalCPULimit
		totalMemoryRequest += item.TotalMemoryRequest
		totalMemoryLimit += item.TotalMemoryLimit

		return true
	})
	summaryTable.Headers = []string{"Summary", "Current", "Recommended", "Net Impact", "Change"}
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"CPU Request (Cores)",
			fmt.Sprintf("%.2f Cores", totalCpuRequest),
			fmt.Sprintf("%.2f Cores", totalCpuRequest+cpuRequestChanges),
			fmt.Sprintf("%.2f Cores", cpuRequestChanges),
			fmt.Sprintf("%.2f%%", cpuRequestChanges/totalCpuRequest*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"CPU Limit (Cores)",
			fmt.Sprintf("%.2f Cores", totalCpuLimit),
			fmt.Sprintf("%.2f Cores", totalCpuLimit+cpuLimitChanges),
			fmt.Sprintf("%.2f Cores", cpuLimitChanges),
			fmt.Sprintf("%.2f%%", cpuLimitChanges/totalCpuLimit*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Memory Request",
			shared.SizeByte64(totalMemoryRequest),
			shared.SizeByte64(totalMemoryRequest + memoryRequestChanges),
			shared.SizeByte64(memoryRequestChanges),
			fmt.Sprintf("%.2f%%", memoryRequestChanges/totalMemoryRequest*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Memory Limit",
			shared.SizeByte64(totalMemoryLimit),
			shared.SizeByte64(totalMemoryLimit + memoryLimitChanges),
			shared.SizeByte64(memoryLimitChanges),
			fmt.Sprintf("%.2f%%", memoryLimitChanges/totalMemoryLimit*100.0),
		},
	})
	return summaryTable
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

		m.summary.Set(i.GetID(), PodSummary{
			CPURequestChange:    cpuRequestChange,
			TotalCPURequest:     totalCpuRequest,
			CPULimitChange:      cpuLimitChange,
			TotalCPULimit:       totalCpuLimit,
			MemoryRequestChange: memoryRequestChange,
			TotalMemoryRequest:  totalMemoryRequest,
			MemoryLimitChange:   memoryLimitChange,
			TotalMemoryLimit:    totalMemoryLimit,
		})
	}
	m.publishResultSummary(m.ResultsSummary())
	m.publishResultSummaryTable(m.ResultsSummaryTable())
}
