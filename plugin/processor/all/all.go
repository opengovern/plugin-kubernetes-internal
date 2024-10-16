package all

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/daemonsets"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/deployments"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/jobs"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/nodes"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/pods"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/simulation"
	"github.com/opengovern/plugin-kubernetes-internal/plugin/processor/statefulsets"
	"strconv"
)

type Processor struct {
	itemsToProcessor          utils.ConcurrentMap[string, string]
	publishOptimizationItem   func(item *golang.ChartOptimizationItem)
	publishResultSummary      func(summary *golang.ResultSummary)
	publishResultSummaryTable func(summary *golang.ResultSummaryTable)
	summary                   utils.ConcurrentMap[string, shared.ResourceSummary]

	nodesProcessor        *nodes.Processor
	daemonsetsProcessor   *daemonsets.Processor
	deploymentsProcessor  *deployments.Processor
	statefulsetsProcessor *statefulsets.Processor
	jobsProcessor         *jobs.Processor
	podsProcessor         *pods.Processor
	schedulingSim         *simulation.SchedulerService
	schedulingSimPrev     *simulation.SchedulerService
	processorConf         shared.Configuration
}

func (p *Processor) publishOptimizationItemFunc(item *golang.ChartOptimizationItem, kuberType string) {
	p.itemsToProcessor.Set(item.OverviewChartRow.GetRowId(), kuberType)
	p.publishOptimizationItem(item)
}

func (p *Processor) publishResultSummaryFunc(kuberType string) {
	var resourceSummary *shared.ResourceSummary
	switch kuberType {
	case "daemonset":
		_, resourceSummary = shared.GetAggregatedResultsSummary(p.daemonsetsProcessor.GetSummaryMap())
	case "deployment":
		_, resourceSummary = shared.GetAggregatedResultsSummary(p.deploymentsProcessor.GetSummaryMap())
	case "statefulset":
		_, resourceSummary = shared.GetAggregatedResultsSummary(p.statefulsetsProcessor.GetSummaryMap())
	case "job":
		_, resourceSummary = shared.GetAggregatedResultsSummary(p.jobsProcessor.GetSummaryMap())
	case "pod":
		_, resourceSummary = shared.GetAggregatedResultsSummary(p.podsProcessor.GetSummaryMap())
	}
	if resourceSummary != nil {
		p.summary.Set(kuberType, *resourceSummary)
		rs, _ := shared.GetAggregatedResultsSummary(&p.summary)
		p.publishResultSummary(rs)
	}
}

func (p *Processor) publishResultSummaryTableFunc(kuberType string) {
	var resourceSummary *shared.ResourceSummary
	switch kuberType {
	case "daemonset":
		_, resourceSummary = shared.GetAggregatedResultsSummaryTable(p.daemonsetsProcessor.GetSummaryMap(), nil, nil, nil)
	case "deployment":
		_, resourceSummary = shared.GetAggregatedResultsSummaryTable(p.deploymentsProcessor.GetSummaryMap(), nil, nil, nil)
	case "statefulset":
		_, resourceSummary = shared.GetAggregatedResultsSummaryTable(p.statefulsetsProcessor.GetSummaryMap(), nil, nil, nil)
	case "job":
		_, resourceSummary = shared.GetAggregatedResultsSummaryTable(p.jobsProcessor.GetSummaryMap(), nil, nil, nil)
	case "pod":
		_, resourceSummary = shared.GetAggregatedResultsSummaryTable(p.podsProcessor.GetSummaryMap(), nil, nil, nil)
	}
	if resourceSummary != nil {
		p.summary.Set(kuberType, *resourceSummary)
		var nds, ndsPrev, cluster []shared.KubernetesNode
		if (p.processorConf.Namespace == nil ||
			*p.processorConf.Namespace == "") && p.processorConf.Selector == "" && p.processorConf.NodeSelector == "" {
			var err error

			cluster = p.nodesProcessor.GetKubernetesNodes()
			nds, err = p.schedulingSim.Simulate()
			if err != nil {
				fmt.Println("failed to simulate due to", err)
			}

			ndsPrev, err = p.schedulingSimPrev.Simulate()
			if err != nil {
				fmt.Println("failed to simulate prev due to", err)
			}
		} else {
			fmt.Println(
				"++++++++++++++++",
				p.processorConf.Namespace,
				p.processorConf.Selector,
				p.processorConf.NodeSelector,
			)
		}

		rs, _ := shared.GetAggregatedResultsSummaryTable(&p.summary, cluster, nds, ndsPrev)
		p.publishResultSummaryTable(rs)
	}
}

func (p *Processor) initDaemonsetProcessor(processorConf shared.Configuration) *daemonsets.Processor {
	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "daemonset")
	}
	publishResultSummary := func(_ *golang.ResultSummary) {
		p.publishResultSummaryFunc("daemonset")
	}
	publishResultSummaryTable := func(_ *golang.ResultSummaryTable) {
		p.publishResultSummaryTableFunc("daemonset")
	}

	processorConf.PublishOptimizationItem = publishOptimizationItem
	processorConf.PublishResultSummary = publishResultSummary
	processorConf.PublishResultSummaryTable = publishResultSummaryTable
	pi := daemonsets.NewProcessor(processorConf, p.nodesProcessor)
	pi.SetSchedulingSim(p.schedulingSim, p.schedulingSimPrev)
	return pi
}

