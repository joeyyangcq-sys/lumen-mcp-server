package gatewayclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnalyzeLatency(t *testing.T) {
	metrics := sampleMetrics()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(metrics))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	out, err := c.AnalyzeLatency(context.Background(), map[string]any{"min_samples": 5})
	if err != nil {
		t.Fatalf("AnalyzeLatency() error = %v", err)
	}

	if asInt(out, "entries_total", 0) != 1 {
		t.Fatalf("entries_total=%v want=1", out["entries_total"])
	}
	entries, ok := out["entries"].([]map[string]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("entries shape unexpected: %#v", out["entries"])
	}
	entry := entries[0]
	if asString(entry, "route_id") != "r1" || asString(entry, "upstream_id") != "u1" {
		t.Fatalf("unexpected key: route=%q upstream=%q", asString(entry, "route_id"), asString(entry, "upstream_id"))
	}
	latency := asMapMust(entry, "latency")
	if got := asFloat(latency, "p95_ms", 0); got < 249 || got > 251 {
		t.Fatalf("p95_ms=%v want about 250", got)
	}
	timeout := asMapMust(entry, "recommended_timeout")
	if asInt(timeout, "connect", 0) != 1 || asInt(timeout, "read", 0) != 1 || asInt(timeout, "send", 0) != 1 {
		t.Fatalf("unexpected recommendation: %#v", timeout)
	}
}

func TestTuneUpstreamTimeoutDryRun(t *testing.T) {
	metrics := sampleMetrics()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(metrics))
		case r.Method == http.MethodGet && r.URL.Path == "/apisix/admin/upstreams/u1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"key":"/apisix/upstreams/u1","value":{"id":"u1","timeout":{"connect":3,"send":5,"read":5},"nodes":{"127.0.0.1:9001":1},"type":"roundrobin"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/apisix/admin/upstreams/u1":
			t.Fatal("unexpected PUT in dry-run mode")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	out, err := c.TuneUpstreamTimeout(context.Background(), map[string]any{
		"route_id":    "r1",
		"dry_run":     true,
		"min_samples": 5,
	})
	if err != nil {
		t.Fatalf("TuneUpstreamTimeout() error = %v", err)
	}
	if asBool(out, "updated", true) {
		t.Fatalf("updated=%v want=false", out["updated"])
	}
	if asString(out, "upstream_id") != "u1" {
		t.Fatalf("upstream_id=%q want=u1", asString(out, "upstream_id"))
	}
}

func TestTuneUpstreamTimeoutApply(t *testing.T) {
	metrics := sampleMetrics()
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/metrics":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(metrics))
		case r.Method == http.MethodGet && r.URL.Path == "/apisix/admin/upstreams/u1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"key":"/apisix/upstreams/u1","value":{"id":"u1","timeout":{"connect":3,"send":5,"read":5},"nodes":{"127.0.0.1:9001":1},"type":"roundrobin"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/apisix/admin/upstreams/u1":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &putBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	out, err := c.TuneUpstreamTimeout(context.Background(), map[string]any{
		"upstream_id": "u1",
		"dry_run":     false,
		"min_samples": 5,
	})
	if err != nil {
		t.Fatalf("TuneUpstreamTimeout() error = %v", err)
	}
	if !asBool(out, "updated", false) {
		t.Fatalf("updated=%v want=true", out["updated"])
	}
	timeout := asMapMust(putBody, "timeout")
	if asInt(timeout, "connect", 0) != 1 || asInt(timeout, "read", 0) != 1 || asInt(timeout, "send", 0) != 1 {
		t.Fatalf("timeout payload mismatch: %#v", timeout)
	}
}

func sampleMetrics() string {
	lines := []string{
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total",le="0.05"} 6`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total",le="0.1"} 9`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total",le="0.25"} 10`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total",le="+Inf"} 10`,
		`lumen_upstream_phase_duration_seconds_sum{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total"} 0.88`,
		`lumen_upstream_phase_duration_seconds_count{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="total"} 10`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="connect",le="0.005"} 8`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="connect",le="0.01"} 10`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="connect",le="+Inf"} 10`,
		`lumen_upstream_phase_duration_seconds_sum{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="connect"} 0.032`,
		`lumen_upstream_phase_duration_seconds_count{route_id="r1",upstream_id="u1",service_id="s1",status_class="2xx",phase="connect"} 10`,
		// below min_samples: should be filtered out.
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r2",upstream_id="u2",service_id="s2",status_class="2xx",phase="total",le="0.1"} 1`,
		`lumen_upstream_phase_duration_seconds_bucket{route_id="r2",upstream_id="u2",service_id="s2",status_class="2xx",phase="total",le="+Inf"} 1`,
		`lumen_upstream_phase_duration_seconds_sum{route_id="r2",upstream_id="u2",service_id="s2",status_class="2xx",phase="total"} 0.05`,
		`lumen_upstream_phase_duration_seconds_count{route_id="r2",upstream_id="u2",service_id="s2",status_class="2xx",phase="total"} 1`,
	}
	return strings.Join(lines, "\n")
}
