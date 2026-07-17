package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"teams-mcp/internal/config"
)

func TestAccessTokenClient(t *testing.T) {
	cfg := &config.Config{AuthMethod: config.AuthAccessToken, AccessToken: "abc123"}
	client, err := NewHTTPClient(t.Context(), cfg, nil)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer abc123" {
			t.Errorf("Authorization = %q", got)
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_ = resp.Body.Close()
}

func TestGraphDefaultScope(t *testing.T) {
	if got := graphDefaultScope("https://graph.microsoft.us/v1.0/"); got != "https://graph.microsoft.us/.default" {
		t.Errorf("graphDefaultScope() = %q", got)
	}
}

func TestTokenCacheConfigurationIsolation(t *testing.T) {
	path := t.TempDir() + "/token.json"
	cfg := &config.Config{TenantID: "tenant-a", ClientID: "client-a", Scopes: []string{"User.Read"}}
	if err := saveToken(path, cfg, nil); err != nil {
		t.Fatalf("saveToken() error = %v", err)
	}

	other := &config.Config{TenantID: "tenant-b", ClientID: "client-a", Scopes: []string{"User.Read"}}
	token, err := loadToken(path, other)
	if err != nil {
		t.Fatalf("loadToken() error = %v", err)
	}
	if token != nil {
		t.Fatal("loadToken() reused a token from a different tenant")
	}
}

func TestTokenCacheCanBeReplaced(t *testing.T) {
	path := t.TempDir() + "/token.json"
	cfg := &config.Config{TenantID: "tenant", ClientID: "client", Scopes: []string{"User.Read"}}
	if err := saveToken(path, cfg, &oauth2.Token{AccessToken: "first"}); err != nil {
		t.Fatalf("first saveToken() error = %v", err)
	}
	if err := saveToken(path, cfg, &oauth2.Token{AccessToken: "second"}); err != nil {
		t.Fatalf("second saveToken() error = %v", err)
	}
	token, err := loadToken(path, cfg)
	if err != nil {
		t.Fatalf("loadToken() error = %v", err)
	}
	if token == nil || token.AccessToken != "second" {
		t.Fatalf("loadToken() = %+v, want replacement token", token)
	}
}
