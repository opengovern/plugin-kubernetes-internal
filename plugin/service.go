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
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
	"strings"
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
					{
						Name:        "prom-address",
						Default:     "http://localhost:9090",
						Description: "Prometheus address",
						Required:    false,
					},
					{
						Name:        "prom-username",
						Default:     "",
						Description: "Prometheus basic auth username",
						Required:    false,
					},
					{
						Name:        "prom-password",
						Default:     "",
						Description: "Prometheus basic auth password",
						Required:    false,
					},
					{
						Name:        "prom-client-id",
						Default:     "",
						Description: "Prometheus OAuth2 client id",
						Required:    false,
					},
					{
						Name:        "prom-client-secret",
						Default:     "",
						Description: "Prometheus OAuth2 client secret",
						Required:    false,
					},
					{
						Name:        "prom-token-url",
						Default:     "",
						Description: "Prometheus OAuth2 token url",
						Required:    false,
					},
					{
						Name:        "prom-scopes",
						Default:     "",
						Description: "Prometheus OAuth2 comma seperated scopes",
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

func getFlagOrNil(flags map[string]string, key string) *string {
	if val, ok := flags[key]; ok {
		return &val
	}
	return nil
}

func (p *KubernetesPlugin) StartProcess(command string, flags map[string]string, kaytuAccessToken string, jobQueue *sdk.JobQueue) error {
	ctx := context.Background()

	kubeContext := getFlagOrNil(flags, "context")
	kubeCfg, err := kaytuKubernetes.GetConfig(ctx, kubeContext)
	if err != nil {
		return err
	}
	kubeClient, err := kaytuKubernetes.NewKubernetes(kubeCfg)
	if err != nil {
		return err
	}

	promAddress := getFlagOrNil(flags, "prom-address")
	promUsername := getFlagOrNil(flags, "prom-username")
	promPassword := getFlagOrNil(flags, "prom-password")
	promClientId := getFlagOrNil(flags, "prom-client-id")
	promClientSecret := getFlagOrNil(flags, "prom-client-secret")
	promTokenUrl := getFlagOrNil(flags, "prom-token-url")
	promScopesStr := getFlagOrNil(flags, "prom-scopes")
	var promScopes []string
	if promScopesStr != nil {
		promScopes = strings.Split(*promScopesStr, ",")
	}
	promCfg := kaytuPrometheus.GetConfig(promAddress, promUsername, promPassword, promClientId, promClientSecret, promTokenUrl, promScopes)
	promClient, err := kaytuPrometheus.NewPrometheus(promCfg)
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
		p.processor = pods.NewProcessor(ctx, kubeClient, promClient, publishOptimizationItem, kaytuAccessToken, jobQueue, nil)
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
