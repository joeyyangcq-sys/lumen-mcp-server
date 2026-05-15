package gatewayclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type latencySeriesKey struct {
	RouteID    string
	UpstreamID string
	ServiceID  string
}

type histogramSeries struct {
	Buckets map[float64]float64
	Count   float64
	Sum     float64
}

type phaseSeries struct {
	Key   latencySeriesKey
	Phase string
}

type seriesStats struct {
	Count float64
	Mean  float64
	P95   float64
	P99   float64
}

type timeoutRecommendation struct {
	Connect int
	Send    int
	Read    int
}

type tuningOptions struct {
	RouteID           string
	UpstreamID        string
	MinSamples        int
	Quantile          float64
	SafetyFactor      float64
	MinConnectMS      float64
	MinReadMS         float64
	MaxConnectSeconds int
	MaxReadSeconds    int
	IncludeNon2xx     bool
}

func defaultTuningOptions(args map[string]any) tuningOptions {
	opts := tuningOptions{
		RouteID:           asString(args, "route_id"),
		UpstreamID:        asString(args, "upstream_id"),
		MinSamples:        asInt(args, "min_samples", 20),
		Quantile:          asFloat(args, "quantile", 0.95),
		SafetyFactor:      asFloat(args, "safety_factor", 3.0),
		MinConnectMS:      asFloat(args, "min_connect_ms", 300),
		MinReadMS:         asFloat(args, "min_read_ms", 1000),
		MaxConnectSeconds: asInt(args, "max_connect_seconds", 10),
		MaxReadSeconds:    asInt(args, "max_read_seconds", 120),
		IncludeNon2xx:     asBool(args, "include_non_2xx", false),
	}
	if opts.MinSamples < 1 {
		opts.MinSamples = 1
	}
	if opts.Quantile <= 0 || opts.Quantile > 1 {
		opts.Quantile = 0.95
	}
	if opts.SafetyFactor < 1 {
		opts.SafetyFactor = 1
	}
	if opts.MinConnectMS < 100 {
		opts.MinConnectMS = 100
	}
	if opts.MinReadMS < 300 {
		opts.MinReadMS = 300
	}
	if opts.MaxConnectSeconds < 1 {
		opts.MaxConnectSeconds = 1
	}
	if opts.MaxReadSeconds < 1 {
		opts.MaxReadSeconds = 1
	}
	return opts
}

func (c Client) AnalyzeLatency(ctx context.Context, args map[string]any) (map[string]any, error) {
	opts := defaultTuningOptions(args)
	seriesByPhase, err := c.fetchPhaseSeries(ctx, opts)
	if err != nil {
		return nil, err
	}

	totalEntries := buildLatencyEntries(seriesByPhase, opts)
	out := map[string]any{
		"min_samples":         opts.MinSamples,
		"quantile":            opts.Quantile,
		"safety_factor":       opts.SafetyFactor,
		"include_non_2xx":     opts.IncludeNon2xx,
		"route_id_filter":     opts.RouteID,
		"upstream_id_filter":  opts.UpstreamID,
		"entries_total":       len(totalEntries),
		"entries":             totalEntries,
		"next_recommendation": "Use tune_upstream_timeout with route_id or upstream_id to apply a recommendation.",
	}
	if len(totalEntries) == 0 {
		out["message"] = "No latency series matched filters/min_samples. Generate traffic and retry."
	}
	return out, nil
}

func (c Client) TuneUpstreamTimeout(ctx context.Context, args map[string]any) (map[string]any, error) {
	opts := defaultTuningOptions(args)
	dryRun := asBool(args, "dry_run", true)

	seriesByPhase, err := c.fetchPhaseSeries(ctx, opts)
	if err != nil {
		return nil, err
	}
	entries := buildLatencyEntries(seriesByPhase, opts)
	if len(entries) == 0 {
		return map[string]any{
			"dry_run":            dryRun,
			"updated":            false,
			"message":            "No latency data matched filters/min_samples; nothing to tune.",
			"route_id_filter":    opts.RouteID,
			"upstream_id_filter": opts.UpstreamID,
		}, nil
	}

	target, err := chooseTuningTarget(entries, opts)
	if err != nil {
		return nil, err
	}
	recMap, ok := target["recommended_timeout"].(map[string]any)
	if !ok {
		return nil, errors.New("internal error: missing recommended_timeout")
	}
	rec := timeoutRecommendation{
		Connect: asInt(recMap, "connect", 3),
		Send:    asInt(recMap, "send", 10),
		Read:    asInt(recMap, "read", 10),
	}

	upstreamID := strings.TrimSpace(asString(target, "upstream_id"))
	if upstreamID == "" {
		return nil, errors.New("cannot tune timeout: upstream_id is empty on selected series")
	}

	upstreamResp, err := c.invokeGatewayTool(ctx, "get_upstream", map[string]any{"id": upstreamID})
	if err != nil {
		return nil, fmt.Errorf("get_upstream failed: %w", err)
	}
	value, err := asMap(upstreamResp, "value")
	if err != nil {
		return nil, errors.New("get_upstream response missing value object")
	}
	currentTimeout, _ := value["timeout"].(map[string]any)
	before := map[string]any{
		"connect": asNumber(currentTimeout, "connect"),
		"send":    asNumber(currentTimeout, "send"),
		"read":    asNumber(currentTimeout, "read"),
	}
	after := map[string]any{
		"connect": rec.Connect,
		"send":    rec.Send,
		"read":    rec.Read,
	}

	result := map[string]any{
		"dry_run":              dryRun,
		"updated":              false,
		"route_id":             asString(target, "route_id"),
		"upstream_id":          upstreamID,
		"current_timeout":      before,
		"recommended_timeout":  after,
		"latency_summary":      target["latency"],
		"selection_reason":     "Highest p95 series that matched filters.",
		"safety_factor":        opts.SafetyFactor,
		"quantile":             opts.Quantile,
		"min_samples":          opts.MinSamples,
		"include_non_2xx":      opts.IncludeNon2xx,
		"scanned_series_count": len(entries),
	}

	if dryRun {
		result["message"] = "Dry-run only. Set dry_run=false to apply timeout update."
		return result, nil
	}

	value["timeout"] = map[string]any{
		"connect": rec.Connect,
		"send":    rec.Send,
		"read":    rec.Read,
	}
	if _, err := c.invokeGatewayTool(ctx, "put_upstream", map[string]any{"id": upstreamID, "body": value}); err != nil {
		return nil, fmt.Errorf("put_upstream failed: %w", err)
	}

	result["updated"] = true
	result["message"] = "Upstream timeout updated."
	return result, nil
}

