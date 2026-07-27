package mcpserver

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/buildinfo"
	"teams-mcp/internal/config"
	"teams-mcp/internal/graph"
)

var readToolNames = []string{
	"teams_get_current_user",
	"teams_get_user",
	"teams_list_users",
	"teams_list_joined_teams",
	"teams_list_channels",
	"teams_get_channel",
	"teams_list_team_members",
	"teams_get_channel_message",
	"teams_list_channel_messages",
	"teams_list_channel_message_replies",
	"teams_list_chats",
	"teams_get_chat",
	"teams_list_chat_members",
	"teams_search_messages",
	"teams_get_chat_message",
	"teams_list_chat_messages",
}

var writeToolNames = []string{
	"teams_create_chat",
	"teams_send_channel_message",
	"teams_update_channel_message",
	"teams_delete_channel_message",
	"teams_reply_to_channel_message",
	"teams_send_chat_message",
	"teams_update_chat_message",
	"teams_delete_chat_message",
}

func testGraphClient(t *testing.T) *graph.Client {
	t.Helper()
	client, err := graph.NewClient("https://graph.microsoft.com/v1.0/", "", nil)
	if err != nil {
		t.Fatalf("graph.NewClient() error = %v", err)
	}
	return client
}

func listToolNames(t *testing.T, server *mcp.Server) map[string]bool {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer func() { _ = serverSession.Close() }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer func() { _ = clientSession.Close() }()
	initializeResult := clientSession.InitializeResult()
	if initializeResult == nil || initializeResult.ServerInfo == nil {
		t.Fatal("client initialization result did not include server information")
	}
	if got := initializeResult.ServerInfo.Version; got != buildinfo.Version {
		t.Errorf("server version = %q, want build version %q", got, buildinfo.Version)
	}

	result, err := clientSession.ListTools(t.Context(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	return names
}

func TestNewServerReadOnly(t *testing.T) {
	server := NewServer(&config.Config{Mode: config.ModeReadOnly}, testGraphClient(t))
	names := listToolNames(t, server)
	for _, name := range readToolNames {
		if !names[name] {
			t.Errorf("read tool %q is not registered", name)
		}
	}
	for _, name := range writeToolNames {
		if names[name] {
			t.Errorf("write tool %q is registered in readonly mode", name)
		}
	}
}

func TestNewServerReadWrite(t *testing.T) {
	server := NewServer(&config.Config{Mode: config.ModeReadWrite}, testGraphClient(t))
	names := listToolNames(t, server)
	for _, name := range append(append([]string{}, readToolNames...), writeToolNames...) {
		if !names[name] {
			t.Errorf("tool %q is not registered in readwrite mode", name)
		}
	}
}

func TestValidateMessage(t *testing.T) {
	if got, err := validateMessage("", "hello"); err != nil || got != "text" {
		t.Errorf("validateMessage() = %q, %v", got, err)
	}
	if _, err := validateMessage("markdown", "hello"); err == nil {
		t.Error("validateMessage() accepted unsupported content type")
	}
	if _, err := validateMessage("text", "  "); err == nil {
		t.Error("validateMessage() accepted empty content")
	}
}
