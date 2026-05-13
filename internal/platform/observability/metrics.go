package observability

import (
	"expvar"
	"sync/atomic"
	"time"
)

type Metrics struct {
	httpRequests atomic.Uint64
	httpErrors   atomic.Uint64
	toolInvokes  atomic.Uint64
	toolDenials  atomic.Uint64
	latencyMS    atomic.Uint64
}

func New() *Metrics {
	m := &Metrics{}
	expvar.Publish("mcp_http_requests_total", expvar.Func(func() any { return m.httpRequests.Load() }))
	expvar.Publish("mcp_http_errors_total", expvar.Func(func() any { return m.httpErrors.Load() }))
	expvar.Publish("mcp_tool_invokes_total", expvar.Func(func() any { return m.toolInvokes.Load() }))
	expvar.Publish("mcp_tool_denials_total", expvar.Func(func() any { return m.toolDenials.Load() }))
	expvar.Publish("mcp_latency_ms_total", expvar.Func(func() any { return m.latencyMS.Load() }))
	return m
}

func (m *Metrics) ObserveHTTP(status int, d time.Duration) {
	m.httpRequests.Add(1)
	m.latencyMS.Add(uint64(d.Milliseconds()))
	if status >= 400 {
		m.httpErrors.Add(1)
	}
}

func (m *Metrics) IncToolInvoke() {
	m.toolInvokes.Add(1)
}

func (m *Metrics) IncToolDeny() {
	m.toolDenials.Add(1)
}
