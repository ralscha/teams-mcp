package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/buildinfo"
	"teams-mcp/internal/config"
	"teams-mcp/internal/graph"
)

// NewServer builds an MCP server exposing Microsoft Teams tools backed by
// client. Write tools are registered only in readwrite mode.
func NewServer(cfg *config.Config, client *graph.Client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "teams-mcp",
		Version: buildinfo.Version,
	}, nil)

	registerReadTools(server, client)
	if cfg.IsReadWrite() {
		registerWriteTools(server, client)
	}
	return server
}
