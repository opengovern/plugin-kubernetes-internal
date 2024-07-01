package shared

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
)

type ResourceSummary struct {
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

func GetAggregatedResultsSummary(processorSummary *util.ConcurrentMap[string, ResourceSummary]) (*golang.ResultSummary, *ResourceSummary) {
	summary := &golang.ResultSummary{}

	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	processorSummary.Range(func(key string, item ResourceSummary) bool {
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
	resourceSummary := ResourceSummary{
		ReplicaCount:        1,
		CPURequestChange:    cpuRequestChanges,
		TotalCPURequest:     totalCpuRequest,
		CPULimitChange:      cpuLimitChanges,
		TotalCPULimit:       totalCpuLimit,
		MemoryRequestChange: memoryRequestChanges,
		TotalMemoryRequest:  totalMemoryRequest,
		MemoryLimitChange:   memoryLimitChanges,
		TotalMemoryLimit:    totalMemoryLimit,
	}
	summary.Message = fmt.Sprintf("Overall changes: CPU request: %.2f of %.2f core, CPU limit: %.2f of %.2f core, Memory request: %s of %s, Memory limit: %s of %s", cpuRequestChanges, totalCpuRequest, cpuLimitChanges, totalCpuLimit, SizeByte64(memoryRequestChanges), SizeByte64(totalMemoryRequest), SizeByte64(memoryLimitChanges), SizeByte64(totalMemoryLimit))
	return summary, &resourceSummary
}

func GetAggregatedResultsSummaryTable(processorSummary *util.ConcurrentMap[string, ResourceSummary], cluster, removableNodes []KubernetesNode) (*golang.ResultSummaryTable, *ResourceSummary) {
	summaryTable := &golang.ResultSummaryTable{}
	var cpuRequestChanges, cpuLimitChanges, memoryRequestChanges, memoryLimitChanges float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	processorSummary.Range(func(key string, item ResourceSummary) bool {
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
			SizeByte64(totalMemoryRequest),
			SizeByte64(totalMemoryRequest + memoryRequestChanges),
			SizeByte64(memoryRequestChanges),
			fmt.Sprintf("%.2f%%", memoryRequestChanges/totalMemoryRequest*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Memory Limit",
			SizeByte64(totalMemoryLimit),
			SizeByte64(totalMemoryLimit + memoryLimitChanges),
			SizeByte64(memoryLimitChanges),
			fmt.Sprintf("%.2f%%", memoryLimitChanges/totalMemoryLimit*100.0),
		},
	})
	var clusterCPU, clusterMemory, reducedCPU, reducedMemory float64
	for _, c := range cluster {
		clusterCPU += c.VCores
		clusterMemory += c.Memory
	}
	for _, n := range removableNodes {
		reducedCPU += n.VCores
		reducedMemory += n.Memory
	}
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Cluster (CPU)",
			fmt.Sprintf("%.2f Cores", clusterCPU),
			fmt.Sprintf("%.2f Cores", clusterCPU-reducedCPU),
			fmt.Sprintf("%.2f Cores", -reducedCPU),
			fmt.Sprintf("%.2f%%", -reducedCPU/clusterCPU*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Cluster (Memory)",
			SizeByte64(clusterMemory),
			SizeByte64(clusterMemory - reducedMemory),
			SizeByte64(-reducedMemory),
			fmt.Sprintf("%.2f%%", -reducedMemory/clusterMemory*100.0),
		},
	})
	resourceSummary := ResourceSummary{
		ReplicaCount:        1,
		CPURequestChange:    cpuRequestChanges,
		TotalCPURequest:     totalCpuRequest,
		CPULimitChange:      cpuLimitChanges,
		TotalCPULimit:       totalCpuLimit,
		MemoryRequestChange: memoryRequestChanges,
		TotalMemoryRequest:  totalMemoryRequest,
		MemoryLimitChange:   memoryLimitChanges,
		TotalMemoryLimit:    totalMemoryLimit,
	}

	return summaryTable, &resourceSummary
}
