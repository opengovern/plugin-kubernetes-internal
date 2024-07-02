package nodes

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	util "github.com/kaytu-io/plugin-kubernetes-internal/utils"
	"sync/atomic"
)

type Processor struct {
	identification     map[string]string
	kubernetesProvider *kaytuKubernetes.Kubernetes
	client             golang.OptimizationClient
	nodeSelector       string
	items              util.ConcurrentMap[string, NodeItem]
	jobQueue           *sdk.JobQueue
	lazyloadCounter    *atomic.Uint32
}

func NewProcessor(processorConf shared.Configuration) *Processor {
	p := Processor{
		identification:     processorConf.Identification,
		kubernetesProvider: processorConf.KubernetesProvider,
		client:             processorConf.Client,
		nodeSelector:       processorConf.NodeSelector,
		jobQueue:           processorConf.JobQueue,
		lazyloadCounter:    processorConf.LazyloadCounter,
		items:              util.NewConcurrentMap[string, NodeItem](),
	}

	p.jobQueue.Push(NewListAllNodesJob(&p))
	return &p
}
