package prometheus

import (
	"context"
	"fmt"
	promapi "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"time"
)

type Prometheus struct {
	cfg    Config
	client promapi.Client
	api    prometheus.API
}

func NewPrometheus(cfg Config) (*Prometheus, error) {
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

func parsePrometheusResponse(value model.Value) (map[string]float64, error) {
	result := make(map[string]float64)

	switch value.Type() {
	case model.ValMatrix:
		matrix := value.(model.Matrix)
		for _, sample := range matrix {
			if len(sample.Values) == 0 {
				continue
			}
			for _, v := range sample.Values {
				result[v.Timestamp.String()] = float64(v.Value)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %s", value.Type())
	}

	return result, nil
}

func (p *Prometheus) GetCpuMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string) (map[string]float64, error) {
	query := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s", container="%s"}[1m])) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(-7 * 24 * time.Hour).Truncate(time.Hour),
		End:   time.Now().Truncate(time.Hour),
		Step:  time.Hour,
	})
	if err != nil {
		return nil, err
	}

	return parsePrometheusResponse(value)
}
