package deployments

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/view"
)

type DownloadKaytuAgentReportJob struct {
	ctx       context.Context
	processor *Processor
}

func NewDownloadKaytuAgentReportJob(ctx context.Context, processor *Processor) *DownloadKaytuAgentReportJob {
	return &DownloadKaytuAgentReportJob{
		ctx:       ctx,
		processor: processor,
	}
}

func (j *DownloadKaytuAgentReportJob) Id() string {
	return "download_kaytu_agent_report_job"
}
func (j *DownloadKaytuAgentReportJob) Description() string {
	return "Downloading Kaytu Agent report (Kubernetes Deployments)"
}
func (j *DownloadKaytuAgentReportJob) Run() error {
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

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
