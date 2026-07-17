package config

import (
	"slices"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TEAMS_TENANT_ID", "TEAMS_CLIENT_ID", "TEAMS_CLIENT_SECRET", "TEAMS_ACCESS_TOKEN",
		"TEAMS_USER_ID", "TEAMS_AUTH_METHOD", "TEAMS_TOKEN_CACHE", "TEAMS_GRAPH_BASE_URL",
		"TEAMS_AUTHORITY_URL", "TEAMS_SCOPES", "TEAMS_MODE", "MCP_TRANSPORT", "MCP_HTTP_ADDR",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDeviceCodeDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("TEAMS_TENANT_ID", "tenant")
	t.Setenv("TEAMS_CLIENT_ID", "client")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthMethod != AuthDeviceCode {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, AuthDeviceCode)
	}
	if cfg.Mode != ModeReadOnly || cfg.Transport != TransportStdio || cfg.HTTPAddr != ":8080" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	wantScopes := []string{"offline_access", "User.Read", "Team.ReadBasic.All", "Channel.ReadBasic.All", "ChannelMessage.Read.All", "Chat.Read"}
	if !slices.Equal(cfg.Scopes, wantScopes) {
		t.Errorf("Scopes = %v, want %v", cfg.Scopes, wantScopes)
	}
}

func TestLoadReadWriteAddsSendScopes(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{
		"--teams-tenant-id=tenant", "--teams-client-id=client", "--mode=readwrite",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !slices.Contains(cfg.Scopes, "ChannelMessage.Send") || !slices.Contains(cfg.Scopes, "ChatMessage.Send") {
		t.Errorf("Scopes = %v, want message send scopes", cfg.Scopes)
	}
	if !cfg.IsReadWrite() {
		t.Error("IsReadWrite() = false, want true")
	}
}

func TestLoadAutoSelectsAccessToken(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{"--teams-access-token=token"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthMethod != AuthAccessToken {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, AuthAccessToken)
	}
}

func TestLoadAutoSelectsClientCredentials(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{
		"--teams-tenant-id=tenant", "--teams-client-id=client", "--teams-client-secret=secret", "--teams-user-id=user@example.com",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthMethod != AuthClientCredentials {
		t.Errorf("AuthMethod = %q, want %q", cfg.AuthMethod, AuthClientCredentials)
	}
}

func TestLoadFlagsOverrideEnvironment(t *testing.T) {
	clearEnv(t)
	t.Setenv("TEAMS_ACCESS_TOKEN", "env-token")
	t.Setenv("TEAMS_MODE", "readonly")
	t.Setenv("MCP_TRANSPORT", "stdio")

	cfg, err := Load([]string{
		"--teams-access-token=flag-token", "--mode=readwrite", "--transport=http", "--http-addr=127.0.0.1:9090",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "flag-token" || cfg.Mode != ModeReadWrite || cfg.Transport != TransportHTTP || cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("flags did not override environment: %+v", cfg)
	}
}

func TestLoadCustomScopes(t *testing.T) {
	clearEnv(t)
	cfg, err := Load([]string{
		"--teams-tenant-id=tenant", "--teams-client-id=client", "--teams-scopes=User.Read,Chat.Read ChatMessage.Send",
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"User.Read", "Chat.Read", "ChatMessage.Send"}
	if !slices.Equal(cfg.Scopes, want) {
		t.Errorf("Scopes = %v, want %v", cfg.Scopes, want)
	}
}

func TestLoadMissingDeviceConfiguration(t *testing.T) {
	clearEnv(t)
	if _, err := Load(nil); err == nil {
		t.Fatal("Load() error = nil, want missing configuration error")
	}
}

func TestLoadClientCredentialsRequiresUser(t *testing.T) {
	clearEnv(t)
	_, err := Load([]string{
		"--auth-method=client-credentials", "--teams-tenant-id=tenant", "--teams-client-id=client", "--teams-client-secret=secret",
	})
	if err == nil {
		t.Fatal("Load() error = nil, want missing user error")
	}
}

func TestLoadClientCredentialsRejectsReadWrite(t *testing.T) {
	clearEnv(t)
	_, err := Load([]string{
		"--auth-method=client-credentials", "--teams-tenant-id=tenant", "--teams-client-id=client",
		"--teams-client-secret=secret", "--teams-user-id=user@example.com", "--mode=readwrite",
	})
	if err == nil {
		t.Fatal("Load() error = nil, want client-credentials readwrite error")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	clearEnv(t)
	_, err := Load([]string{
		"--teams-access-token=token", "--teams-graph-base-url=http://graph.example", "--mode=bogus", "--transport=bogus",
	})
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}
