// Command teams-mcp runs a Model Context Protocol server exposing Microsoft
// Teams tools over stdio or streamable HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"teams-mcp/internal/auth"
	"teams-mcp/internal/buildinfo"
	"teams-mcp/internal/config"
	"teams-mcp/internal/graph"
	"teams-mcp/internal/mcpserver"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		_, err := fmt.Fprintf(os.Stdout, "teams-mcp %s\n", buildinfo.Version)
		return err
	}

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient, err := auth.NewHTTPClient(ctx, cfg, func(prompt auth.DeviceCodePrompt) {
		// log writes to stderr, which is important because stdout carries the
		// MCP protocol when using stdio transport.
		log.Printf("teams-mcp: Microsoft sign-in required: %s", prompt.Message)
	})
	if err != nil {
		return err
	}
	httpClient.Timeout = 30 * time.Second

	graphClient, err := graph.NewClient(cfg.GraphBaseURL, cfg.UserID, httpClient)
	if err != nil {
		return err
	}
	server := mcpserver.NewServer(cfg, graphClient)

	switch cfg.Transport {
	case config.TransportStdio:
		return server.Run(ctx, &mcp.StdioTransport{})
	case config.TransportHTTP:
		return runHTTP(ctx, cfg, server)
	default:
		return errors.New("teams-mcp: unsupported transport: " + string(cfg.Transport))
	}
}

func runHTTP(ctx context.Context, cfg *config.Config, server *mcp.Server) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		//nolint:gosec // %q escapes control characters, preventing log injection
		log.Printf("teams-mcp: listening on %q (mode=%q, auth=%q)", cfg.HTTPAddr, cfg.Mode, cfg.AuthMethod)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
