// Package auth creates authenticated HTTP clients for Microsoft Graph.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"teams-mcp/internal/config"
)

// DeviceCodePrompt is emitted when interactive device-code authentication is
// required. Message is suitable for writing to stderr.
type DeviceCodePrompt struct {
	VerificationURL string
	UserCode        string
	Message         string
}

// NewHTTPClient returns an HTTP client that adds and refreshes Microsoft Graph
// bearer tokens according to cfg. notify is called during a new device-code
// sign-in and may be nil.
func NewHTTPClient(ctx context.Context, cfg *config.Config, notify func(DeviceCodePrompt)) (*http.Client, error) {
	switch cfg.AuthMethod {
	case config.AuthAuto:
		return nil, fmt.Errorf("teams auth: automatic authentication method was not resolved")
	case config.AuthAccessToken:
		source := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.AccessToken, TokenType: "Bearer"})
		return oauth2.NewClient(ctx, source), nil
	case config.AuthClientCredentials:
		source := (&clientcredentials.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TokenURL:     endpointURL(cfg.AuthorityURL, cfg.TenantID, "oauth2/v2.0/token"),
			Scopes:       []string{graphDefaultScope(cfg.GraphBaseURL)},
		}).TokenSource(ctx)
		return oauth2.NewClient(ctx, source), nil
	case config.AuthDeviceCode:
		return newDeviceCodeClient(ctx, cfg, notify)
	default:
		return nil, fmt.Errorf("teams auth: unsupported authentication method %q", cfg.AuthMethod)
	}
}

func newDeviceCodeClient(ctx context.Context, cfg *config.Config, notify func(DeviceCodePrompt)) (*http.Client, error) {
	oauthConfig := &oauth2.Config{
		ClientID: cfg.ClientID,
		Scopes:   cfg.Scopes,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: endpointURL(cfg.AuthorityURL, cfg.TenantID, "oauth2/v2.0/devicecode"),
			TokenURL:      endpointURL(cfg.AuthorityURL, cfg.TenantID, "oauth2/v2.0/token"),
		},
	}

	token, err := loadToken(cfg.TokenCache, cfg)
	if err != nil {
		return nil, err
	}
	var source oauth2.TokenSource
	if token != nil {
		source = oauthConfig.TokenSource(ctx, token)
		refreshed, refreshErr := source.Token()
		if refreshErr == nil {
			token = refreshed
			if err := saveToken(cfg.TokenCache, cfg, token); err != nil {
				return nil, err
			}
		} else {
			token = nil
			source = nil
		}
	}

	if token == nil {
		deviceAuth, err := oauthConfig.DeviceAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("teams auth: start device-code flow: %w", err)
		}
		if notify != nil {
			message := fmt.Sprintf("Open %s and enter code %s", deviceAuth.VerificationURI, deviceAuth.UserCode)
			notify(DeviceCodePrompt{
				VerificationURL: deviceAuth.VerificationURI,
				UserCode:        deviceAuth.UserCode,
				Message:         message,
			})
		}
		token, err = oauthConfig.DeviceAccessToken(ctx, deviceAuth)
		if err != nil {
			return nil, fmt.Errorf("teams auth: complete device-code flow: %w", err)
		}
		if err := saveToken(cfg.TokenCache, cfg, token); err != nil {
			return nil, err
		}
		source = oauthConfig.TokenSource(ctx, token)
	}

	persisting := &persistingTokenSource{
		source:    source,
		cachePath: cfg.TokenCache,
		cfg:       cfg,
	}
	return oauth2.NewClient(ctx, oauth2.ReuseTokenSource(token, persisting)), nil
}

func endpointURL(authorityURL, tenantID, endpoint string) string {
	return strings.TrimRight(authorityURL, "/") + "/" + url.PathEscape(tenantID) + "/" + endpoint
}

func graphDefaultScope(graphBaseURL string) string {
	u, err := url.Parse(graphBaseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "https://graph.microsoft.com/.default"
	}
	return u.Scheme + "://" + u.Host + "/.default"
}

type cachedToken struct {
	TenantID string        `json:"tenant_id"`
	ClientID string        `json:"client_id"`
	Scopes   []string      `json:"scopes"`
	Token    *oauth2.Token `json:"token"`
}

func loadToken(path string, cfg *config.Config) (*oauth2.Token, error) {
	if path == "" {
		return nil, nil
	}
	//nolint:gosec // The operator explicitly configures the local token-cache path.
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("teams auth: read token cache: %w", err)
	}
	var cached cachedToken
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("teams auth: decode token cache: %w", err)
	}
	if cached.TenantID != cfg.TenantID || cached.ClientID != cfg.ClientID || !slices.Equal(cached.Scopes, cfg.Scopes) {
		return nil, nil
	}
	return cached.Token, nil
}

func saveToken(path string, cfg *config.Config, token *oauth2.Token) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(cachedToken{
		TenantID: cfg.TenantID,
		ClientID: cfg.ClientID,
		Scopes:   cfg.Scopes,
		Token:    token,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("teams auth: encode token cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("teams auth: create token cache directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".token-*.json")
	if err != nil {
		return fmt.Errorf("teams auth: create temporary token cache: %w", err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("teams auth: secure token cache: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("teams auth: write token cache: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("teams auth: close token cache: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		// Windows does not replace an existing destination with os.Rename.
		// Remove the old cache and retry; the temporary file remains secured
		// with user-only permissions throughout.
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("teams auth: remove previous token cache: %w", removeErr)
		}
		if retryErr := os.Rename(tempName, path); retryErr != nil {
			return fmt.Errorf("teams auth: replace token cache: %w", retryErr)
		}
	}
	return nil
}

type persistingTokenSource struct {
	mu        sync.Mutex
	source    oauth2.TokenSource
	cachePath string
	cfg       *config.Config
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if err := saveToken(s.cachePath, s.cfg, token); err != nil {
		return nil, err
	}
	return token, nil
}
