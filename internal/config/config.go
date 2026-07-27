// Package config loads and validates teams-mcp server configuration from
// environment variables and command-line flags.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Mode controls whether write tools are registered on the MCP server.
type Mode string

const (
	ModeReadOnly  Mode = "readonly"
	ModeReadWrite Mode = "readwrite"
)

// Transport selects how the MCP server communicates with clients.
type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

// AuthMethod selects how the server gets a Microsoft Graph access token.
type AuthMethod string

const (
	AuthAuto              AuthMethod = "auto"
	AuthDeviceCode        AuthMethod = "device-code"
	AuthClientCredentials AuthMethod = "client-credentials"
	AuthAccessToken       AuthMethod = "access-token"
)

const (
	defaultGraphBaseURL = "https://graph.microsoft.com/v1.0/"
	defaultAuthorityURL = "https://login.microsoftonline.com/"
)

var readOnlyScopes = []string{
	"offline_access",
	"User.Read",
	"User.ReadBasic.All",
	"Team.ReadBasic.All",
	"TeamMember.Read.All",
	"Channel.ReadBasic.All",
	"ChannelMessage.Read.All",
	"Chat.Read",
	"ChatMember.Read",
}

// Config holds all settings needed to run the teams-mcp server.
type Config struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	AccessToken  string
	UserID       string
	AuthMethod   AuthMethod
	Scopes       []string
	TokenCache   string

	GraphBaseURL string
	AuthorityURL string

	Mode      Mode
	Transport Transport
	HTTPAddr  string
}

// Load builds a Config from environment variables, then applies overrides
// from args (excluding the program name). Flags take precedence over the
// corresponding environment variables.
func Load(args []string) (*Config, error) {
	cfg := &Config{
		TenantID:     os.Getenv("TEAMS_TENANT_ID"),
		ClientID:     os.Getenv("TEAMS_CLIENT_ID"),
		ClientSecret: os.Getenv("TEAMS_CLIENT_SECRET"),
		AccessToken:  os.Getenv("TEAMS_ACCESS_TOKEN"),
		UserID:       os.Getenv("TEAMS_USER_ID"),
		AuthMethod:   AuthAuto,
		TokenCache:   defaultTokenCache(),
		GraphBaseURL: defaultGraphBaseURL,
		AuthorityURL: defaultAuthorityURL,
		Mode:         ModeReadOnly,
		Transport:    TransportStdio,
		HTTPAddr:     ":8080",
	}

	if v := os.Getenv("TEAMS_AUTH_METHOD"); v != "" {
		cfg.AuthMethod = AuthMethod(v)
	}
	if v := os.Getenv("TEAMS_TOKEN_CACHE"); v != "" {
		cfg.TokenCache = v
	}
	if v := os.Getenv("TEAMS_GRAPH_BASE_URL"); v != "" {
		cfg.GraphBaseURL = v
	}
	if v := os.Getenv("TEAMS_AUTHORITY_URL"); v != "" {
		cfg.AuthorityURL = v
	}
	if v := os.Getenv("TEAMS_MODE"); v != "" {
		cfg.Mode = Mode(v)
	}
	if v := os.Getenv("MCP_TRANSPORT"); v != "" {
		cfg.Transport = Transport(v)
	}
	if v := os.Getenv("MCP_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}

	scopesValue := os.Getenv("TEAMS_SCOPES")
	fs := flag.NewFlagSet("teams-mcp", flag.ContinueOnError)
	tenantID := fs.String("teams-tenant-id", cfg.TenantID, "Microsoft Entra tenant id or domain")
	clientID := fs.String("teams-client-id", cfg.ClientID, "Microsoft Entra application (client) id")
	clientSecret := fs.String("teams-client-secret", cfg.ClientSecret, "client secret for application authentication")
	accessToken := fs.String("teams-access-token", cfg.AccessToken, "pre-acquired Microsoft Graph access token")
	userID := fs.String("teams-user-id", cfg.UserID, "target user id or UPN; required for client-credentials auth")
	authMethod := fs.String("auth-method", string(cfg.AuthMethod), "Authentication: auto, device-code, client-credentials, or access-token")
	scopes := fs.String("teams-scopes", scopesValue, "delegated OAuth scopes separated by spaces or commas")
	tokenCache := fs.String("teams-token-cache", cfg.TokenCache, "device-code token cache path; empty disables persistence")
	graphBaseURL := fs.String("teams-graph-base-url", cfg.GraphBaseURL, "Microsoft Graph API base URL")
	authorityURL := fs.String("teams-authority-url", cfg.AuthorityURL, "Microsoft identity authority base URL")
	mode := fs.String("mode", string(cfg.Mode), "Server mode: readonly or readwrite")
	transport := fs.String("transport", string(cfg.Transport), "Transport: stdio or http")
	httpAddr := fs.String("http-addr", cfg.HTTPAddr, "Address to listen on when --transport=http")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.TenantID = strings.TrimSpace(*tenantID)
	cfg.ClientID = strings.TrimSpace(*clientID)
	cfg.ClientSecret = *clientSecret
	cfg.AccessToken = strings.TrimSpace(*accessToken)
	cfg.UserID = strings.TrimSpace(*userID)
	cfg.AuthMethod = AuthMethod(*authMethod)
	cfg.TokenCache = *tokenCache
	cfg.GraphBaseURL = ensureTrailingSlash(strings.TrimSpace(*graphBaseURL))
	cfg.AuthorityURL = ensureTrailingSlash(strings.TrimSpace(*authorityURL))
	cfg.Mode = Mode(*mode)
	cfg.Transport = Transport(*transport)
	cfg.HTTPAddr = *httpAddr
	cfg.Scopes = parseScopes(*scopes)

	if cfg.AuthMethod == AuthAuto {
		switch {
		case cfg.AccessToken != "":
			cfg.AuthMethod = AuthAccessToken
		case cfg.ClientSecret != "":
			cfg.AuthMethod = AuthClientCredentials
		default:
			cfg.AuthMethod = AuthDeviceCode
		}
	}
	if len(cfg.Scopes) == 0 && cfg.AuthMethod == AuthDeviceCode {
		cfg.Scopes = defaultScopes(cfg.Mode)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultScopes(mode Mode) []string {
	scopes := slices.Clone(readOnlyScopes)
	if mode == ModeReadWrite {
		// ReadWrite scopes cover sending, editing, and soft-deleting messages,
		// plus creating chats.
		scopes = append(scopes,
			"ChannelMessage.Send",
			"ChannelMessage.ReadWrite",
			"ChatMessage.Send",
			"Chat.ReadWrite",
			"Chat.Create",
		)
	}
	return scopes
}

func parseScopes(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	})
}

func defaultTokenCache() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "teams-mcp", "token.json")
}

