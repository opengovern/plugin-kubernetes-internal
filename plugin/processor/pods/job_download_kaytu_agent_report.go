package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	v1 "k8s.io/api/core/v1"
)

type DownloadKaytuAgentReportJob struct {
	processor *Processor
}

func NewDownloadKaytuAgentReportJob(processor *Processor) *DownloadKaytuAgentReportJob {
	return &DownloadKaytuAgentReportJob{
		processor: processor,
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

		item.Nodes = j.processor.nodeProcessor.GetKubernetesNodes()
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

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
