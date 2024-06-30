package shared

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu"
	kaytuAgent "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kaytu-agent"
	kaytuKubernetes "github.com/kaytu-io/plugin-kubernetes-internal/plugin/kubernetes"
	kaytuPrometheus "github.com/kaytu-io/plugin-kubernetes-internal/plugin/prometheus"
	golang2 "github.com/kaytu-io/plugin-kubernetes-internal/plugin/proto/src/golang"
	"sync/atomic"
)

type Configuration struct {
	Identification            map[string]string
	KubernetesProvider        *kaytuKubernetes.Kubernetes
	PrometheusProvider        *kaytuPrometheus.Prometheus
	KaytuClient               *kaytuAgent.KaytuAgent
	KaytuAcccessToken         string
	PublishOptimizationItem   func(item *golang.ChartOptimizationItem)
	PublishResultSummary      func(summary *golang.ResultSummary)
	PublishResultSummaryTable func(summary *golang.ResultSummaryTable)
	LazyloadCounter           *atomic.Uint32
	JobQueue                  *sdk.JobQueue
	Configuration             *kaytu.Configuration
	Client                    golang2.OptimizationClient
	Namespace                 *string
	Selector                  string
	NodeSelector              string
	ObservabilityDays         int
	DefaultPreferences        []*golang.PreferenceItem
}
