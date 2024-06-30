package all

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/daemonsets"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/deployments"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/jobs"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/pods"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/statefulsets"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	"sync/atomic"
)

type Processor struct {
	identification            map[string]string
	kubernetesProvider        *kaytuKubernetes.Kubernetes
	prometheusProvider        *kaytuPrometheus.Prometheus
	itemsToProcessor          util.ConcurrentMap[string, string]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	jobQueue                  *sdk.JobQueue
	lazyloadCounter           *atomic.Uint32
	configuration             *kaytu.Configuration
	client                    golang2.OptimizationClient
	kaytuClient               *kaytuAgent.KaytuAgent
	namespace                 *string
	selector                  string
	nodeSelector              string
	observabilityDays         int
	defaultPreferences        []*golang.PreferenceItem

	summary util.ConcurrentMap[string, any]

	daemonsetsProcessor   *daemonsets.Processor
	deploymentsProcessor  *deployments.Processor
	statefulsetsProcessor *statefulsets.Processor
	jobsProcessor         *jobs.Processor
	podsProcessor         *pods.Processor
}

func (p *Processor) publishOptimizationItemFunc(item *golang.ChartOptimizationItem, kuberType string) {
	p.itemsToProcessor.Set(item.OverviewChartRow.GetRowId(), kuberType)
	p.publishOptimizationItem(item)
}

func (p *Processor) initDaemonsetProcessor(processorConf shared.Configuration) *daemonsets.Processor {
	daemonSetPublisher := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "daemonset")
	}
	processorConf.PublishOptimizationItem = daemonSetPublisher
	return daemonsets.NewProcessor(processorConf)
}

func (p *Processor) initDeploymentProcessor(processorConf shared.Configuration) *deployments.Processor {
	deploymentPublisher := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "deployment")
	}
	processorConf.PublishOptimizationItem = deploymentPublisher
	return deployments.NewProcessor(processorConf)
}

func (p *Processor) initStatefulsetProcessor(processorConf shared.Configuration) *statefulsets.Processor {
	statefulSetPublisher := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "statefulset")
	}
	processorConf.PublishOptimizationItem = statefulSetPublisher
	return statefulsets.NewProcessor(processorConf)
}

func (p *Processor) initJobProcessor(processorConf shared.Configuration) *jobs.Processor {
	jobsPublisher := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "job")
	}
	processorConf.PublishOptimizationItem = jobsPublisher
	return jobs.NewProcessor(processorConf)
}

func (p *Processor) initPodProcessor(processorConf shared.Configuration) *pods.Processor {
	podsPublisher := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "pod")
	}
	processorConf.PublishOptimizationItem = podsPublisher
	return pods.NewProcessor(processorConf, pods.ProcessorModeOrphan)
}

func NewProcessor(processorConf shared.Configuration) *Processor {
	// TODO: implement specific interaction functions
	p := &Processor{
		identification:            processorConf.Identification,
		kubernetesProvider:        processorConf.KubernetesProvider,
		prometheusProvider:        processorConf.PrometheusProvider,
		itemsToProcessor:          util.NewConcurrentMap[string, string](),
		publishOptimizationItem:   processorConf.PublishOptimizationItem,
		publishResultSummary:      processorConf.PublishResultSummary,
		publishResultSummaryTable: processorConf.PublishResultSummaryTable,
		jobQueue:                  processorConf.JobQueue,
		lazyloadCounter:           processorConf.LazyloadCounter,
		configuration:             processorConf.Configuration,
		client:                    processorConf.Client,
		kaytuClient:               processorConf.KaytuClient,
		namespace:                 processorConf.Namespace,
		selector:                  processorConf.Selector,
		nodeSelector:              processorConf.NodeSelector,
		observabilityDays:         processorConf.ObservabilityDays,
		defaultPreferences:        processorConf.DefaultPreferences,
		summary:                   util.NewConcurrentMap[string, any](),
	}

	p.daemonsetsProcessor = p.initDaemonsetProcessor(processorConf)
	p.deploymentsProcessor = p.initDeploymentProcessor(processorConf)
	p.statefulsetsProcessor = p.initStatefulsetProcessor(processorConf)
	p.jobsProcessor = p.initJobProcessor(processorConf)
	p.podsProcessor = p.initPodProcessor(processorConf)

	return p
}

func (p *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	processorName, ok := p.itemsToProcessor.Get(id)
	if !ok {
		return
	}
	switch processorName {
	case "daemonset":
		p.daemonsetsProcessor.ReEvaluate(id, items)
	case "deployment":
		p.deploymentsProcessor.ReEvaluate(id, items)
	case "statefulset":
		p.statefulsetsProcessor.ReEvaluate(id, items)
	case "job":
		p.jobsProcessor.ReEvaluate(id, items)
	case "pod":
		p.podsProcessor.ReEvaluate(id, items)
	}
}

func (p *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}
