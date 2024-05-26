package plugin

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor/pods"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
)

type KubernetesPlugin struct {
	cfg       aws.Config
	stream    golang.Plugin_RegisterClient
	processor processor.Processor
}

func NewPlugin() *KubernetesPlugin {
	return &KubernetesPlugin{}
}

func (p *KubernetesPlugin) GetConfig() golang.RegisterConfig {
	return golang.RegisterConfig{
		Name:     "kaytu-io/plugin-kubernetes",
		Version:  version.VERSION,
		Provider: "kubernetes",
		Commands: []*golang.Command{
			{
				Name:        "kubernetes-pods",
				Description: "Get optimization suggestions for your Kubernetes Pods",
				Flags: []*golang.Flag{
					{
						Name:        "context",
						Default:     "",
						Description: "Kubectl context name",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultPodsPreferences,
				LoginRequired:      true,
			},
		},
	}
}

func (p *KubernetesPlugin) SetStream(stream golang.Plugin_RegisterClient) {
	p.stream = stream
}

func (p *KubernetesPlugin) StartProcess(command string, flags map[string]string, kaytuAccessToken string, jobQueue *sdk.JobQueue) error {
	ctx := context.Background()
	kubeContext := flags["context"]
	cfg, err := kaytuKubernetes.GetConfig(ctx, &kubeContext)
	if err != nil {
		return err
	}

	kubeClient, err := kaytuKubernetes.NewKubernetes(cfg)
	if err != nil {
		return err
	}

	publishOptimizationItem := func(item *golang.OptimizationItem) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Oi{
				Oi: item,
			},
		})
	}

	publishResultsReady := func(b bool) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Ready{
				Ready: &golang.ResultsReady{
					Ready: b,
				},
			},
		})
	}
	publishResultsReady(false)

	switch command {
	case "kubernetes-pods":
		p.processor = pods.NewProcessor(ctx, kubeClient, nil, publishOptimizationItem, kaytuAccessToken, jobQueue, nil)
		if err != nil {
			return err
		}
	}

	jobQueue.SetOnFinish(func() {
		publishResultsReady(true)
	})

	return nil
}

func (p *KubernetesPlugin) ReEvaluate(evaluate *golang.ReEvaluate) {
	p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}
