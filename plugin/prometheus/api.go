package prometheus

import (
	"context"
	"errors"
	"fmt"
	promapi "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"math"
	"sort"
	"time"
)

const (
	defaultScrapeInterval = time.Minute
)

var (
	cpuUsageMetrics = []string{
		"container_cpu_usage_seconds_total",
	}
	cpuThrottlingThrottledPeriodsMetrics = []string{
		"container_cpu_cfs_throttled_periods_total",
	}
	cpuThrottlingPeriodsMetrics = []string{
		"container_cpu_cfs_periods_total",
	}
	memoryUsageMetrics = []string{
		"container_memory_working_set_bytes",
	}
)

type Prometheus struct {
	cfg            *Config
	client         promapi.Client
	api            prometheus.API
	scrapeInterval time.Duration
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

func (p PromDimensionType) String() string {
	switch p {
	case PromDimensionTypeDatapoint:
		return "datapoint"
	case PromDimensionTypeGroupedDimension:
		return "grouped_dimension"
	default:
		return fmt.Sprintf("unknown dimension type: %d", p)
	}
}

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

type PodSuffixMode int

const (
	PodSuffixModeRandom PodSuffixMode = iota
	PodSuffixModeIncremental
)

func (p PodSuffixMode) Regex() string {
	switch p {
	case PodSuffixModeRandom:
		return "[a-zA-Z0-9]{5}"
	case PodSuffixModeIncremental:
		return "[0-9]+"
	default:
		return ""
	}
}

func NewPrometheus(ctx context.Context, cfg *Config) (*Prometheus, error) {
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
		cfg:            cfg,
		client:         promClient,
		api:            promApi,
		scrapeInterval: defaultScrapeInterval,
	}

	scrapeInterval, err := prom.calculateScrapeInterval(ctx)
	if err == nil {
		prom.scrapeInterval = scrapeInterval
	}

	return &prom, nil
}

func (p *Prometheus) calculateScrapeInterval(ctx context.Context) (time.Duration, error) {
	sampleDuration := 24 * time.Hour
	minV := math.MaxInt

	metrics := []string{"up", "prometheus_ready"}
	metrics = append(metrics, cpuUsageMetrics...)
	metrics = append(metrics, cpuThrottlingThrottledPeriodsMetrics...)
	metrics = append(metrics, cpuThrottlingPeriodsMetrics...)
	metrics = append(metrics, memoryUsageMetrics...)

	for _, metric := range metrics {
		value, _, err := p.api.Query(ctx, fmt.Sprintf("scalar(max(count_over_time(%s[%s])))", metric, model.Duration(sampleDuration).String()), time.Now())
		if err != nil {
			return 0, err
		}
		if value.Type() != model.ValScalar {
			return 0, fmt.Errorf("unexpected response type: %s", value.Type())
		}
		v := value.(*model.Scalar)
		if !v.Value.Equal(model.SampleValue(math.NaN())) && int(v.Value) < minV {
			minV = int(v.Value)
		}
	}

	if minV == math.MaxInt {
		return defaultScrapeInterval, errors.New("could not determine scrape interval")
	}

	return time.Duration(int(sampleDuration) / minV), nil
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

func (p *Prometheus) mergeMultiDimensionalGroupedPrometheusResponse(dim1, dim2 PromDimension, groupBys ...model.LabelName) (PromDimension, error) {
	var err error
	if len(groupBys) == 0 {
		if dim1.promDimensionType() != PromDimensionTypeDatapoint || dim2.promDimensionType() != PromDimensionTypeDatapoint {
			return nil, fmt.Errorf("unexpected dimension type: %d", dim1.promDimensionType())
		}
		datapoints1 := dim1.(PromDatapoints)
		datapoints2 := dim2.(PromDatapoints)
		merged := make([]PromDatapoint, 0, len(datapoints1.Values)+len(datapoints2.Values))
		merged = append(merged, datapoints1.Values...)
		merged = append(merged, datapoints2.Values...)
		sort.Slice(merged, func(i, j int) bool {
			return merged[i].Timestamp.Before(merged[j].Timestamp)
		})
		return PromDatapoints{Values: merged}, nil
	}

	if dim1.promDimensionType() != PromDimensionTypeGroupedDimension || dim2.promDimensionType() != PromDimensionTypeGroupedDimension {
		return nil, fmt.Errorf("unexpected dimension type: d1: %s, d2: %s, expected: %s", dim1.promDimensionType(), dim2.promDimensionType(), PromDimensionTypeGroupedDimension)
	}

	groupedDim1 := dim1.(PromGroupedDimension)
	groupedDim2 := dim2.(PromGroupedDimension)

	merged := PromGroupedDimension{Values: make(map[string]PromDimension)}

	for key, dim1Value := range groupedDim1.Values {
		if dim2Value, ok := groupedDim2.Values[key]; ok {
			merged.Values[key], err = p.mergeMultiDimensionalGroupedPrometheusResponse(dim1Value, dim2Value, groupBys[1:]...)
			if err != nil {
				return nil, err
			}
		} else {
			merged.Values[key] = dim1Value
		}
	}

	for key, dim2Value := range groupedDim2.Values {
		if _, ok := groupedDim1.Values[key]; !ok {
			merged.Values[key] = dim2Value
		}
	}

	return merged, nil
}

func (p *Prometheus) parseMultiDimensionalQueryRange(ctx context.Context, query string, rangeStart, rangeEnd time.Time, step time.Duration, groupBys ...model.LabelName) (PromDimension, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	if rangeEnd.Sub(rangeStart) < time.Hour*24 {
		value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
			Start: rangeStart,
			End:   rangeEnd,
			Step:  step,
		})
		if err != nil {
			return nil, err
		}
		return parseMultiDimensionalGroupedPrometheusResponse(value, groupBys...)
	}

	var result *PromDimension
	for rangeStart.Before(rangeEnd) {
		rangeEndStep := rangeStart.Add(time.Hour * 24)
		if rangeEndStep.After(rangeEnd) {
			rangeEndStep = rangeEnd
		}
		value, _, err := p.api.QueryRange(ctx, query, prometheus.Range{
			Start: rangeStart,
			End:   rangeEndStep,
			Step:  step,
		})
		if err != nil {
			return nil, err
		}

		dim, err := parseMultiDimensionalGroupedPrometheusResponse(value, groupBys...)
		if err != nil {
			return nil, err
		}

		if result == nil {
			result = &dim
		} else {
			*result, err = p.mergeMultiDimensionalGroupedPrometheusResponse(*result, dim, groupBys...)
			if err != nil {
				return nil, err
			}
		}

		rangeStart = rangeEndStep
	}

	if result == nil {
		return nil, errors.New("no data found")
	}
	return *result, nil
}

