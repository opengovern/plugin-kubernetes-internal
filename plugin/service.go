package plugin

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/preferences"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/all"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/daemonsets"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/deployments"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/jobs"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/nodes"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/pods"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/shared"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/processor/statefulsets"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/version"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"log"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
)

type KubernetesPlugin struct {
	stream    *sdk.StreamController
	processor processor.Processor
}

func NewPlugin() *KubernetesPlugin {
	return &KubernetesPlugin{}
}

func (p *KubernetesPlugin) GetConfig(_ context.Context) golang.RegisterConfig {
	commonFlags := []*golang.Flag{
		{
			Name:        "context",
			Default:     "",
			Description: "Kubectl context name",
			Required:    false,
		},
		{
			Name:        "observabilityDays",
			Default:     "1",
			Description: "Observability Days",
			Required:    false,
		},
		{
			Name:        "namespace",
			Default:     "",
			Description: "Kubernetes namespace",
			Required:    false,
		},
		{
			Name:        "selector",
			Default:     "",
			Description: "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)",
			Required:    false,
		},
		{
			Name:        "nodeSelector",
			Default:     "",
			Description: "Node selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)",
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
		{
			Name:        "agent-address",
			Default:     "",
			Description: "Agent address",
			Required:    false,
		},
		{
			Name:        "agent-disabled",
			Default:     "false",
			Description: "Disable agent",
			Required:    false,
		},
		{
			Name:        "aws-cli-profile",
			Default:     "",
			Description: "AWS profile for authentication",
			Required:    false,
		},
	}
	return golang.RegisterConfig{
		Name:     "kaytu-io/plugin-kubernetes",
		Version:  version.VERSION,
		Provider: "kubernetes",
		Commands: []*golang.Command{
			{
				Name:               "kubernetes-pods",
				Description:        "Get optimization suggestions for your Kubernetes Pods",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
			{
				Name:               "kubernetes-deployments",
				Description:        "Get optimization suggestions for your Kubernetes Deployments",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
			{
				Name:               "kubernetes-statefulsets",
				Description:        "Get optimization suggestions for your Kubernetes Statefulsets",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
			{
				Name:               "kubernetes-daemonsets",
				Description:        "Get optimization suggestions for your Kubernetes Daemonsets",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
			{
				Name:               "kubernetes-jobs",
				Description:        "Get optimization suggestions for your Kubernetes Jobs",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
			{
				Name:               "kubernetes",
				Description:        "Get optimization suggestions for all Kubernetes resources",
				Flags:              commonFlags,
				DefaultPreferences: preferences.DefaultKubernetesPreferences,
				LoginRequired:      true,
			},
		},
		MinKaytuVersion: "v0.9.0",
		OverviewChart: &golang.ChartDefinition{
			Columns: []*golang.ChartColumnItem{
				{
					Id:    "name",
					Name:  "Name",
					Width: 20,
				},
				{
					Id:    "namespace",
					Name:  "Namespace",
					Width: 15,
				},
				{
					Id:       "cpu_change",
					Name:     "CPU Change",
					Width:    40,
					Sortable: true,
				},
				{
					Id:       "memory_change",
					Name:     "Memory Change",
					Width:    40,
					Sortable: true,
				},
				{
					Id:       "cost",
					Name:     "Monthly Cost",
					Width:    10,
					Sortable: true,
				},
				{
					Id:    "x_kaytu_status",
					Name:  "Status",
					Width: 21,
				},
				{
					Id:    "x_kaytu_right_arrow",
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
					Id:    "current_cpu",
					Name:  "Current CPU Configuration",
					Width: 20,
				},
				{
					Id:    "current_memory",
					Name:  "Current Memory Configuration",
					Width: 20,
				},
				{
					Id:    "suggested_cpu",
					Name:  "Suggested CPU Configuration",
					Width: 20,
				},
				{
					Id:    "suggested_memory",
					Name:  "Suggested Memory Configuration",
					Width: 20,
				},
			},
		},
	}
}

func (p *KubernetesPlugin) SetStream(_ context.Context, stream *sdk.StreamController) {
	p.stream = stream
}

func getFlagOrNil(flags map[string]string, key string) *string {
	if val, ok := flags[key]; ok {
		return &val
	}
	return nil
}

func (p *KubernetesPlugin) StartProcess(ctx context.Context, command string, flags map[string]string, kaytuAccessToken string, preferences []*golang.PreferenceItem, jobQueue *sdk.JobQueue) error {
	kubeContext := getFlagOrNil(flags, "context")
	restclientConfig, kubeConfig, err := kaytuKubernetes.GetConfig(ctx, kubeContext)
	if err != nil {
		return err
	}
	kubeClient, err := kaytuKubernetes.NewKubernetes(restclientConfig, kubeConfig)
	if err != nil {
		return err
	}

	identification := kubeClient.Identify()
	conn, err := grpc.NewClient("gapi.kaytu.io:443",
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

	agentAddress := getFlagOrNil(flags, "agent-address")
	agentDisabledStr := getFlagOrNil(flags, "agent-disabled")
	agentDisabled := false
	if agentDisabledStr != nil {
		agentDisabled, err = strconv.ParseBool(*agentDisabledStr)
		if err != nil {
			return err
		}
	}
	kaytuAgentCfg, err := kaytuAgent.GetConfig(ctx, agentAddress, agentDisabled, kubeClient)
	if err != nil {
		return err
	}

	kaytuClient, err := kaytuAgent.NewKaytuAgent(kaytuAgentCfg, agentDisabled)
	if err != nil {
		return err
	}

	if kaytuClient.IsEnabled() {
		err = kaytuClient.Ping(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to kaytu agent on %s due to %v", kaytuAgentCfg.Address, err)
		}
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

	var promClient *kaytuPrometheus.Prometheus
	if !kaytuClient.IsEnabled() {
		promCfg, err := kaytuPrometheus.GetConfig(ctx, promAddress, promUsername, promPassword, promClientId, promClientSecret, promTokenUrl, promScopes, kubeClient)
		if err != nil {
			return err
		}
		promClient, err = kaytuPrometheus.NewPrometheus(ctx, promCfg)
		if err != nil {
			return err
		}
		err = promClient.Ping(ctx)
		if err != nil {
			return fmt.Errorf("failed to connect to prometheus on %s due to %v", promCfg.Address, err)
		}
	}

	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		err := p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Coi{
				Coi: item,
			},
		})
		if err != nil {
			log.Printf("failed to send COI: %v", err)
		}
	}

	publishResultsReady := func(b bool) {
		err := p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Ready{
				Ready: &golang.ResultsReady{
					Ready: b,
				},
			},
		})
		if err != nil {
			log.Printf("failed to send results ready: %v", err)
		}
	}

	publishResultSummary := func(summary *golang.ResultSummary) {
		err := p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Summary{
				Summary: summary,
			},
		})
		if err != nil {
			log.Printf("failed to send summary: %v", err)
		}
	}

	publishResultSummaryTable := func(summary *golang.ResultSummaryTable) {
		err := p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_SummaryTable{
				SummaryTable: summary,
			},
		})
		if err != nil {
			log.Printf("failed to send summary table: %v", err)
		}
	}

	publishNonInteractiveExport := func(ex *golang.NonInteractiveExport) {
		err := p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_NonInteractive{
				NonInteractive: ex,
			},
		})
		if err != nil {
			log.Printf("failed to send non interactive export: %v", err)
		}
	}

	publishResultsReady(false)

	configurations, err := kaytu.ConfigurationRequest()
	if err != nil {
		return err
	}

	namespace := getFlagOrNil(flags, "namespace")
	selector := getFlagOrNil(flags, "selector")
	labelSelector := ""
	if selector != nil {
		labelSelector = *selector
	}
	nodeSelector := getFlagOrNil(flags, "nodeSelector")
	nodeLabelSelector := ""
	if nodeSelector != nil {
		nodeLabelSelector = *nodeSelector
	}

	for key, value := range flags {
		if key == "output" && value != "" && value != "interactive" {
			configurations.KubernetesLazyLoad = math.MaxInt
		}
	}

	observabilityDays := 1
	if flags["observabilityDays"] != "" {
		days, _ := strconv.ParseInt(strings.TrimSpace(flags["observabilityDays"]), 10, 64)
		if days > 0 {
			observabilityDays = int(days)
		}
	}

	processorConf := shared.Configuration{
		Identification:            identification,
		KubernetesProvider:        kubeClient,
		PrometheusProvider:        promClient,
		KaytuClient:               kaytuClient,
		KaytuAcccessToken:         kaytuAccessToken,
		PublishOptimizationItem:   publishOptimizationItem,
		PublishResultSummary:      publishResultSummary,
		PublishResultSummaryTable: publishResultSummaryTable,
		LazyloadCounter:           &atomic.Uint32{},
		JobQueue:                  jobQueue,
		Configuration:             configurations,
		Client:                    client,
		Namespace:                 namespace,
		Selector:                  labelSelector,
		NodeSelector:              nodeLabelSelector,
		ObservabilityDays:         observabilityDays,
		DefaultPreferences:        preferences,
	}

	switch command {
	case "kubernetes-pods":
		nodeProcessor := nodes.NewProcessor(processorConf)
		p.processor = pods.NewProcessor(processorConf, pods.ProcessorModeAll, nodeProcessor)
	case "kubernetes-deployments":
		nodeProcessor := nodes.NewProcessor(processorConf)
		err = p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_UpdateChart{
				UpdateChart: &golang.UpdateChartDefinition{
					OverviewChart: &golang.ChartDefinition{
						Columns: []*golang.ChartColumnItem{
							{
								Id:    "name",
								Name:  "Name",
								Width: 20,
							},
							{
								Id:    "namespace",
								Name:  "Namespace",
								Width: 15,
							},
							{
								Id:       "pod_count",
								Name:     "# Pods",
								Width:    6,
								Sortable: true,
							},
							{
								Id:       "cpu_change",
								Name:     "CPU Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "memory_change",
								Name:     "Memory Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "cost",
								Name:     "Monthly Cost",
								Width:    10,
								Sortable: true,
							},
							{
								Id:    "x_kaytu_status",
								Name:  "Status",
								Width: 21,
							},
							{
								Id:    "x_kaytu_right_arrow",
								Name:  "",
								Width: 1,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		p.processor = deployments.NewProcessor(processorConf, nodeProcessor)
	case "kubernetes-statefulsets":
		nodeProcessor := nodes.NewProcessor(processorConf)
		err = p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_UpdateChart{
				UpdateChart: &golang.UpdateChartDefinition{
					OverviewChart: &golang.ChartDefinition{
						Columns: []*golang.ChartColumnItem{
							{
								Id:    "name",
								Name:  "Name",
								Width: 20,
							},
							{
								Id:    "namespace",
								Name:  "Namespace",
								Width: 15,
							},
							{
								Id:       "pod_count",
								Name:     "# Pods",
								Width:    6,
								Sortable: true,
							},
							{
								Id:       "cpu_change",
								Name:     "CPU Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "memory_change",
								Name:     "Memory Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "cost",
								Name:     "Monthly Cost",
								Width:    10,
								Sortable: true,
							},
							{
								Id:    "x_kaytu_status",
								Name:  "Status",
								Width: 21,
							},
							{
								Id:    "x_kaytu_right_arrow",
								Name:  "",
								Width: 1,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		p.processor = statefulsets.NewProcessor(processorConf, nodeProcessor)
	case "kubernetes-daemonsets":
		nodeProcessor := nodes.NewProcessor(processorConf)
		err = p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_UpdateChart{
				UpdateChart: &golang.UpdateChartDefinition{
					OverviewChart: &golang.ChartDefinition{
						Columns: []*golang.ChartColumnItem{
							{
								Id:    "name",
								Name:  "Name",
								Width: 20,
							},
							{
								Id:    "namespace",
								Name:  "Namespace",
								Width: 15,
							},
							{
								Id:       "pod_count",
								Name:     "# Pods",
								Width:    6,
								Sortable: true,
							},
							{
								Id:       "cpu_change",
								Name:     "CPU Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "memory_change",
								Name:     "Memory Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "cost",
								Name:     "Monthly Cost",
								Width:    10,
								Sortable: true,
							},
							{
								Id:    "x_kaytu_status",
								Name:  "Status",
								Width: 21,
							},
							{
								Id:    "x_kaytu_right_arrow",
								Name:  "",
								Width: 1,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		p.processor = daemonsets.NewProcessor(processorConf, nodeProcessor)
	case "kubernetes-jobs":
		nodeProcessor := nodes.NewProcessor(processorConf)
		err = p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_UpdateChart{
				UpdateChart: &golang.UpdateChartDefinition{
					OverviewChart: &golang.ChartDefinition{
						Columns: []*golang.ChartColumnItem{
							{
								Id:    "name",
								Name:  "Name",
								Width: 20,
							},
							{
								Id:    "namespace",
								Name:  "Namespace",
								Width: 15,
							},
							{
								Id:       "pod_count",
								Name:     "# Pods",
								Width:    6,
								Sortable: true,
							},
							{
								Id:       "cpu_change",
								Name:     "CPU Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "memory_change",
								Name:     "Memory Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "cost",
								Name:     "Monthly Cost",
								Width:    10,
								Sortable: true,
							},
							{
								Id:    "x_kaytu_status",
								Name:  "Status",
								Width: 21,
							},
							{
								Id:    "x_kaytu_right_arrow",
								Name:  "",
								Width: 1,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		p.processor = jobs.NewProcessor(processorConf, nodeProcessor)
	case "kubernetes":
		nodeProcessor := nodes.NewProcessor(processorConf)
		err = p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_UpdateChart{
				UpdateChart: &golang.UpdateChartDefinition{
					OverviewChart: &golang.ChartDefinition{
						Columns: []*golang.ChartColumnItem{
							{
								Id:    "name",
								Name:  "Name",
								Width: 20,
							},
							{
								Id:    "namespace",
								Name:  "Namespace",
								Width: 15,
							},
							{
								Id:       "kubernetes_type",
								Name:     "Kube Type",
								Width:    11,
								Sortable: true,
							},
							{
								Id:       "pod_count",
								Name:     "# Pods",
								Width:    6,
								Sortable: true,
							},
							{
								Id:       "cpu_change",
								Name:     "CPU Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "memory_change",
								Name:     "Memory Change (x Replicas)",
								Width:    40,
								Sortable: true,
							},
							{
								Id:       "cost",
								Name:     "Monthly Cost",
								Width:    10,
								Sortable: true,
							},
							{
								Id:    "x_kaytu_status",
								Name:  "Status",
								Width: 21,
							},
							{
								Id:    "x_kaytu_right_arrow",
								Name:  "",
								Width: 1,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		p.processor = all.NewProcessor(processorConf, nodeProcessor)
	}

	jobQueue.SetOnFinish(func(ctx context.Context) {
		publishNonInteractiveExport(p.processor.ExportNonInteractive())
		publishResultsReady(true)
	})

	return nil
}

func (p *KubernetesPlugin) ReEvaluate(_ context.Context, evaluate *golang.ReEvaluate) {
	p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}
