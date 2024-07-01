package nodes

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	"sync/atomic"
)

type Processor struct {
	kubernetesProvider *kaytuKubernetes.Kubernetes
	nodeSelector       string
	items              util.ConcurrentMap[string, NodeItem]
	jobQueue           *sdk.JobQueue
	lazyloadCounter    *atomic.Uint32
}

func NewProcessor(processorConf shared.Configuration) *Processor {
	p := Processor{
		kubernetesProvider: processorConf.KubernetesProvider,
		nodeSelector:       processorConf.NodeSelector,
		jobQueue:           processorConf.JobQueue,
		lazyloadCounter:    processorConf.LazyloadCounter,
		items:              util.NewConcurrentMap[string, NodeItem](),
	}

	p.jobQueue.Push(NewListAllNodesJob(&p))
	return &p
}
