package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioTransportListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", ".",
		"--teams-access-token=test-token",
		"--mode=readonly",
		"--transport=stdio",
	)
	cmd.Env = os.Environ()

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-test", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	foundRead := false
	for _, tool := range result.Tools {
		if tool.Name == "teams_list_joined_teams" {
			foundRead = true
		}
		if tool.Name == "teams_send_chat_message" {
			t.Errorf("write tool %q is registered in readonly mode", tool.Name)
		}
	}
	if !foundRead {
		t.Error("teams_list_joined_teams is not registered")
	}
}
