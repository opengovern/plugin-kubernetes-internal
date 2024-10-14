package daemonsets

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
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
		ID:          "download_kaytu_agent_report_job_kubernetes_daemonsets",
		Description: "Downloading Kaytu Agent report (Kubernetes DaemonSets)",
		MaxRetry:    0,
	}
}
func (j *DownloadKaytuAgentReportJob) Run(ctx context.Context) error {
	report, err := j.processor.kaytuClient.DownloadReport(ctx, "kubernetes-daemonsets")
	if err != nil {
		return err
	}
	var items []view.PluginResult
	err = json.Unmarshal(report, &items)
	if err != nil {
		return err
	}

	for _, i := range items {
		var item DaemonsetItem
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
			if !shared.LabelFilter(j.processor.selector, item.Daemonset.Labels) {
				continue
			}
		}
		if j.processor.nodeSelector != "" {
			if !shared.PodsInNodes(item.Pods, item.Nodes) {
				continue
			}
		}

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
