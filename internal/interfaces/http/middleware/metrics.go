package middleware

import (
	"net/http"
	"time"

	"github.com/joey/lumen-mcp-server/internal/platform/observability"
)

func Metrics(m *observability.Metrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			m.ObserveHTTP(rec.status, time.Since(start))
		})
	}
}
