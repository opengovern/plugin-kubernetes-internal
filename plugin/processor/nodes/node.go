package nodes

import kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"

type Processor struct {
	kubernetesProvider *kaytuKubernetes.Kubernetes
}

func NewProcessor(kubernetesProvider *kaytuKubernetes.Kubernetes) *Processor {
	p := Processor{
		kubernetesProvider: kubernetesProvider,
	}
	return &p
}