func (p *Prometheus) GetCpuMetricsForPod(ctx context.Context, namespace, podName string, observabilityDays int) (map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))

	var result map[string][]PromDatapoint
	for _, metric := range cpuUsageMetrics {
		query := fmt.Sprintf(`sum(rate(%s{namespace="%s", pod="%s", container!=""}[%s])) by (container)`, metric, namespace, podName, model.Duration(step).String())
		datapoints, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "container")
		if err != nil {
			return nil, err
		}
		if datapoints.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", datapoints.promDimensionType())
		}

		result = make(map[string][]PromDatapoint)
		hasData := false
		for containerName, promContainerDim := range datapoints.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			if len(promContainerDim.(PromDatapoints).Values) > 0 {
				hasData = true
			}
			result[containerName] = promContainerDim.(PromDatapoints).Values
		}
		if hasData {
			break
		}
	}

	return result, nil
}

func (p *Prometheus) GetCpuMetricsForPodOwnerPrefix(ctx context.Context, namespace, podOwnerPrefix string, observabilityDays int, suffixMode PodSuffixMode) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))

	var result map[string]map[string][]PromDatapoint
	for _, metric := range cpuUsageMetrics {
		pattern := podOwnerPrefix + "-"
		if len(pattern) > 58 {
			pattern = pattern[:58]
		}
		query := fmt.Sprintf(`sum(rate(%s{namespace="%s", pod=~"%s%s$", container!=""}[%s])) by (pod, container)`, metric, namespace, pattern, suffixMode.Regex(), model.Duration(step).String())
		promDims, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "pod", "container")
		if err != nil {
			return nil, err
		}
		if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
		}
		result = make(map[string]map[string][]PromDatapoint)
		hasData := false
		for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
			if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
				return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
			}
			result[podName] = make(map[string][]PromDatapoint)
			for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
				if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
					return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
				}
				if len(promContainerDim.(PromDatapoints).Values) > 0 {
					hasData = true
				}
				result[podName][containerName] = promContainerDim.(PromDatapoints).Values
			}
		}
		if hasData {
			break
		}
	}

	return result, nil
}

func (p *Prometheus) GetMemoryMetricsForPod(ctx context.Context, namespace, podName string, observabilityDays int) (map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))

	var result map[string][]PromDatapoint
	for _, metric := range memoryUsageMetrics {
		query := fmt.Sprintf(`max(%s{namespace="%s", pod="%s", container!=""}) by (container)`, metric, namespace, podName)
		datapoints, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "container")
		if err != nil {
			return nil, err
		}
		if datapoints.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", datapoints.promDimensionType())
		}
		result = make(map[string][]PromDatapoint)
		hasData := false
		for containerName, promContainerDim := range datapoints.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			if len(promContainerDim.(PromDatapoints).Values) > 0 {
				hasData = true
			}
			result[containerName] = promContainerDim.(PromDatapoints).Values
		}
		if hasData {
			break
		}
	}

	return result, nil
}

