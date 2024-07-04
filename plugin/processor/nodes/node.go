package nodes

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/simulation"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"sync"
	"sync/atomic"
)

type Processor struct {
	identification     map[string]string
	kubernetesProvider *kaytuKubernetes.Kubernetes
	client             golang.OptimizationClient
	nodeSelector       string
	items              utils.ConcurrentMap[string, NodeItem]
	jobQueue           *sdk.JobQueue
	lazyloadCounter    *atomic.Uint32
	nodesReady         sync.WaitGroup
}

func NewProcessor(processorConf shared.Configuration) *Processor {
	p := Processor{
		identification:     processorConf.Identification,
		kubernetesProvider: processorConf.KubernetesProvider,
		client:             processorConf.Client,
		nodeSelector:       processorConf.NodeSelector,
		jobQueue:           processorConf.JobQueue,
		lazyloadCounter:    processorConf.LazyloadCounter,
		items:              utils.NewConcurrentMap[string, NodeItem](),
		nodesReady:         sync.WaitGroup{},
	}
	p.nodesReady.Add(1)

	p.jobQueue.Push(NewListAllNodesJob(&p))
	return &p
}

func (p *Processor) GetKubernetesNodes() []shared.KubernetesNode {
	p.nodesReady.Wait()
	knodes := make([]shared.KubernetesNode, 0)
	p.items.Range(func(_ string, nodeItem NodeItem) bool {
		knode := shared.KubernetesNode{
			Name:        nodeItem.Node.Name,
			VCores:      float64(nodeItem.Node.Status.Capacity.Cpu().MilliValue()) / 1000.0,
			Memory:      float64(nodeItem.Node.Status.Capacity.Memory().Value()) / simulation.GB,
			MaxPodCount: nodeItem.Node.Status.Capacity.Pods().Value(),
			Taints:      nodeItem.Node.Spec.Taints,
			Labels:      nodeItem.Node.Labels,
		}
		if nodeItem.CostResponse != nil {
			v := nodeItem.CostResponse.GetCost().GetValue()
			knode.Cost = &v
		}

		knodes = append(knodes, knode)
		return true
	})
	return knodes
}
