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

type PromDimension interface {
	promDimensionType() PromDimensionType
	isPromDimension()
}

type PromDimensionType int

const (
	PromDimensionTypeDatapoint PromDimensionType = iota
	PromDimensionTypeGroupedDimension
)

type PromDatapoint struct {
	Timestamp time.Time
	Value     float64
}

type PromDatapoints struct {
	Values []PromDatapoint
}

func (PromDatapoints) isPromDimension() {}
func (PromDatapoints) promDimensionType() PromDimensionType {
	return PromDimensionTypeDatapoint
}

type PromGroupedDimension struct {
	Values map[string]PromDimension
}

func (PromGroupedDimension) isPromDimension() {}
func (PromGroupedDimension) promDimensionType() PromDimensionType {
	return PromDimensionTypeGroupedDimension
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

func parsePrometheusResponse(value model.Value) (PromDatapoints, error) {
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
		return PromDatapoints{}, fmt.Errorf("unexpected response type: %s", value.Type())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return PromDatapoints{Values: result}, nil
}

func parseMultiDimensionalGroupedPrometheusResponse(value model.Value, groupBys ...model.LabelName) (PromDimension, error) {
	if len(groupBys) == 0 {
		return parsePrometheusResponse(value)
	}
	groupBy := groupBys[0]

	result := make(map[string]PromDimension)

	switch value.Type() {
	case model.ValMatrix:
		matrix := value.(model.Matrix)
		groupedMatrices := make(map[string]model.Matrix)
		for _, sample := range matrix {
			if len(sample.Values) == 0 {
				continue
			}
			sample := sample
			groupByValue, ok := sample.Metric[groupBy]
			if !ok || !groupByValue.IsValid() {
				return nil, fmt.Errorf("missing or invalid group by value: %s", groupByValue)
			}
			groupByValueStr := string(groupByValue)
			if _, ok := groupedMatrices[groupByValueStr]; !ok {
				groupedMatrices[groupByValueStr] = make(model.Matrix, 0)
			}
			groupedMatrices[groupByValueStr] = append(groupedMatrices[groupByValueStr], sample)
		}

		for groupByValueStr, groupedMatrix := range groupedMatrices {
			dimension, err := parseMultiDimensionalGroupedPrometheusResponse(groupedMatrix, groupBys[1:]...)
			if err != nil {
				return nil, err
			}
			result[groupByValueStr] = dimension
		}
	default:
		return nil, fmt.Errorf("unexpected response type: %s", value.Type())
	}

	return PromGroupedDimension{Values: result}, nil
}

func (p *Prometheus) GetCpuMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string, observabilityDays int) ([]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s", container="%s"}[1m])) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	datapoints, err := parsePrometheusResponse(value)
	if err != nil {
		return nil, err
	}

	return datapoints.Values, nil
}

func (p *Prometheus) GetCpuMetricsForPodPrefix(ctx context.Context, namespace, podPrefix string, observabilityDays int) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s.*", container!=""}[1m])) by (pod, container)`, namespace, podPrefix)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	promDims, err := parseMultiDimensionalGroupedPrometheusResponse(value, "pod", "container")
	if err != nil {
		return nil, err
	}
	if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
		return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
	}
	result := make(map[string]map[string][]PromDatapoint)
	for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
		if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
		}
		result[podName] = make(map[string][]PromDatapoint)
		for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			result[podName][containerName] = promContainerDim.(PromDatapoints).Values
		}
	}
	return result, nil
}

func (p *Prometheus) GetMemoryMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string, observabilityDays int) ([]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`max(container_memory_working_set_bytes{namespace="%s", pod="%s", container="%s"}) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	datapoints, err := parsePrometheusResponse(value)
	if err != nil {
		return nil, err
	}

	return datapoints.Values, nil
}

func (p *Prometheus) GetMemoryMetricsForPodPrefix(ctx context.Context, namespace, podPrefix string, observabilityDays int) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`max(container_memory_working_set_bytes{namespace="%s", pod=~"%s.*", container!=""}) by (pod, container)`, namespace, podPrefix)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	promDims, err := parseMultiDimensionalGroupedPrometheusResponse(value, "pod", "container")
	if err != nil {
		return nil, err
	}
	if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
		return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
	}
	result := make(map[string]map[string][]PromDatapoint)
	for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
		if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
		}
		result[podName] = make(map[string][]PromDatapoint)
		for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			result[podName][containerName] = promContainerDim.(PromDatapoints).Values
		}
	}
	return result, nil
}

func (p *Prometheus) GetCpuThrottlingMetricsForPodContainer(ctx context.Context, namespace, podName, containerName string, observabilityDays int) ([]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(increase(container_cpu_cfs_throttled_periods_total{namespace="%[1]s", pod="%[2]s", container="%[3]s"}[1m])) by (container) / sum(increase(container_cpu_cfs_periods_total{namespace="%[1]s", pod="%[2]s", container="%[3]s"}[1m])) by (container)`, namespace, podName, containerName)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	datapoints, err := parsePrometheusResponse(value)
	if err != nil {
		return nil, err
	}

	return datapoints.Values, nil
}

func (p *Prometheus) GetCpuThrottlingMetricsForPodPrefix(ctx context.Context, namespace, podPrefix string, observabilityDays int) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	step := time.Minute
	query := fmt.Sprintf(`sum(increase(container_cpu_cfs_throttled_periods_total{namespace="%s", pod=~"%s.*", container!=""}[1m])) by (pod, container) / sum(increase(container_cpu_cfs_periods_total{namespace="%s", pod=~"%s.*", container!=""}[1m])) by (pod, container)`, namespace, podPrefix, namespace, podPrefix)
	value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
		Start: time.Now().Add(time.Duration(observabilityDays) * -24 * time.Hour).Truncate(step),
		End:   time.Now().Truncate(step),
		Step:  step,
	})
	if err != nil {
		return nil, err
	}

	promDims, err := parseMultiDimensionalGroupedPrometheusResponse(value, "pod", "container")
	if err != nil {
		return nil, err
	}
	if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
		return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
	}
	result := make(map[string]map[string][]PromDatapoint)
	for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
		if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
		}
		result[podName] = make(map[string][]PromDatapoint)
		for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			result[podName][containerName] = promContainerDim.(PromDatapoints).Values
		}
	}
	return result, nil
}

func (p *Prometheus) Ping(ctx context.Context) error {
	_, err := p.api.Config(ctx)
	return err
}
