package plugin

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes/plugin/kaytu"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor"
	"github.com/kaytu-io/plugin-kubernetes/plugin/processor/pods"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes/plugin/version"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"strings"
)

type KubernetesPlugin struct {
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
						Default:     "",
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
		MinKaytuVersion: "v0.11.2-rc.0",
		OverviewChart: &golang.ChartDefinition{
			Columns: []*golang.ChartColumnItem{
				{
					Id:    "namespace",
					Name:  "Namespace",
					Width: 15,
				},
				{
					Id:    "name",
					Name:  "Name",
					Width: 15,
				},
				{
					Id:    "current_cpu_request",
					Name:  "CPU Request",
					Width: 12,
				},
				{
					Id:    "current_cpu_limit",
					Name:  "CPU Limit",
					Width: 10,
				},
				{
					Id:    "current_memory_request",
					Name:  "Memory Request",
					Width: 15,
				},
				{
					Id:    "current_memory_limit",
					Name:  "Memory Limit",
					Width: 13,
				},
				{
					Id:    "suggested_cpu_request",
					Name:  "Suggested CPU Request",
					Width: 22,
				},
				{
					Id:    "suggested_cpu_limit",
					Name:  "Suggested CPU Limit",
					Width: 20,
				},
				{
					Id:    "suggested_memory_request",
					Name:  "Suggested Memory Request",
					Width: 25,
				},
				{
					Id:    "suggested_memory_limit",
					Name:  "Suggested Memory Limit",
					Width: 23,
				},
				{
					Id:    "skipped",
					Name:  "Skipped",
					Width: 30,
				},
				{
					Id:    "loading",
					Name:  "Loading",
					Width: 8,
				},
				{
					Id:    "right_arrow",
					Name:  "",
					Width: 1,
				},
			},
		},
		DevicesChart: &golang.ChartDefinition{
			Columns: []*golang.ChartColumnItem{
				{
					Id:    "name",
					Name:  "Name",
					Width: 15,
				},
				{
					Id:    "current_cpu_request",
					Name:  "CPU Request",
					Width: 12,
				},
				{
					Id:    "current_cpu_limit",
					Name:  "CPU Limit",
					Width: 10,
				},
				{
					Id:    "current_memory_request",
					Name:  "Memory Request",
					Width: 15,
				},
				{
					Id:    "current_memory_limit",
					Name:  "Memory Limit",
					Width: 13,
				},
				{
					Id:    "suggested_cpu_request",
					Name:  "Suggested CPU Request",
					Width: 22,
				},
				{
					Id:    "suggested_cpu_limit",
					Name:  "Suggested CPU Limit",
					Width: 20,
				},
				{
					Id:    "suggested_memory_request",
					Name:  "Suggested Memory Request",
					Width: 25,
				},
				{
					Id:    "suggested_memory_limit",
					Name:  "Suggested Memory Limit",
					Width: 23,
				},
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

	conn, err := grpc.Dial("gapi.kaytu.io:443",
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
		grpc.WithPerRPCCredentials(oauth.TokenSource{
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: kaytuAccessToken,
			}),
		}))
	if err != nil {
		return err
	}
	client := golang2.NewOptimizationClient(conn)

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
	promCfg, err := kaytuPrometheus.GetConfig(promAddress, promUsername, promPassword, promClientId, promClientSecret, promTokenUrl, promScopes, kubeClient)
	if err != nil {
		return err
	}

	promClient, err := kaytuPrometheus.NewPrometheus(promCfg)
	if err != nil {
		return err
	}

	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Coi{
				Coi: item,
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

	configurations, err := kaytu.ConfigurationRequest()
	if err != nil {
		return err
	}

	switch command {
	case "kubernetes-pods":
		p.processor = pods.NewProcessor(ctx, kubeClient, promClient, publishOptimizationItem, kaytuAccessToken, jobQueue, configurations, client)
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
