package prometheus

import (
	"context"
	"fmt"
	promapi "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"sort"
	"time"
)

type Prometheus struct {
	cfg    *Config
	client promapi.Client
	api    prometheus.API
}

type PromDatapoint struct {
	Timestamp time.Time
	Value     float64
}

func NewPrometheus(cfg *Config) (*Prometheus, error) {
	promCfg := promapi.Config{
		Address:      cfg.Address,
		RoundTripper: promapi.DefaultRoundTripper,
	}
	switch cfg.AuthType {
	case PromAuthTypeBasic:
		promCfg.RoundTripper = config.NewBasicAuthRoundTripper(cfg.BasicUsername, config.Secret(cfg.BasicPassword), "", "", promCfg.RoundTripper)
	case PromAuthTypeOAuth2:
		promCfg.RoundTripper = config.NewOAuth2RoundTripper(&config.OAuth2{
			ClientID:     cfg.OAuth2ClientID,
			ClientSecret: config.Secret(cfg.OAuth2ClientSecret),
			TokenURL:     cfg.OAuth2TokenURL,
			Scopes:       cfg.OAuth2Scopes,
		}, promCfg.RoundTripper, nil)
	}

	promClient, err := promapi.NewClient(promCfg)
	if err != nil {
		return nil, err
	}
	promApi := prometheus.NewAPI(promClient)

	prom := Prometheus{
		cfg:    cfg,
		client: promClient,
		api:    promApi,
	}
	return &prom, nil
}

func parsePrometheusResponse(value model.Value) ([]PromDatapoint, error) {
	var result []PromDatapoint

	switch value.Type() {
	case model.ValMatrix:
		matrix := value.(model.Matrix)
		for _, sample := range matrix {
			if len(sample.Values) == 0 {
				continue
			}
			for _, v := range sample.Values {
				result = append(result, PromDatapoint{
					Timestamp: v.Timestamp.Time(),
					Value:     float64(v.Value),
				})
			}
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %s", value.Type())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result, nil
}

func (p *Prometheus) GetCpuMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string) ([]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s", container="%s"}[1m])) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(-1 * 24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	return parsePrometheusResponse(value)
}

func (p *Prometheus) GetMemoryMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string) ([]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(container_memory_usage_bytes{namespace="%s", pod="%s", container="%s"}) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(-1 * 24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	return parsePrometheusResponse(value)
}

func (p *Prometheus) Ping(ctx context.Context) error {
	_, err := p.api.Config(ctx)
	return err
}
