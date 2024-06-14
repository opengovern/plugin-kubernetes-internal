package deployments

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"sync"
)

type Processor struct {
	identification          map[string]string
	kubernetesProvider      *kaytuKubernetes.Kubernetes
	prometheusProvider      *kaytuPrometheus.Prometheus
	items                   util.ConcurrentMap[string, DeploymentItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	publishResultSummary    func(summary *golang.ResultSummary)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	lazyloadCounter         *sdk.SafeCounter
	configuration           *kaytu.Configuration
	client                  golang2.OptimizationClient
	namespace               *string

	summary      map[string]DeploymentSummary
	summaryMutex sync.RWMutex
}

func NewProcessor(ctx context.Context, identification map[string]string, kubernetesProvider *kaytuKubernetes.Kubernetes, prometheusProvider *kaytuPrometheus.Prometheus, publishOptimizationItem func(item *golang.ChartOptimizationItem), publishResultSummary func(summary *golang.ResultSummary), kaytuAcccessToken string, jobQueue *sdk.JobQueue, configuration *kaytu.Configuration, client golang2.OptimizationClient, namespace *string) *Processor {
	r := &Processor{
		identification:          identification,
		kubernetesProvider:      kubernetesProvider,
		prometheusProvider:      prometheusProvider,
		items:                   util.NewMap[string, DeploymentItem](),
		publishOptimizationItem: publishOptimizationItem,
		publishResultSummary:    publishResultSummary,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		lazyloadCounter:         &sdk.SafeCounter{},
		configuration:           configuration,
		client:                  client,
		namespace:               namespace,

		summary:      map[string]DeploymentSummary{},
		summaryMutex: sync.RWMutex{},
	}
	jobQueue.Push(NewListAllNamespacesJob(ctx, r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	v.OptimizationLoading = true
	m.items.Set(id, v)
	m.jobQueue.Push(NewOptimizeDeploymentJob(context.Background(), m, id))

	v.LazyLoadingEnabled = false
	m.publishOptimizationItem(v.ToOptimizationItem())
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return &golang.NonInteractiveExport{
		Csv: m.exportCsv(),
	}
}

func (m *Processor) exportCsv() []*golang.CSVRow {
	headers := []string{
		"Namespace", "Deployment Name", "Pod Name", "Container Name",
		"Current CPU Request", "Current CPU Limit", "Current Memory Request", "Current Memory Limit",
		"Suggested CPU Request", "Suggested CPU Limit", "Suggested Memory Request", "Suggested Memory Limit",
		"CPU Request Change", "CPU Limit Change", "Memory Request Change", "Memory Limit Change",
		"Justification", "Additional Details",
	}
	var rows []*golang.CSVRow
	rows = append(rows, &golang.CSVRow{Row: headers})

	for name, _ := range m.summary {
		deployment, ok := m.items.Get(name)
		if !ok {
			continue
		}
		for _, pod := range deployment.Pods {
			for _, container := range pod.Spec.Containers {
				var row []string

				row = append(row, deployment.Deployment.Namespace, deployment.Deployment.Name, pod.Name, container.Name)

				var righSizing *golang2.KubernetesContainerRightsizingRecommendation
				if deployment.Wastage != nil {
					for _, c := range deployment.Wastage.Rightsizing.ContainerResizing {
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

					additionalDetails = append(additionalDetails,
						fmt.Sprintf("CPU Usage:: Avg: %s - Max: %s",
							fmt.Sprintf("%.2f", righSizing.CpuTrimmedMean.Value),
							fmt.Sprintf("%.2f", righSizing.CpuMax.Value)))
					additionalDetails = append(additionalDetails,
						fmt.Sprintf("Memory Usage:: Avg: %s - Max: %s",
							fmt.Sprintf("%.2f", righSizing.MemoryTrimmedMean.Value/(1024*1024*1024)),
							fmt.Sprintf("%.2f", righSizing.MemoryMax.Value/(1024*1024*1024))))
					row = append(row, strings.Join(additionalDetails, "---"))
				}
				rows = append(rows, &golang.CSVRow{Row: row})
			}
		}
	}

	return rows
}

type DeploymentSummary struct {
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
	m.summaryMutex.RLock()
	for _, item := range m.summary {
		cpuRequestChanges += item.CPURequestChange
		cpuLimitChanges += item.CPULimitChange
		memoryRequestChanges += item.MemoryRequestChange
		memoryLimitChanges += item.MemoryLimitChange

		totalCpuRequest += item.TotalCPURequest
		totalCpuLimit += item.TotalCPULimit
		totalMemoryRequest += item.TotalMemoryRequest
		totalMemoryLimit += item.TotalMemoryLimit
	}
	m.summaryMutex.RUnlock()
	summary.Message = fmt.Sprintf("Overall changes: CPU request: %.2f of %.2f core, CPU limit: %.2f of %.2f core, Memory request: %s of %s, Memory limit: %s of %s", cpuRequestChanges, totalCpuRequest, cpuLimitChanges, totalCpuLimit, shared.SizeByte64(memoryRequestChanges), shared.SizeByte64(totalMemoryRequest), shared.SizeByte64(memoryLimitChanges), shared.SizeByte64(totalMemoryLimit))
	return summary
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
			for _, podContainer := range i.Deployment.Spec.Template.Spec.Containers {
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

		m.summaryMutex.Lock()
		m.summary[i.GetID()] = DeploymentSummary{
			CPURequestChange:    cpuRequestChange,
			TotalCPURequest:     totalCpuRequest,
			CPULimitChange:      cpuLimitChange,
			TotalCPULimit:       totalCpuLimit,
			MemoryRequestChange: memoryRequestChange,
			TotalMemoryRequest:  totalMemoryRequest,
			MemoryLimitChange:   memoryLimitChange,
			TotalMemoryLimit:    totalMemoryLimit,
		}
		m.summaryMutex.Unlock()

	}
	m.publishResultSummary(m.ResultsSummary())
}
