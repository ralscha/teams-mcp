package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if err := saveToken(path, cfg, &oauth2.Token{AccessToken: "tenant-a-token"}); err != nil {
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

func TestTokenCacheCloudAndScopeIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token.json")
	cfg := config.Config{
		TenantID: "tenant", ClientID: "client", Scopes: []string{"User.Read", "Chat.Read"},
		AuthorityURL: "https://login.microsoftonline.com/", GraphBaseURL: "https://graph.microsoft.com/v1.0/",
	}
	if err := saveToken(path, &cfg, &oauth2.Token{AccessToken: "saved"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*config.Config)
		reuse  bool
	}{
		{"same configuration", func(*config.Config) {}, true},
		{"scope order and duplicates", func(c *config.Config) { c.Scopes = []string{"Chat.Read", "User.Read", "Chat.Read"} }, true},
		{"different scopes", func(c *config.Config) { c.Scopes = []string{"User.Read"} }, false},
		{"different client", func(c *config.Config) { c.ClientID = "other" }, false},
		{"different authority", func(c *config.Config) { c.AuthorityURL = "https://login.microsoftonline.us/" }, false},
		{"different Graph cloud", func(c *config.Config) { c.GraphBaseURL = "https://graph.microsoft.us/v1.0/" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			other := cfg
			tc.change(&other)
			token, err := loadToken(path, &other)
			if err != nil || (token != nil) != tc.reuse {
				t.Errorf("loadToken() = %v, %v, want reuse=%v", token, err, tc.reuse)
			}
		})
	}
}

func TestSaveTokenPreservesDestinationOnRenameFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing-directory")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveToken(path, &config.Config{}, &oauth2.Token{AccessToken: "token"}); err == nil {
		t.Fatal("saveToken replaced an existing directory")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("previous destination was removed: %v", err)
	}
}

func TestDeviceCodeRefreshFailureDoesNotForceSignIn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		errorCode  string
		wantSignIn bool
	}{
		{"temporary outage", http.StatusServiceUnavailable, "temporarily_unavailable", false},
		{"invalid client", http.StatusUnauthorized, "invalid_client", false},
		{"expired refresh token", http.StatusBadRequest, "invalid_grant", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signIn := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/devicecode") {
					signIn = true
					w.WriteHeader(http.StatusBadRequest)
					_, _ = io.WriteString(w, `{"error":"access_denied"}`)
					return
				}
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": tc.errorCode})
			}))
			defer server.Close()
			cfg := &config.Config{
				AuthMethod: config.AuthDeviceCode, TenantID: "tenant", ClientID: "client",
				AuthorityURL: server.URL, TokenCache: filepath.Join(t.TempDir(), "token.json"),
			}
			if err := saveToken(cfg.TokenCache, cfg, &oauth2.Token{
				AccessToken: "expired", RefreshToken: "keep-me", Expiry: time.Now().Add(-time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := NewHTTPClient(t.Context(), cfg, nil); err == nil {
				t.Error("NewHTTPClient should report authentication failure")
			}
			if signIn != tc.wantSignIn {
				t.Errorf("interactive sign-in=%v, want %v", signIn, tc.wantSignIn)
			}
			token, err := loadToken(cfg.TokenCache, cfg)
			if err != nil || token == nil || token.RefreshToken != "keep-me" {
				t.Errorf("cached refresh token was lost: token=%v err=%v", token, err)
			}
		})
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
