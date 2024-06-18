package statefulsets

import (
	"context"
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

type Processor struct {
	identification          map[string]string
	kubernetesProvider      *kaytuKubernetes.Kubernetes
	prometheusProvider      *kaytuPrometheus.Prometheus
	items                   util.ConcurrentMap[string, StatefulsetItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	publishResultSummary    func(summary *golang.ResultSummary)
	jobQueue                *sdk.JobQueue
	lazyloadCounter         atomic.Uint32
	configuration           *kaytu.Configuration
	client                  golang2.OptimizationClient
	kaytuClient             *kaytuAgent.KaytuAgent
	namespace               *string
	observabilityDays       int

	summary util.ConcurrentMap[string, StatefulsetSummary]
}

func NewProcessor(ctx context.Context, identification map[string]string, kubernetesProvider *kaytuKubernetes.Kubernetes, prometheusProvider *kaytuPrometheus.Prometheus, kaytuClient *kaytuAgent.KaytuAgent, publishOptimizationItem func(item *golang.ChartOptimizationItem), publishResultSummary func(summary *golang.ResultSummary), jobQueue *sdk.JobQueue, configuration *kaytu.Configuration, client golang2.OptimizationClient, namespace *string, observabilityDays int) *Processor {
	r := &Processor{
		identification:          identification,
		kubernetesProvider:      kubernetesProvider,
		prometheusProvider:      prometheusProvider,
		items:                   util.NewMap[string, StatefulsetItem](),
		publishOptimizationItem: publishOptimizationItem,
		publishResultSummary:    publishResultSummary,
		jobQueue:                jobQueue,
		lazyloadCounter:         atomic.Uint32{},
		configuration:           configuration,
		kaytuClient:             kaytuClient,
		client:                  client,
		namespace:               namespace,
		observabilityDays:       observabilityDays,

		summary: util.NewMap[string, StatefulsetSummary](),
	}
	if kaytuClient.IsEnabled() {
		jobQueue.Push(NewDownloadKaytuAgentReportJob(ctx, r))
	} else {
		jobQueue.Push(NewListAllNamespacesJob(ctx, r))
	}
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	v.OptimizationLoading = true
	m.items.Set(id, v)
	m.jobQueue.Push(NewOptimizeStatefulsetJob(context.Background(), m, id))

	v.LazyLoadingEnabled = false
	m.publishOptimizationItem(v.ToOptimizationItem())
}

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}

type StatefulsetSummary struct {
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
	m.summary.Range(func(key string, item StatefulsetSummary) bool {
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

func (m *Processor) UpdateSummary(itemId string) {
	i, ok := m.items.Get(itemId)
	if ok && i.Wastage != nil {
		cpuRequestChange, totalCpuRequest := 0.0, 0.0
		cpuLimitChange, totalCpuLimit := 0.0, 0.0
		memoryRequestChange, totalMemoryRequest := 0.0, 0.0
		memoryLimitChange, totalMemoryLimit := 0.0, 0.0
		for _, container := range i.Wastage.Rightsizing.ContainerResizing {
			var pContainer corev1.Container
			for _, podContainer := range i.Statefulset.Spec.Template.Spec.Containers {
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

		ss := StatefulsetSummary{
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
		if i.Statefulset.Spec.Replicas != nil {
			ss.ReplicaCount = *i.Statefulset.Spec.Replicas
		}

		m.summary.Set(i.GetID(), ss)
	}
	m.publishResultSummary(m.ResultsSummary())
}