func ensureTrailingSlash(value string) string {
	if value == "" || strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

func validateHTTPSURL(name, value string) error {
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", name, err)
	}
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%s must be an absolute https URL", name)
	}
	return nil
}

func (c *Config) validate() error {
	var errs []string
	if err := validateHTTPSURL("TEAMS_GRAPH_BASE_URL", c.GraphBaseURL); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateHTTPSURL("TEAMS_AUTHORITY_URL", c.AuthorityURL); err != nil {
		errs = append(errs, err.Error())
	}

	switch c.AuthMethod {
	case AuthAuto:
		errs = append(errs, "automatic authentication method was not resolved")
	case AuthDeviceCode:
		if c.TenantID == "" {
			errs = append(errs, "TEAMS_TENANT_ID (or --teams-tenant-id) is required for device-code authentication")
		}
		if c.ClientID == "" {
			errs = append(errs, "TEAMS_CLIENT_ID (or --teams-client-id) is required for device-code authentication")
		}
	case AuthClientCredentials:
		if c.TenantID == "" {
			errs = append(errs, "TEAMS_TENANT_ID (or --teams-tenant-id) is required for client-credentials authentication")
		}
		if c.ClientID == "" {
			errs = append(errs, "TEAMS_CLIENT_ID (or --teams-client-id) is required for client-credentials authentication")
		}
		if c.ClientSecret == "" {
			errs = append(errs, "TEAMS_CLIENT_SECRET (or --teams-client-secret) is required for client-credentials authentication")
		}
		if c.UserID == "" {
			errs = append(errs, "TEAMS_USER_ID (or --teams-user-id) is required for client-credentials authentication")
		}
		if c.Mode == ModeReadWrite {
			errs = append(errs, "client-credentials authentication cannot use readwrite mode: normal Teams message sends require delegated authentication")
		}
	case AuthAccessToken:
		if c.AccessToken == "" {
			errs = append(errs, "TEAMS_ACCESS_TOKEN (or --teams-access-token) is required for access-token authentication")
		}
	default:
		errs = append(errs, fmt.Sprintf("invalid authentication method %q", c.AuthMethod))
	}

	switch c.Mode {
	case ModeReadOnly, ModeReadWrite:
	default:
		errs = append(errs, fmt.Sprintf("invalid mode %q: must be %q or %q", c.Mode, ModeReadOnly, ModeReadWrite))
	}
	switch c.Transport {
	case TransportStdio, TransportHTTP:
	default:
		errs = append(errs, fmt.Sprintf("invalid transport %q: must be %q or %q", c.Transport, TransportStdio, TransportHTTP))
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// IsReadWrite reports whether write tools should be registered.
func (c *Config) IsReadWrite() bool { return c.Mode == ModeReadWrite }
