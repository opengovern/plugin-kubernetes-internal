package shared

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"strings"
)

type ResourceSummary struct {
	ReplicaCount int32

	CPURequestDownSizing float64
	CPURequestUpSizing   float64
	TotalCPURequest      float64

	CPULimitDownSizing float64
	CPULimitUpSizing   float64
	TotalCPULimit      float64

	MemoryRequestDownSizing float64
	MemoryRequestUpSizing   float64
	TotalMemoryRequest      float64

	MemoryLimitDownSizing float64
	MemoryLimitUpSizing   float64
	TotalMemoryLimit      float64
}

func GetAggregatedResultsSummary(processorSummary *utils.ConcurrentMap[string, ResourceSummary]) (*golang.ResultSummary, *ResourceSummary) {
	summary := &golang.ResultSummary{}

	var cpuRequestDownSizing, cpuRequestUpSizing,
		cpuLimitDownSizing, cpuLimitUpSizing,
		memoryRequestDownSizing, memoryRequestUpSizing,
		memoryLimitDownSizing, memoryLimitUpSizing float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	processorSummary.Range(func(key string, item ResourceSummary) bool {
		cpuRequestDownSizing += item.CPURequestDownSizing * float64(item.ReplicaCount)
		cpuRequestUpSizing += item.CPURequestUpSizing * float64(item.ReplicaCount)
		cpuLimitDownSizing += item.CPULimitDownSizing * float64(item.ReplicaCount)
		cpuLimitUpSizing += item.CPULimitUpSizing * float64(item.ReplicaCount)
		memoryRequestDownSizing += item.MemoryRequestDownSizing * float64(item.ReplicaCount)
		memoryRequestUpSizing += item.MemoryRequestUpSizing * float64(item.ReplicaCount)
		memoryLimitDownSizing += item.MemoryLimitDownSizing * float64(item.ReplicaCount)
		memoryLimitUpSizing += item.MemoryLimitUpSizing * float64(item.ReplicaCount)

		totalCpuRequest += item.TotalCPURequest * float64(item.ReplicaCount)
		totalCpuLimit += item.TotalCPULimit * float64(item.ReplicaCount)
		totalMemoryRequest += item.TotalMemoryRequest * float64(item.ReplicaCount)
		totalMemoryLimit += item.TotalMemoryLimit * float64(item.ReplicaCount)

		return true
	})
	resourceSummary := ResourceSummary{
		ReplicaCount:            1,
		CPURequestUpSizing:      cpuRequestUpSizing,
		CPURequestDownSizing:    cpuRequestDownSizing,
		TotalCPURequest:         totalCpuRequest,
		CPULimitUpSizing:        cpuLimitUpSizing,
		CPULimitDownSizing:      cpuLimitDownSizing,
		TotalCPULimit:           totalCpuLimit,
		MemoryRequestUpSizing:   memoryRequestUpSizing,
		MemoryRequestDownSizing: memoryRequestDownSizing,
		TotalMemoryRequest:      totalMemoryRequest,
		MemoryLimitUpSizing:     memoryLimitUpSizing,
		MemoryLimitDownSizing:   memoryLimitDownSizing,
		TotalMemoryLimit:        totalMemoryLimit,
	}
	summary.Message = fmt.Sprintf("Overall changes: CPU request: %.2f of %.2f core, CPU limit: %.2f of %.2f core, Memory request: %s of %s, Memory limit: %s of %s", (cpuRequestUpSizing + cpuRequestDownSizing), totalCpuRequest, (cpuLimitUpSizing + cpuLimitDownSizing), totalCpuLimit, SizeByte64(memoryRequestUpSizing+memoryRequestDownSizing), SizeByte64(totalMemoryRequest), SizeByte64(memoryLimitUpSizing+memoryLimitDownSizing), SizeByte64(totalMemoryLimit))
	return summary, &resourceSummary
}