func (p *Processor) initDeploymentProcessor(processorConf shared.Configuration) *deployments.Processor {
	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "deployment")
	}
	publishResultSummary := func(_ *golang.ResultSummary) {
		p.publishResultSummaryFunc("deployment")
	}
	publishResultSummaryTable := func(_ *golang.ResultSummaryTable) {
		p.publishResultSummaryTableFunc("deployment")
	}

	processorConf.PublishOptimizationItem = publishOptimizationItem
	processorConf.PublishResultSummary = publishResultSummary
	processorConf.PublishResultSummaryTable = publishResultSummaryTable
	pi := deployments.NewProcessor(processorConf, p.nodesProcessor)
	pi.SetSchedulingSim(p.schedulingSim, p.schedulingSimPrev)
	return pi
}

func (p *Processor) initStatefulsetProcessor(processorConf shared.Configuration) *statefulsets.Processor {
	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "statefulset")
	}
	publishResultSummary := func(_ *golang.ResultSummary) {
		p.publishResultSummaryFunc("statefulset")
	}
	publishResultSummaryTable := func(_ *golang.ResultSummaryTable) {
		p.publishResultSummaryTableFunc("statefulset")
	}

	processorConf.PublishOptimizationItem = publishOptimizationItem
	processorConf.PublishResultSummary = publishResultSummary
	processorConf.PublishResultSummaryTable = publishResultSummaryTable
	pi := statefulsets.NewProcessor(processorConf, p.nodesProcessor)
	pi.SetSchedulingSim(p.schedulingSim, p.schedulingSimPrev)
	return pi
}

func (p *Processor) initJobProcessor(processorConf shared.Configuration) *jobs.Processor {
	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "job")
	}
	publishResultSummary := func(_ *golang.ResultSummary) {
		p.publishResultSummaryFunc("job")
	}
	publishResultSummaryTable := func(_ *golang.ResultSummaryTable) {
		p.publishResultSummaryTableFunc("job")
	}

	processorConf.PublishOptimizationItem = publishOptimizationItem
	processorConf.PublishResultSummary = publishResultSummary
	processorConf.PublishResultSummaryTable = publishResultSummaryTable
	pi := jobs.NewProcessor(processorConf, p.nodesProcessor)
	pi.SetSchedulingSim(p.schedulingSim, p.schedulingSimPrev)
	return pi
}

func (p *Processor) initPodProcessor(processorConf shared.Configuration) *pods.Processor {
	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.publishOptimizationItemFunc(item, "pod")
	}
	publishResultSummary := func(_ *golang.ResultSummary) {
		p.publishResultSummaryFunc("pod")
	}
	publishResultSummaryTable := func(summary *golang.ResultSummaryTable) {
		p.publishResultSummaryTableFunc("pod")
	}

	processorConf.PublishOptimizationItem = publishOptimizationItem
	processorConf.PublishResultSummary = publishResultSummary
	processorConf.PublishResultSummaryTable = publishResultSummaryTable
	pi := pods.NewProcessor(processorConf, pods.ProcessorModeOrphan, p.nodesProcessor)
	pi.SetSchedulingSim(p.schedulingSim, p.schedulingSimPrev)
	return pi
}

func NewProcessor(processorConf shared.Configuration, nodesProcessor *nodes.Processor) *Processor {
	p := &Processor{
		itemsToProcessor:          utils.NewConcurrentMap[string, string](),
		publishOptimizationItem:   processorConf.PublishOptimizationItem,
		publishResultSummary:      processorConf.PublishResultSummary,
		publishResultSummaryTable: processorConf.PublishResultSummaryTable,
		summary:                   utils.NewConcurrentMap[string, shared.ResourceSummary](),
		schedulingSim:             simulation.NewSchedulerService(nil),
		schedulingSimPrev:         simulation.NewSchedulerService(nil),
		nodesProcessor:            nodesProcessor,
		processorConf:             processorConf,
	}

	p.schedulingSim.SetNodes(nodesProcessor.GetKubernetesNodes())
	p.schedulingSimPrev.SetNodes(nodesProcessor.GetKubernetesNodes())

	p.daemonsetsProcessor = p.initDaemonsetProcessor(processorConf)
	p.deploymentsProcessor = p.initDeploymentProcessor(processorConf)
	p.statefulsetsProcessor = p.initStatefulsetProcessor(processorConf)
	p.jobsProcessor = p.initJobProcessor(processorConf)
	p.podsProcessor = p.initPodProcessor(processorConf)

	return p
}

func (p *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	nodeCpuBreathingRoom, nodeMemoryBreathingRoom, nodePodCountBreathingRoom := "", "", ""
	for _, i := range items {
		if i.Key == "NodeCPUBreathingRoom" {
			nodeCpuBreathingRoom = i.Value.GetValue()
			f, err := strconv.ParseFloat(nodeCpuBreathingRoom, 64)
			if err == nil {
				simulation.CPUHeadroomFactor = 1.0 - (f / 100.0)
			}
		}
		if i.Key == "NodeMemoryBreathingRoom" {
			nodeMemoryBreathingRoom = i.Value.GetValue()
			f, err := strconv.ParseFloat(nodeMemoryBreathingRoom, 64)
			if err == nil {
				simulation.MemoryHeadroomFactor = 1.0 - (f / 100.0)
			}
		}
		if i.Key == "NodePodCountBreathingRoom" {
			nodePodCountBreathingRoom = i.Value.GetValue()
			f, err := strconv.ParseFloat(nodePodCountBreathingRoom, 64)
			if err == nil {
				simulation.PodHeadroomFactor = 1.0 - (f / 100.0)
			}
		}
	}

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
