package oauthlogin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type dcrRequest struct {
	ClientName    string   `json:"client_name"`
	RedirectURIs  []string `json:"redirect_uris"`
	GrantTypes    []string `json:"grant_types"`
	ResponseTypes []string `json:"response_types"`
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method"`
}

type dcrResponse struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
}

func registerClient(ctx context.Context, registrationURL, callbackURL string) (dcrResponse, error) {
	body, err := json.Marshal(dcrRequest{
		ClientName:    "lumen-mcp-stdio",
		RedirectURIs:  []string{callbackURL},
		GrantTypes:    []string{"authorization_code", "refresh_token"},
		ResponseTypes: []string{"code"},
		TokenEndpointAuthMethod: "none",
	})
	if err != nil {
		return dcrResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationURL, bytes.NewReader(body))
	if err != nil {
		return dcrResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return dcrResponse{}, fmt.Errorf("dcr request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return dcrResponse{}, err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return dcrResponse{}, fmt.Errorf("dcr failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	var result dcrResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return dcrResponse{}, fmt.Errorf("dcr response parse failed: %w", err)
	}
	return result, nil
}
