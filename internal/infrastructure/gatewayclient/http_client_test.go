package gatewayclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestInvokeTool_MapsCRUDAndControlEndpoints(t *testing.T) {
	t.Run("list routes", func(t *testing.T) {
		assertToolRequestWithQuery(t, "list_routes", map[string]any{"page": 2, "page_size": 50, "keyword": "users"}, http.MethodGet, "/apisix/admin/routes", map[string]string{"page": "2", "page_size": "50", "keyword": "users"}, nil)
	})

	t.Run("put route", func(t *testing.T) {
		assertToolRequestWithQuery(t, "put_route", map[string]any{"id": "route-1", "body": map[string]any{"uri": "/demo", "upstream_id": "up-1"}}, http.MethodPut, "/apisix/admin/routes/route-1", nil, map[string]any{"uri": "/demo", "upstream_id": "up-1"})
	})

	t.Run("delete service", func(t *testing.T) {
		assertToolRequestWithQuery(t, "delete_service", map[string]any{"id": "svc-1"}, http.MethodDelete, "/apisix/admin/services/svc-1", nil, nil)
	})

	t.Run("preview import", func(t *testing.T) {
		assertToolRequestWithQuery(t, "preview_import", map[string]any{"bundle": "x", "prune": true}, http.MethodPost, "/apisix/admin/control/imports/preview", nil, map[string]any{"bundle": "x", "prune": true})
	})

	t.Run("history rollback", func(t *testing.T) {
		assertToolRequestWithQuery(t, "history_rollback", map[string]any{"id": "h-1"}, http.MethodPost, "/apisix/admin/control/history/h-1/rollback", nil, map[string]any{})
	})
}

func assertToolRequestWithQuery(
	t *testing.T,
	tool string,
	args map[string]any,
	wantMethod, wantPath string,
	wantQuery map[string]string,
	wantBody map[string]any,
) {
	t.Helper()
	var gotMethod, gotPath, gotAPIKey string
	var gotQuery url.Values
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		gotAPIKey = r.Header.Get("X-API-KEY")
		if r.Body != nil {
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key")
	_, err := c.InvokeTool(context.Background(), tool, args)
	if err != nil {
		t.Fatalf("InvokeTool() error = %v", err)
	}
	if gotMethod != wantMethod {
		t.Fatalf("method=%s want=%s", gotMethod, wantMethod)
	}
	if gotPath != wantPath {
		t.Fatalf("path=%s want=%s", gotPath, wantPath)
	}
	for k, v := range wantQuery {
		if got := gotQuery.Get(k); got != v {
			t.Fatalf("query[%s]=%s want=%s", k, got, v)
		}
	}
	if gotAPIKey != "test-key" {
		t.Fatalf("x-api-key=%s want=test-key", gotAPIKey)
	}

	if wantBody == nil {
		if len(gotBody) != 0 {
			t.Fatalf("expected empty body, got=%s", string(gotBody))
		}
		return
	}
	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("decode body: %v (%s)", err, string(gotBody))
	}
	if len(got) != len(wantBody) {
		t.Fatalf("body fields mismatch got=%v want=%v", got, wantBody)
	}
	for k, v := range wantBody {
		if got[k] != v {
			t.Fatalf("body[%s]=%v want=%v", k, got[k], v)
		}
	}
}

func TestInvokeTool_UnsupportedTool(t *testing.T) {
	c := New("http://localhost:1", "k")
	_, err := c.InvokeTool(context.Background(), "not_supported", map[string]any{})
	if err == nil {
		t.Fatal("expected error")
	}
}
