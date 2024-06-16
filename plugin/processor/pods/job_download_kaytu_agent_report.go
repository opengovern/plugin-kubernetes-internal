package pods

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu/view"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	resource2 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
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
	return "Downloading Kaytu Agent report (Kubernetes Pods)"
}
func (j *DownloadKaytuAgentReportJob) Run() error {
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
		var containers []corev1.Container
		var containerRightsizings []*golang.KubernetesContainerRightsizingRecommendation
		for _, resource := range i.Resources {
			c := corev1.Container{
				Name: resource.Overview["name"],
				Resources: corev1.ResourceRequirements{
					Limits:   map[v1.ResourceName]resource2.Quantity{},
					Requests: map[v1.ResourceName]resource2.Quantity{},
				},
			}
			if resource.Details["cpu_request"].Recommended != "" {
				c.Resources.Requests[v1.ResourceCPU] = parseCPU(resource.Details["cpu_request"].Recommended)
			}
			if resource.Details["memory_request"].Recommended != "" {
				c.Resources.Requests[v1.ResourceMemory] = parseMemory(resource.Details["memory_request"].Recommended)
			}
			if resource.Details["cpu_limit"].Recommended != "" {
				c.Resources.Limits[v1.ResourceCPU] = parseCPU(resource.Details["cpu_limit"].Recommended)
			}
			if resource.Details["memory_limit"].Recommended != "" {
				c.Resources.Limits[v1.ResourceMemory] = parseMemory(resource.Details["memory_limit"].Recommended)
			}
			containers = append(containers, c)

			currentMemoryRequest := parseMemory(resource.Details["memory_request"].Current)
			currentMemoryLimit := parseMemory(resource.Details["memory_limit"].Current)
			currentCPURequest := parseCPU(resource.Details["cpu_request"].Current)
			currentCPULimit := parseCPU(resource.Details["cpu_limit"].Current)

			recommendedMemoryRequest := parseMemory(resource.Details["memory_request"].Recommended)
			recommendedMemoryLimit := parseMemory(resource.Details["memory_limit"].Recommended)
			recommendedCPURequest := parseCPU(resource.Details["cpu_request"].Recommended)
			recommendedCPULimit := parseCPU(resource.Details["cpu_limit"].Recommended)

			avgMemoryRequest := parseMemory(resource.Details["memory_request"].Average)
			avgMemoryLimit := parseMemory(resource.Details["memory_limit"].Average)
			avgCPURequest := parseCPU(resource.Details["cpu_request"].Average)
			avgCPULimit := parseCPU(resource.Details["cpu_limit"].Average)

			containerRightsizings = append(containerRightsizings, &golang.KubernetesContainerRightsizingRecommendation{
				Name: resource.Overview["name"],
				Current: &golang.RightsizingKubernetesContainer{
					Name:          resource.Overview["name"],
					MemoryRequest: currentMemoryRequest.AsApproximateFloat64(),
					MemoryLimit:   currentMemoryLimit.AsApproximateFloat64(),
					CpuRequest:    currentCPURequest.AsApproximateFloat64(),
					CpuLimit:      currentCPULimit.AsApproximateFloat64(),
				},
				Recommended: &golang.RightsizingKubernetesContainer{
					Name:          resource.Overview["name"],
					MemoryRequest: recommendedMemoryRequest.AsApproximateFloat64(),
					MemoryLimit:   recommendedMemoryLimit.AsApproximateFloat64(),
					CpuRequest:    recommendedCPURequest.AsApproximateFloat64(),
					CpuLimit:      recommendedCPULimit.AsApproximateFloat64(),
				},
				MemoryTrimmedMean: wrapperspb.Double(avgMemoryRequest.AsApproximateFloat64()),
				MemoryMax:         wrapperspb.Double(avgMemoryLimit.AsApproximateFloat64()),
				CpuTrimmedMean:    wrapperspb.Double(avgCPURequest.AsApproximateFloat64()),
				CpuMax:            wrapperspb.Double(avgCPULimit.AsApproximateFloat64()),
				Description:       "",
			})
		}

		item := PodItem{
			Pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      i.Properties["name"],
					Namespace: i.Properties["namespace"],
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
			Namespace:           i.Properties["namespace"],
			LazyLoadingEnabled:  false,
			OptimizationLoading: false,
			Preferences:         nil,
			Skipped:             false,
			SkipReason:          "",
			Wastage: &golang.KubernetesPodOptimizationResponse{
				Rightsizing: &golang.KubernetesPodRightsizingRecommendation{
					Name:              i.Properties["name"],
					ContainerResizing: containerRightsizings,
				},
			},
		}
		j.processor.items.Set(item.GetID(), item)
		j.processor.publishOptimizationItem(item.ToOptimizationItem())
		j.processor.UpdateSummary(item.GetID())
	}

	return nil
}
func parseCPU(c string) resource2.Quantity {
	c = strings.TrimSpace(c)
	c = strings.ToLower(c)
	c = strings.TrimSuffix(c, " core")

	q, err := resource2.ParseQuantity(c)
	if err != nil {
		//panic(err)
	}

	return q
}

func parseMemory(c string) resource2.Quantity {
	c = strings.TrimSpace(c)
	c = strings.ReplaceAll(c, " ", "")
	c = strings.ReplaceAll(c, "KiB", "Ki")
	c = strings.ReplaceAll(c, "KB", "K")
	c = strings.ReplaceAll(c, "MiB", "Mi")
	c = strings.ReplaceAll(c, "MB", "M")
	c = strings.ReplaceAll(c, "GiB", "Gi")
	c = strings.ReplaceAll(c, "GB", "G")
	q, err := resource2.ParseQuantity(c)
	if err != nil {
		//panic(err)
	}
	//q.Format = resource2.BinarySI

	return q
}
