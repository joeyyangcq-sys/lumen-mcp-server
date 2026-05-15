package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) Client {
	return Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c Client) InvokeTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	if args == nil {
		args = map[string]any{}
	}

	switch toolName {
	case "analyze_latency":
		return c.AnalyzeLatency(ctx, args)
	case "tune_upstream_timeout":
		return c.TuneUpstreamTimeout(ctx, args)
	}

	return c.invokeGatewayTool(ctx, toolName, args)
}

func (c Client) invokeGatewayTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	reqSpec, err := buildRequest(toolName, args)
	if err != nil {
		return nil, err
	}
	bodyReader := io.Reader(http.NoBody)
	if reqSpec.body != nil {
		raw, err := json.Marshal(reqSpec.body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, reqSpec.method, c.BaseURL+reqSpec.path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if reqSpec.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gateway request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if len(data) == 0 {
		return map[string]any{"status": resp.StatusCode}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("gateway response is not json object: %w", err)
	}
	return payload, nil
}

func (c Client) doRequest(ctx context.Context, method, path string, body map[string]any, accept string) ([]byte, int, error) {
	bodyReader := io.Reader(http.NoBody)
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-API-KEY", c.APIKey)
	if strings.TrimSpace(accept) == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return data, resp.StatusCode, fmt.Errorf("gateway request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, resp.StatusCode, nil
}

type requestSpec struct {
	method string
	path   string
	body   map[string]any
}

func buildRequest(toolName string, args map[string]any) (requestSpec, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch toolName {
	case "list_routes", "list_services", "list_upstreams", "list_plugin_configs", "list_global_rules":
		kind := strings.TrimPrefix(toolName, "list_")
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/" + kind + buildListQuery(args)}, nil
	case "get_route", "get_service", "get_upstream", "get_plugin_config", "get_global_rule":
		kind, id, err := parseKindAndID(toolName, args)
		if err != nil {
			return requestSpec{}, err
		}
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/" + kind + "/" + url.PathEscape(id)}, nil
	case "put_route", "put_service", "put_upstream", "put_plugin_config", "put_global_rule":
		kind, id, body, err := parseWrite(toolName, args)
		if err != nil {
			return requestSpec{}, err
		}
		return requestSpec{method: http.MethodPut, path: "/apisix/admin/" + kind + "/" + url.PathEscape(id), body: body}, nil
	case "patch_route", "patch_service", "patch_upstream", "patch_plugin_config", "patch_global_rule":
		kind, id, body, err := parseWrite(toolName, args)
		if err != nil {
			return requestSpec{}, err
		}
		return requestSpec{method: http.MethodPatch, path: "/apisix/admin/" + kind + "/" + url.PathEscape(id), body: body}, nil
	case "delete_route", "delete_service", "delete_upstream", "delete_plugin_config", "delete_global_rule":
		kind, id, err := parseKindAndID(toolName, args)
		if err != nil {
			return requestSpec{}, err
		}
		return requestSpec{method: http.MethodDelete, path: "/apisix/admin/" + kind + "/" + url.PathEscape(id)}, nil
	case "preview_import":
		body, err := asMap(args, "request")
		if err != nil {
			body = args
		}
		return requestSpec{method: http.MethodPost, path: "/apisix/admin/control/imports/preview", body: body}, nil
	case "apply_import":
		body, err := asMap(args, "request")
		if err != nil {
			body = args
		}
		return requestSpec{method: http.MethodPost, path: "/apisix/admin/control/imports/apply", body: body}, nil
	case "export_bundle":
		q := url.Values{}
		if format := asString(args, "format"); format != "" {
			q.Set("format", format)
		}
		if kinds := asStringSlice(args, "kinds"); len(kinds) > 0 {
			for _, k := range kinds {
				q.Add("kind", k)
			}
		}
		path := "/apisix/admin/control/exports"
		if encoded := q.Encode(); encoded != "" {
			path += "?" + encoded
		}
		return requestSpec{method: http.MethodGet, path: path}, nil
	case "history_list":
		limit := asInt(args, "limit", 10)
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/control/history?limit=" + strconv.Itoa(limit)}, nil
	case "history_rollback":
		id := asString(args, "id")
		if id == "" {
			return requestSpec{}, errors.New("history_rollback requires args.id")
		}
		return requestSpec{method: http.MethodPost, path: "/apisix/admin/control/history/" + url.PathEscape(id) + "/rollback", body: map[string]any{}}, nil
	case "get_schema":
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/control/schema"}, nil
	case "list_plugins":
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/control/plugins"}, nil
	case "get_stats":
		return requestSpec{method: http.MethodGet, path: "/apisix/admin/control/stats"}, nil
	default:
		return requestSpec{}, fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func parseKindAndID(toolName string, args map[string]any) (string, string, error) {
	id := asString(args, "id")
	if id == "" {
		return "", "", errors.New("missing args.id")
	}
	kind := ""
	switch {
	case strings.Contains(toolName, "_route"):
		kind = "routes"
	case strings.Contains(toolName, "_service"):
		kind = "services"
	case strings.Contains(toolName, "_upstream"):
		kind = "upstreams"
	case strings.Contains(toolName, "_plugin_config"):
		kind = "plugin_configs"
	case strings.Contains(toolName, "_global_rule"):
		kind = "global_rules"
	default:
		return "", "", fmt.Errorf("cannot infer resource kind from tool: %s", toolName)
	}
	return kind, id, nil
}

func parseWrite(toolName string, args map[string]any) (string, string, map[string]any, error) {
	kind, id, err := parseKindAndID(toolName, args)
	if err != nil {
		return "", "", nil, err
	}
	body, err := asMap(args, "body")
	if err != nil {
		return "", "", nil, errors.New("missing args.body object")
	}
	return kind, id, body, nil
}

func buildListQuery(args map[string]any) string {
	q := url.Values{}
	if page := asInt(args, "page", 0); page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if size := asInt(args, "page_size", 0); size > 0 {
		q.Set("page_size", strconv.Itoa(size))
	}
	if keyword := asString(args, "keyword"); keyword != "" {
		q.Set("keyword", keyword)
	}
	if raw := q.Encode(); raw != "" {
		return "?" + raw
	}
	return ""
}

func asMap(args map[string]any, key string) (map[string]any, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, errors.New("missing key")
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("not object")
	}
	return m, nil
}

func asString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func asStringSlice(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func asInt(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return i
		}
	}
	return fallback
}
