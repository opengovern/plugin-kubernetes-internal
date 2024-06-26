package pods

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/view"
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
		ID:          "download_kaytu_agent_report_job",
		Description: "Downloading Kaytu Agent report (Kubernetes Pods)",
		MaxRetry:    0,
	}
}
func (j *DownloadKaytuAgentReportJob) Run(ctx context.Context) error {
	report, err := j.processor.kaytuClient.DownloadReport("kubernetes-pods")
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

		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
