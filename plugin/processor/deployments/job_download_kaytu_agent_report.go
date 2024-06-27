package deployments

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	v1 "k8s.io/api/core/v1"
)

type DownloadKaytuAgentReportJob struct {
	processor *Processor
	nodes     []v1.Node
}

func NewDownloadKaytuAgentReportJob(processor *Processor, nodes []v1.Node) *DownloadKaytuAgentReportJob {
	return &DownloadKaytuAgentReportJob{
		processor: processor,
		nodes:     nodes,
	}
}
func (j *DownloadKaytuAgentReportJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "download_kaytu_agent_report_job",
		Description: "Downloading Kaytu Agent report (Kubernetes Deployments)",
		MaxRetry:    0,
	}
}
func (j *DownloadKaytuAgentReportJob) Run(ctx context.Context) error {
	report, err := j.processor.kaytuClient.DownloadReport("kubernetes-deployments")
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

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