func (c Client) fetchPhaseSeries(ctx context.Context, opts tuningOptions) (map[phaseSeries]*histogramSeries, error) {
	body, _, err := c.doRequest(ctx, http.MethodGet, "/metrics", nil, "text/plain")
	if err != nil {
		return nil, err
	}
	return parseLatencySeries(string(body), opts), nil
}

func buildLatencyEntries(seriesByPhase map[phaseSeries]*histogramSeries, opts tuningOptions) []map[string]any {
	aggregateByKey := map[latencySeriesKey]map[string]seriesStats{}
	for series, hist := range seriesByPhase {
		if hist.Count < float64(opts.MinSamples) {
			continue
		}
		stats := summarizeHistogram(hist)
		if _, ok := aggregateByKey[series.Key]; !ok {
			aggregateByKey[series.Key] = map[string]seriesStats{}
		}
		aggregateByKey[series.Key][series.Phase] = stats
	}

	entries := make([]map[string]any, 0, len(aggregateByKey))
	for key, phases := range aggregateByKey {
		total, ok := phases["total"]
		if !ok {
			continue
		}

		phaseStats := map[string]any{}
		for phaseName, st := range phases {
			phaseStats[phaseName] = map[string]any{
				"samples": int(st.Count),
				"mean_ms": round2(st.Mean * 1000),
				"p95_ms":  round2(st.P95 * 1000),
				"p99_ms":  round2(st.P99 * 1000),
			}
		}

		rec := recommendTimeouts(phases["connect"], total, opts)
		entries = append(entries, map[string]any{
			"route_id":    key.RouteID,
			"upstream_id": key.UpstreamID,
			"service_id":  key.ServiceID,
			"latency": map[string]any{
				"samples": int(total.Count),
				"mean_ms": round2(total.Mean * 1000),
				"p95_ms":  round2(total.P95 * 1000),
				"p99_ms":  round2(total.P99 * 1000),
				"phases":  phaseStats,
			},
			"recommended_timeout": map[string]any{
				"connect": rec.Connect,
				"send":    rec.Send,
				"read":    rec.Read,
			},
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		li := asMapMust(entries[i], "latency")
		lj := asMapMust(entries[j], "latency")
		return asFloat(li, "p95_ms", 0) > asFloat(lj, "p95_ms", 0)
	})
	return entries
}

func chooseTuningTarget(entries []map[string]any, opts tuningOptions) (map[string]any, error) {
	for _, entry := range entries {
		routeID := asString(entry, "route_id")
		upstreamID := asString(entry, "upstream_id")
		if opts.RouteID != "" && routeID != opts.RouteID {
			continue
		}
		if opts.UpstreamID != "" && upstreamID != opts.UpstreamID {
			continue
		}
		return entry, nil
	}

	if opts.RouteID != "" || opts.UpstreamID != "" {
		return nil, fmt.Errorf("no latency series matched route_id=%q upstream_id=%q", opts.RouteID, opts.UpstreamID)
	}
	return entries[0], nil
}

func parseLatencySeries(metrics string, opts tuningOptions) map[phaseSeries]*histogramSeries {
	out := map[phaseSeries]*histogramSeries{}
	lines := strings.Split(metrics, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		metricType, labelPart, valueRaw, ok := splitMetricLine(line)
		if !ok {
			continue
		}
		if !strings.HasPrefix(metricType, "lumen_upstream_phase_duration_seconds_") {
			continue
		}
		suffix := strings.TrimPrefix(metricType, "lumen_upstream_phase_duration_seconds_")
		if suffix != "bucket" && suffix != "sum" && suffix != "count" {
			continue
		}

		labels := parseLabels(labelPart)
		if labels["phase"] == "" {
			continue
		}
		routeID := labels["route_id"]
		upstreamID := labels["upstream_id"]
		serviceID := labels["service_id"]
		statusClass := labels["status_class"]

		if opts.RouteID != "" && routeID != opts.RouteID {
			continue
		}
		if opts.UpstreamID != "" && upstreamID != opts.UpstreamID {
			continue
		}
		if !opts.IncludeNon2xx && !strings.HasPrefix(statusClass, "2") {
			continue
		}
		if routeID == "" && upstreamID == "" {
			continue
		}

		value, err := strconv.ParseFloat(valueRaw, 64)
		if err != nil {
			continue
		}
		k := phaseSeries{
			Key: latencySeriesKey{
				RouteID:    routeID,
				UpstreamID: upstreamID,
				ServiceID:  serviceID,
			},
			Phase: labels["phase"],
		}
		series := out[k]
		if series == nil {
			series = &histogramSeries{Buckets: map[float64]float64{}}
			out[k] = series
		}

		switch suffix {
		case "bucket":
			leRaw := labels["le"]
			le := math.Inf(1)
			if leRaw != "" && leRaw != "+Inf" {
				if parsed, err := strconv.ParseFloat(leRaw, 64); err == nil {
					le = parsed
				}
			}
			series.Buckets[le] += value
		case "sum":
			series.Sum += value
		case "count":
			series.Count += value
		}
	}
	return out
}

func summarizeHistogram(series *histogramSeries) seriesStats {
	count := series.Count
	if count <= 0 {
		if inf, ok := series.Buckets[math.Inf(1)]; ok {
			count = inf
		}
	}
	mean := 0.0
	if count > 0 {
		mean = series.Sum / count
	}

	return seriesStats{
		Count: count,
		Mean:  mean,
		P95:   histogramQuantile(series.Buckets, count, 0.95),
		P99:   histogramQuantile(series.Buckets, count, 0.99),
	}
}

func recommendTimeouts(connect, total seriesStats, opts tuningOptions) timeoutRecommendation {
	connectMS := connect.P95 * 1000
	totalMS := total.P95 * 1000
	if connectMS <= 0 {
		connectMS = opts.MinConnectMS
	}
	if totalMS <= 0 {
		totalMS = opts.MinReadMS
	}

	recommendedConnectMS := math.Max(connectMS*opts.SafetyFactor, opts.MinConnectMS)
	recommendedReadMS := math.Max(totalMS*opts.SafetyFactor, opts.MinReadMS)

	connectSec := clampInt(int(math.Ceil(recommendedConnectMS/1000)), 1, opts.MaxConnectSeconds)
	readSec := clampInt(int(math.Ceil(recommendedReadMS/1000)), 1, opts.MaxReadSeconds)
	sendSec := readSec

	return timeoutRecommendation{
		Connect: connectSec,
		Send:    sendSec,
		Read:    readSec,
	}
}

func histogramQuantile(buckets map[float64]float64, count float64, q float64) float64 {
	if len(buckets) == 0 || count <= 0 {
		return 0
	}
	bounds := make([]float64, 0, len(buckets))
	for bound := range buckets {
		bounds = append(bounds, bound)
	}
	sort.Float64s(bounds)

	target := count * q
	if target < 1 {
		target = 1
	}
	lastFinite := 0.0
	for _, bound := range bounds {
		cum := buckets[bound]
		if !math.IsInf(bound, 1) {
			lastFinite = bound
		}
		if cum >= target {
			if math.IsInf(bound, 1) {
				return lastFinite
			}
			return bound
		}
	}
	return lastFinite
}

func splitMetricLine(line string) (metric string, labels string, value string, ok bool) {
	start := strings.Index(line, "{")
	end := strings.LastIndex(line, "}")
	if start <= 0 || end <= start {
		return "", "", "", false
	}
	metric = strings.TrimSpace(line[:start])
	labels = line[start+1 : end]
	rest := strings.TrimSpace(line[end+1:])
	if rest == "" {
		return "", "", "", false
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", "", "", false
	}
	return metric, labels, parts[0], true
}

func parseLabels(raw string) map[string]string {
	out := map[string]string{}
	items := strings.Split(raw, ",")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(parts[1], `"`)
		out[key] = val
	}
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func asFloat(args map[string]any, key string, fallback float64) float64 {
	v, ok := args[key]
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return f
		}
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return f
		}
	}
	return fallback
}

func asBool(args map[string]any, key string, fallback bool) bool {
	v, ok := args[key]
	if !ok || v == nil {
		return fallback
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true", "1", "yes", "y":
			return true
		case "false", "0", "no", "n":
			return false
		}
	}
	return fallback
}

func asNumber(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	if v, ok := m[key]; ok {
		return v
	}
	return nil
}

func asMapMust(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	if v == nil {
		return map[string]any{}
	}
	return v
}