func GetAggregatedResultsSummaryTable(processorSummary *utils.ConcurrentMap[string, ResourceSummary], cluster, removableNodes, removableNodesPrev []KubernetesNode) (*golang.ResultSummaryTable, *ResourceSummary) {
	summaryTable := &golang.ResultSummaryTable{}
	var cpuRequestDownSizing, cpuRequestUpSizing,
		cpuLimitDownSizing, cpuLimitUpSizing,
		memoryRequestDownSizing, memoryRequestUpSizing,
		memoryLimitDownSizing, memoryLimitUpSizing float64
	var totalCpuRequest, totalCpuLimit, totalMemoryRequest, totalMemoryLimit float64
	processorSummary.Range(func(key string, item ResourceSummary) bool {
		cpuRequestUpSizing += item.CPURequestUpSizing * float64(item.ReplicaCount)
		cpuRequestDownSizing += item.CPURequestDownSizing * float64(item.ReplicaCount)
		cpuLimitUpSizing += item.CPULimitUpSizing * float64(item.ReplicaCount)
		cpuLimitDownSizing += item.CPULimitDownSizing * float64(item.ReplicaCount)
		memoryRequestUpSizing += item.MemoryRequestUpSizing * float64(item.ReplicaCount)
		memoryRequestDownSizing += item.MemoryRequestDownSizing * float64(item.ReplicaCount)
		memoryLimitUpSizing += item.MemoryLimitUpSizing * float64(item.ReplicaCount)
		memoryLimitDownSizing += item.MemoryLimitDownSizing * float64(item.ReplicaCount)

		totalCpuRequest += item.TotalCPURequest * float64(item.ReplicaCount)
		totalCpuLimit += item.TotalCPULimit * float64(item.ReplicaCount)
		totalMemoryRequest += item.TotalMemoryRequest * float64(item.ReplicaCount)
		totalMemoryLimit += item.TotalMemoryLimit * float64(item.ReplicaCount)

		return true
	})
	summaryTable.Headers = []string{"Summary", "Current", "Recommended", "Net Impact (Total)", "Change"}
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"CPU Request (Cores)",
			fmt.Sprintf("%.2f Cores", totalCpuRequest),
			fmt.Sprintf("%.2f Cores", totalCpuRequest+cpuRequestUpSizing+cpuRequestDownSizing),
			fmt.Sprintf("%.2f Cores", cpuRequestUpSizing+cpuRequestDownSizing),
			fmt.Sprintf("%.2f%%", (cpuRequestUpSizing+cpuRequestDownSizing)/totalCpuRequest*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"CPU Limit (Cores)",
			fmt.Sprintf("%.2f Cores", totalCpuLimit),
			fmt.Sprintf("%.2f Cores", totalCpuLimit+cpuLimitUpSizing+cpuLimitDownSizing),
			fmt.Sprintf("%.2f Cores", cpuLimitUpSizing+cpuLimitDownSizing),
			fmt.Sprintf("%.2f%%", (cpuLimitUpSizing+cpuLimitDownSizing)/totalCpuLimit*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Memory Request",
			SizeByte64(totalMemoryRequest),
			SizeByte64(totalMemoryRequest + memoryRequestUpSizing + memoryRequestDownSizing),
			SizeByte64(memoryRequestUpSizing + memoryRequestDownSizing),
			fmt.Sprintf("%.2f%%", (memoryRequestUpSizing+memoryRequestDownSizing)/totalMemoryRequest*100.0),
		},
	})
	summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
		Cells: []string{
			"Memory Limit",
			SizeByte64(totalMemoryLimit),
			SizeByte64(totalMemoryLimit + memoryLimitUpSizing + memoryLimitDownSizing),
			SizeByte64(memoryLimitUpSizing + memoryLimitDownSizing),
			fmt.Sprintf("%.2f%%", (memoryLimitUpSizing+memoryLimitDownSizing)/totalMemoryLimit*100.0),
		},
	})
	var clusterCPU, clusterMemory, clusterCost, reducedCPU, reducedMemory, reducedCost float64
	var hasCost = false
	for _, c := range cluster {
		clusterCPU += c.VCores
		clusterMemory += c.Memory * 1024 * 1024 * 1024
		if c.Cost != nil {
			clusterCost += *c.Cost
			hasCost = true
		}
	}
	nodes := make(map[string]KubernetesNode)
	if hasCost {
		for _, n := range cluster {
			nodes[n.Name] = n
		}
	}
	for _, n := range removableNodes {
		reducedCPU += n.VCores
		reducedMemory += n.Memory * 1024 * 1024 * 1024
		if v := nodes[n.Name]; v.Cost != nil {
			reducedCost += *v.Cost
		}
	}

	if hasCost {
		summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
			Cells: []string{
				"Cluster (Cost)",
				fmt.Sprintf("$%.2f", clusterCost),
				fmt.Sprintf("$%.2f", clusterCost-reducedCost),
				fmt.Sprintf("-$%.2f", reducedCost),
				fmt.Sprintf("%.2f%%", -reducedCost/clusterCost*100.0),
			},
		})
		summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
			Cells: []string{
				"Cluster (Nodes)",
				nodeListToString(cluster, false),
				nodeListToString(diff(cluster, removableNodes), false),
				nodeListToString(removableNodes, true),
				fmt.Sprintf("%.2f%%", -float64(len(removableNodes))/float64(len(cluster))*100.0),
			},
		})
		for _, n := range removableNodesPrev {
			summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
				Cells: []string{
					"Removable Nodes in the Current Configuration",
					n.Name,
					"",
					"",
					"",
				},
			})
		}
		for _, n := range removableNodes {
			summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
				Cells: []string{
					"Removable Nodes after implementing Optimization",
					n.Name,
					"",
					"",
					"",
				},
			})
		}
	} else {
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
		summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
			Cells: []string{
				"Cluster (Nodes)",
				nodeListToString(cluster, false),
				nodeListToString(diff(cluster, removableNodes), false),
				nodeListToString(removableNodes, true),
				fmt.Sprintf("%.2f%%", -float64(len(removableNodes))/float64(len(cluster))*100.0),
			},
		})
		for _, n := range removableNodes {
			summaryTable.Message = append(summaryTable.Message, &golang.ResultSummaryTableRow{
				Cells: []string{
					"Removable node",
					n.Name,
					"",
					"",
					"",
				},
			})
		}
	}
	resourceSummary := ResourceSummary{
		ReplicaCount:            1,
		CPURequestUpSizing:      cpuRequestUpSizing,
		CPURequestDownSizing:    cpuRequestDownSizing,
		TotalCPURequest:         totalCpuRequest,
		CPULimitUpSizing:        cpuLimitUpSizing,
		CPULimitDownSizing:      cpuLimitDownSizing,
		TotalCPULimit:           totalCpuLimit,
		MemoryRequestUpSizing:   memoryRequestUpSizing,
		MemoryRequestDownSizing: memoryRequestDownSizing,
		TotalMemoryRequest:      totalMemoryRequest,
		MemoryLimitUpSizing:     memoryLimitUpSizing,
		MemoryLimitDownSizing:   memoryLimitDownSizing,
		TotalMemoryLimit:        totalMemoryLimit,
	}

	return summaryTable, &resourceSummary
}

func diff(nodes, remove []KubernetesNode) []KubernetesNode {
	var res []KubernetesNode
	for _, n := range nodes {
		shouldRemove := false
		for _, r := range remove {
			if n.Name == r.Name {
				shouldRemove = true
				break
			}
		}

		if !shouldRemove {
			res = append(res, n)
		}
	}
	return res
}

func nodeListToString(nodes []KubernetesNode, removing bool) string {
	nodeTypeCount := map[string]int{}
	for _, c := range nodes {
		if l, ok := c.Labels[v1.LabelInstanceType]; ok && len(l) > 0 {
			nodeTypeCount[l]++
		} else if l, ok := c.Labels[v1.LabelInstanceTypeStable]; ok && len(l) > 0 {
			nodeTypeCount[l]++
		}
	}

	var nodePool []string
	for nodeType, count := range nodeTypeCount {
		if removing {
			count = -count
		}
		nodePool = append(nodePool, fmt.Sprintf("%d * %s", count, nodeType))
	}
	return strings.Join(nodePool, " + ")
}