func (p *Prometheus) GetMemoryMetricsForPodOwnerPrefix(ctx context.Context, namespace, podPrefix string, observabilityDays int, suffixMode PodSuffixMode) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))

	var result map[string]map[string][]PromDatapoint
	for _, metric := range memoryUsageMetrics {
		pattern := podPrefix + "-"
		if len(pattern) > 58 {
			pattern = pattern[:58]
		}

		query := fmt.Sprintf(`max(%s{namespace="%s", pod=~"%s%s$", container!=""}) by (pod, container)`, metric, namespace, pattern, suffixMode.Regex())
		promDims, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "pod", "container")
		if err != nil {
			return nil, err
		}
		if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
		}
		result = make(map[string]map[string][]PromDatapoint)
		hasData := false
		for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
			if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
				return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
			}
			result[podName] = make(map[string][]PromDatapoint)
			for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
				if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
					return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
				}
				if len(promContainerDim.(PromDatapoints).Values) > 0 {
					hasData = true
				}
				result[podName][containerName] = promContainerDim.(PromDatapoints).Values
			}
		}
		if hasData {
			break
		}
	}
	return result, nil
}

func (p *Prometheus) GetCpuThrottlingMetricsForPod(ctx context.Context, namespace, podName string, observabilityDays int) (map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))

	var result map[string][]PromDatapoint
	for i, metric := range cpuThrottlingThrottledPeriodsMetrics {
		query := fmt.Sprintf(`sum(increase(%[4]s{namespace="%[1]s", pod="%[2]s", container!=""}[%[3]s])) by (container) / sum(increase(%[5]s{namespace="%[1]s", pod="%[2]s", container!=""}[%[3]s])) by (container)`, namespace, podName, model.Duration(step).String(), metric, cpuThrottlingPeriodsMetrics[i])
		datapoints, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "container")
		if err != nil {
			return nil, err
		}
		if datapoints.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", datapoints.promDimensionType())
		}
		result = make(map[string][]PromDatapoint)
		hasData := false
		for containerName, promContainerDim := range datapoints.(PromGroupedDimension).Values {
			if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
				return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
			}
			if len(promContainerDim.(PromDatapoints).Values) > 0 {
				hasData = true
			}
			result[containerName] = promContainerDim.(PromDatapoints).Values
		}
		if hasData {
			break
		}
	}

	return result, nil
}

func (p *Prometheus) GetCpuThrottlingMetricsForPodOwnerPrefix(ctx context.Context, namespace, podPrefix string, observabilityDays int, suffixMode PodSuffixMode) (map[string]map[string][]PromDatapoint, error) {
	p.cfg.reconnectWait.Lock()
	p.cfg.reconnectWait.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	step := time.Duration(math.Max(float64(time.Minute), float64(4*p.scrapeInterval)))
	var result map[string]map[string][]PromDatapoint
	for i, metric := range cpuThrottlingThrottledPeriodsMetrics {
		pattern := podPrefix + "-"
		if len(pattern) > 58 {
			pattern = pattern[:58]
		}

		query := fmt.Sprintf(`sum(increase(%[5]s{namespace="%[1]s", pod=~"%[2]s%[3]s$", container!=""}[%[4]s])) by (pod, container) / sum(increase(%[6]s{namespace="%[1]s", pod=~"%[2]s%[3]s", container!=""}[%[4]s])) by (pod, container)`, namespace, pattern, suffixMode.Regex(), model.Duration(step).String(), metric, cpuThrottlingPeriodsMetrics[i])
		promDims, err := p.parseMultiDimensionalQueryRange(ctx, query,
			time.Now().Add(time.Duration(observabilityDays)*-24*time.Hour).Truncate(step),
			time.Now().Truncate(step),
			step, "pod", "container")
		if err != nil {
			return nil, err
		}
		if promDims.promDimensionType() != PromDimensionTypeGroupedDimension {
			return nil, fmt.Errorf("unexpected dimension type: %d", promDims.promDimensionType())
		}
		result = make(map[string]map[string][]PromDatapoint)
		hasData := false
		for podName, promPodDim := range promDims.(PromGroupedDimension).Values {
			if promPodDim.promDimensionType() != PromDimensionTypeGroupedDimension {
				return nil, fmt.Errorf("unexpected dimension type: %d", promPodDim.promDimensionType())
			}
			result[podName] = make(map[string][]PromDatapoint)
			for containerName, promContainerDim := range promPodDim.(PromGroupedDimension).Values {
				if promContainerDim.promDimensionType() != PromDimensionTypeDatapoint {
					return nil, fmt.Errorf("unexpected dimension type: %d", promContainerDim.promDimensionType())
				}
				if len(promContainerDim.(PromDatapoints).Values) > 0 {
					hasData = true
				}
				result[podName][containerName] = promContainerDim.(PromDatapoints).Values
			}
		}
		if hasData {
			break
		}
	}
	return result, nil
}

func (p *Prometheus) Ping(ctx context.Context) error {
	_, _, err := p.api.Query(ctx, "up", time.Now())
	return err
}
