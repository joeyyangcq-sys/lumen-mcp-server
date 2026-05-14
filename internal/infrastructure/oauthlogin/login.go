package oauthlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Config struct {
	Issuer       string
	Audience     string
	Scopes       []string
	ClientID     string
	RegistrationURL string
}

type TokenResult struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func Login(ctx context.Context, cfg Config, logger *slog.Logger) (TokenResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return TokenResult{}, fmt.Errorf("failed to start callback listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	clientID := cfg.ClientID
	if clientID == "" {
		regURL := cfg.RegistrationURL
		if regURL == "" {
			regURL = strings.TrimRight(cfg.Issuer, "/") + "/oauth/register"
		}
		logger.Info("registering dynamic client via DCR", "url", regURL)
		dcr, err := registerClient(ctx, regURL, callbackURL)
		if err != nil {
			listener.Close()
			return TokenResult{}, fmt.Errorf("dynamic client registration failed: %w", err)
		}
		clientID = dcr.ClientID
		logger.Info("DCR succeeded", "client_id", clientID)
	}

	pkce, err := newPKCE()
	if err != nil {
		listener.Close()
		return TokenResult{}, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	state, err := randomState()
	if err != nil {
		listener.Close()
		return TokenResult{}, err
	}

	codeCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", makeCallbackHandler(state, codeCh))
	srv := &http.Server{Handler: mux}

	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := buildAuthorizeURL(cfg.Issuer, clientID, callbackURL, state, pkce, cfg.Scopes, cfg.Audience)
	logger.Info("opening browser for authorization", "url", authURL)
	fmt.Printf("\n  Authorization required. Opening browser...\n  If it doesn't open, visit:\n  %s\n\n", authURL)
	_ = openBrowser(authURL)

	select {
	case <-ctx.Done():
		return TokenResult{}, ctx.Err()
	case result := <-codeCh:
		if result.err != "" {
			return TokenResult{}, fmt.Errorf("authorization denied: %s", result.err)
		}
		logger.Info("received authorization code, exchanging for token")
		return exchangeCode(ctx, cfg.Issuer, clientID, result.code, callbackURL, pkce.Verifier)
	}
}

type callbackResult struct {
	code string
	err  string
}

func makeCallbackHandler(expectedState string, codeCh chan<- callbackResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			codeCh <- callbackResult{err: errMsg + ": " + r.URL.Query().Get("error_description")}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h2>Authorization failed</h2><p>You can close this tab.</p></body></html>")
			return
		}
		state := r.URL.Query().Get("state")
		if state != expectedState {
			codeCh <- callbackResult{err: "state mismatch"}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			codeCh <- callbackResult{err: "missing authorization code"}
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		codeCh <- callbackResult{code: code}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authorization successful</h2><p>You can close this tab.</p></body></html>")
	}
}

func buildAuthorizeURL(issuer, clientID, redirectURI, state string, pkce pkceChallenge, scopes []string, audience string) string {
	base := strings.TrimRight(issuer, "/") + "/oauth/authorize"
	v := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {pkce.Method},
		"scope":                 {strings.Join(scopes, " ")},
	}
	if audience != "" {
		v.Set("audience", audience)
	}
	return base + "?" + v.Encode()
}

func exchangeCode(ctx context.Context, issuer, clientID, code, redirectURI, codeVerifier string) (TokenResult, error) {
	tokenURL := strings.TrimRight(issuer, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResult{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return TokenResult{}, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResult{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return TokenResult{}, fmt.Errorf("token exchange failed: status=%d body=%s", resp.StatusCode, string(data))
	}

	var result TokenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return TokenResult{}, fmt.Errorf("token response parse failed: %w", err)
	}
	return result, nil
}

func randomState() (string, error) {
	pkce, err := newPKCE()
	if err != nil {
		return "", err
	}
	return pkce.Verifier[:16], nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
