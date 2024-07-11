package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
	v1 "k8s.io/api/core/v1"
	"time"
)

type DownloadKaytuAgentReportJob struct {
	processor *Processor
	nodes     []shared.KubernetesNode
}

func NewDownloadKaytuAgentReportJob(processor *Processor, nodes []shared.KubernetesNode) *DownloadKaytuAgentReportJob {
	return &DownloadKaytuAgentReportJob{
		processor: processor,
		nodes:     nodes,
	}
}
func (j *DownloadKaytuAgentReportJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "download_kaytu_agent_report_job_kubernetes_pods",
		Description: "Downloading Kaytu Agent report (Kubernetes Pods)",
		MaxRetry:    0,
	}
}
func (j *DownloadKaytuAgentReportJob) Run(ctx context.Context) error {
	report, err := j.processor.kaytuClient.DownloadReport(ctx, "kubernetes-pods")
	if err != nil {
		return err
	}
	var items []view.PluginResult
	err = json.Unmarshal(report, &items)
	if err != nil {
		return err
	}

	for _, i := range items {
		var item PodItem
		err = json.Unmarshal([]byte(i.Properties["x_kaytu_raw_json"]), &item)
		if err != nil {
			return err
		}

		item.Nodes = j.nodes
		if j.processor.namespace != nil && *j.processor.namespace != "" {
			if item.Namespace != *j.processor.namespace {
				fmt.Println("ignoring by namespace")
				continue
			}
		}
		if j.processor.selector != "" {
			if !shared.LabelFilter(j.processor.selector, item.Pod.Labels) {
				fmt.Println("ignoring by label")
				continue
			}
		}
		if j.processor.nodeSelector != "" {
			if !shared.PodsInNodes([]v1.Pod{item.Pod}, item.Nodes) {
				fmt.Println("ignoring by node")
				continue
			}
		}

		if j.processor.mode == ProcessorModeOrphan {
			isOrphan := true
			for _, owner := range item.Pod.OwnerReferences {
				if owner.Kind == "ReplicaSet" || owner.Kind == "StatefulSet" || owner.Kind == "DaemonSet" || owner.Kind == "Job" {
					isOrphan = false
					break
				}
			}
			if !isOrphan {
				continue
			}
		}

		nodeCost := 0.0
		nodeCPU := 0.0
		nodeMemory := 0.0
		if j.processor.NodeProcessor != nil {
			for _, n := range j.processor.NodeProcessor.GetKubernetesNodes() {
				if n.Name == item.Pod.Spec.NodeName {
					if n.Cost != nil {
						nodeCost = *n.Cost
						nodeCPU = n.VCores
						nodeMemory = n.Memory * simulation.GB
					}
					break
				}
			}
		}

		observabilityPeriod := time.Duration(j.processor.observabilityDays*24) * time.Hour
		totalCost := 0.0
		for _, containerDatapoints := range item.Metrics["cpu_usage"] {
			if nodeCPU > 0 {
				totalCost += nodeCost * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeCPU)
			}
		}
		for _, containerDatapoints := range item.Metrics["memory_usage"] {
			if nodeMemory > 0 {
				totalCost += nodeCost * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeMemory)
			}
		}
		item.Cost = totalCost

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
