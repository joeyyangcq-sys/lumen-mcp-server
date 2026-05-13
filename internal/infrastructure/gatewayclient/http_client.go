package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) Client {
	return Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c Client) InvokeTool(ctx context.Context, toolName string, args map[string]any) (map[string]any, error) {
	_ = toolName
	_ = args
	// TODO: map toolName -> gateway admin/control endpoint and pass validated args.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/apisix/admin/control/schema", bytes.NewReader(nil))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("gateway request failed")
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":      true,
		"tool":    toolName,
		"preview": payload,
	}, nil
}
