package daemonsets

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
	"sync/atomic"
)

const MB = 1024 * 1024

type Processor struct {
	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	items                     util.ConcurrentMap[string, DaemonsetItem]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	jobQueue                  *sdk.JobQueue
	lazyloadCounter           atomic.Uint32
	configuration             *kaytu.Configuration
	client                    golang2.OptimizationClient
	kaytuClient               *kaytuAgent.KaytuAgent
	namespace                 *string
	observabilityDays         int
	defaultPreferences        []*golang.PreferenceItem

	summary util.ConcurrentMap[string, DaemonsetSummary]
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
		items:                     util.NewMap[string, DaemonsetItem](),
		publishOptimizationItem:   publishOptimizationItem,
		publishResultSummary:      publishResultSummary,
		publishResultSummaryTable: publishResultSummaryTable,
		jobQueue:                  jobQueue,
		lazyloadCounter:           atomic.Uint32{},
		configuration:             configuration,
		client:                    client,
		kaytuClient:               kaytuClient,
		namespace:                 namespace,
		observabilityDays:         observabilityDays,
		defaultPreferences:        defaultPreferences,

		summary: util.NewMap[string, DaemonsetSummary](),
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
	m.jobQueue.Push(NewOptimizeDaemonsetJob(m, id))
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}

type DaemonsetSummary struct {
	ReplicaCount        int32
	CPURequestChange    float64
	TotalCPURequest     float64
	CPULimitChange      float64
	TotalCPULimit       float64
	MemoryRequestChange float64
	TotalMemoryRequest  float64
	MemoryLimitChange   float64
	TotalMemoryLimit    float64
}

func (m *Processor) ResultsSummary() *golang.ResultSummary {
	summary := &golang.ResultSummary{}
	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	m.summary.Range(func(key string, item DaemonsetSummary) bool {
		cpuRequestChanges += item.CPURequestChange * float64(item.ReplicaCount)
		cpuLimitChanges += item.CPULimitChange * float64(item.ReplicaCount)
		memoryRequestChanges += item.MemoryRequestChange * float64(item.ReplicaCount)
		memoryLimitChanges += item.MemoryLimitChange * float64(item.ReplicaCount)

		totalCpuRequest += item.TotalCPURequest * float64(item.ReplicaCount)
		totalCpuLimit += item.TotalCPULimit * float64(item.ReplicaCount)
		totalMemoryRequest += item.TotalMemoryRequest * float64(item.ReplicaCount)
		totalMemoryLimit += item.TotalMemoryLimit * float64(item.ReplicaCount)

		return true
	})
	summary.Message = fmt.Sprintf("Overall changes: CPU request: %.2f of %.2f core, CPU limit: %.2f of %.2f core, Memory request: %s of %s, Memory limit: %s of %s", cpuRequestChanges, totalCpuRequest, cpuLimitChanges, totalCpuLimit, shared.SizeByte64(memoryRequestChanges), shared.SizeByte64(totalMemoryRequest), shared.SizeByte64(memoryLimitChanges), shared.SizeByte64(totalMemoryLimit))
	return summary
}

func (m *Processor) ResultsSummaryTable() *golang.ResultSummaryTable {
	summaryTable := &golang.ResultSummaryTable{}
	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	m.summary.Range(func(key string, item DaemonsetSummary) bool {
		cpuRequestChanges += item.CPURequestChange * float64(item.ReplicaCount)
		cpuLimitChanges += item.CPULimitChange * float64(item.ReplicaCount)
		memoryRequestChanges += item.MemoryRequestChange * float64(item.ReplicaCount)
		memoryLimitChanges += item.MemoryLimitChange * float64(item.ReplicaCount)

		totalCpuRequest += item.TotalCPURequest * float64(item.ReplicaCount)
		totalCpuLimit += item.TotalCPULimit * float64(item.ReplicaCount)
		totalMemoryRequest += item.TotalMemoryRequest * float64(item.ReplicaCount)
		totalMemoryLimit += item.TotalMemoryLimit * float64(item.ReplicaCount)

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

		ds := DaemonsetSummary{
			ReplicaCount:        i.Daemonset.Status.CurrentNumberScheduled,
			CPURequestChange:    cpuRequestChange,
			TotalCPURequest:     totalCpuRequest,
			CPULimitChange:      cpuLimitChange,
			TotalCPULimit:       totalCpuLimit,
			MemoryRequestChange: memoryRequestChange,
			TotalMemoryRequest:  totalMemoryRequest,
			MemoryLimitChange:   memoryLimitChange,
			TotalMemoryLimit:    totalMemoryLimit,
		}

		m.summary.Set(i.GetID(), ds)
	}
	m.publishResultSummary(m.ResultsSummary())
	m.publishResultSummaryTable(m.ResultsSummaryTable())
}
