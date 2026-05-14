package oauthlogin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCallbackHandler_Success(t *testing.T) {
	codeCh := make(chan callbackResult, 1)
	handler := makeCallbackHandler("test-state", codeCh)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=abc123&state=test-state", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	result := <-codeCh
	if result.code != "abc123" {
		t.Errorf("expected code=abc123, got %s", result.code)
	}
	if result.err != "" {
		t.Errorf("unexpected error: %s", result.err)
	}
}

func TestCallbackHandler_StateMismatch(t *testing.T) {
	codeCh := make(chan callbackResult, 1)
	handler := makeCallbackHandler("expected-state", codeCh)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=abc&state=wrong-state", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	result := <-codeCh
	if result.err != "state mismatch" {
		t.Errorf("expected 'state mismatch', got %s", result.err)
	}
}

func TestCallbackHandler_OAuthError(t *testing.T) {
	codeCh := make(chan callbackResult, 1)
	handler := makeCallbackHandler("s", codeCh)

	req := httptest.NewRequest(http.MethodGet, "/callback?error=access_denied&error_description=user+denied", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	result := <-codeCh
	if result.err != "access_denied: user denied" {
		t.Errorf("unexpected error: %s", result.err)
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	pkce := pkceChallenge{Verifier: "v", Challenge: "c", Method: "S256"}
	got := buildAuthorizeURL("http://auth.example.com", "client-1", "http://localhost:1234/callback", "state-xyz", pkce, []string{"mcp:read", "mcp:write"}, "lumen-mcp")

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if u.Host != "auth.example.com" {
		t.Errorf("unexpected host: %s", u.Host)
	}
	if u.Path != "/oauth/authorize" {
		t.Errorf("unexpected path: %s", u.Path)
	}
	q := u.Query()
	if q.Get("response_type") != "code" {
		t.Errorf("expected response_type=code, got %s", q.Get("response_type"))
	}
	if q.Get("client_id") != "client-1" {
		t.Errorf("expected client_id=client-1, got %s", q.Get("client_id"))
	}
	if q.Get("code_challenge") != "c" {
		t.Errorf("expected code_challenge=c, got %s", q.Get("code_challenge"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("expected S256, got %s", q.Get("code_challenge_method"))
	}
	if q.Get("audience") != "lumen-mcp" {
		t.Errorf("expected audience=lumen-mcp, got %s", q.Get("audience"))
	}
}

func TestExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", 400)
			return
		}
		if r.FormValue("grant_type") != "authorization_code" {
			http.Error(w, "bad grant_type", 400)
			return
		}
		if r.FormValue("code") != "test-code" {
			http.Error(w, "bad code", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at-123",
			"token_type":   "Bearer",
			"expires_in":   900,
		})
	}))
	defer srv.Close()

	result, err := exchangeCode(context.Background(), srv.URL, "client-1", "test-code", "http://localhost/callback", "verifier")
	if err != nil {
		t.Fatalf("exchangeCode() error = %v", err)
	}
	if result.AccessToken != "at-123" {
		t.Errorf("expected access_token=at-123, got %s", result.AccessToken)
	}
	if result.TokenType != "Bearer" {
		t.Errorf("expected token_type=Bearer, got %s", result.TokenType)
	}
}

func TestExchangeCode_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", 500)
	}))
	defer srv.Close()

	_, err := exchangeCode(context.Background(), srv.URL, "c", "code", "http://localhost/callback", "v")
	if err == nil {
		t.Fatal("expected error from 500 response")
	}
}
