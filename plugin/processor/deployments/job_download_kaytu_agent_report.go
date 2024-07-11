package deployments

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
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
		ID:          "download_kaytu_agent_report_job_kubernetes_deployments",
		Description: "Downloading Kaytu Agent report (Kubernetes Deployments)",
		MaxRetry:    0,
	}
}
func (j *DownloadKaytuAgentReportJob) Run(ctx context.Context) error {
	report, err := j.processor.kaytuClient.DownloadReport(ctx, "kubernetes-deployments")
	if err != nil {
		return err
	}
	var items []view.PluginResult
	err = json.Unmarshal(report, &items)
	if err != nil {
		return err
	}

	for _, i := range items {
		var item DeploymentItem
		err = json.Unmarshal([]byte(i.Properties["x_kaytu_raw_json"]), &item)
		if err != nil {
			return err
		}

		item.Nodes = j.nodes
		if j.processor.namespace != nil && *j.processor.namespace != "" {
			if item.Namespace != *j.processor.namespace {
				continue
			}
		}
		if j.processor.selector != "" {
			if !shared.LabelFilter(j.processor.selector, item.Deployment.Labels) {
				continue
			}
		}
		if j.processor.nodeSelector != "" {
			if !shared.PodsInNodes(item.Pods, item.Nodes) {
				continue
			}
		}
		nodeCost := map[string]float64{}
		nodeCPU := map[string]float64{}
		nodeMemory := map[string]float64{}
		for _, p := range item.Pods {
			if j.processor.NodeProcessor != nil {
				for _, n := range j.processor.NodeProcessor.GetKubernetesNodes() {
					if n.Name == p.Spec.NodeName {
						if n.Cost != nil {
							nodeCost[p.Name] = *n.Cost
							nodeCPU[p.Name] = n.VCores
							nodeMemory[p.Name] = n.Memory * simulation.GB
						}
						break
					}
				}
			}
		}

		observabilityPeriod := time.Duration(j.processor.observabilityDays*24) * time.Hour
		totalCost := 0.0
		for pod, podMetrics := range item.Metrics["cpu_usage"] {
			if nodeCPU[pod] > 0 {
				for _, containerDatapoints := range podMetrics {
					totalCost += nodeCost[pod] * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeCPU[pod])
				}
			}
		}
		for pod, podMetrics := range item.Metrics["memory_usage"] {
			if nodeMemory[pod] > 0 {
				for _, containerDatapoints := range podMetrics {
					totalCost += nodeCost[pod] * 0.5 * (shared.MetricAverageOverObservabilityPeriod(containerDatapoints, observabilityPeriod) / nodeMemory[pod])
				}
			}
		}
		item.Cost = totalCost

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
