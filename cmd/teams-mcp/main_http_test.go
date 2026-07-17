package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPTransportListTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort() error = %v", err)
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	exe := filepath.Join(t.TempDir(), "teams-mcp.exe")
	//nolint:gosec // Executable and arguments are fixed test inputs.
	// This workspace may not be initialized as a Git repository yet, so VCS
	// stamping is disabled for the temporary smoke-test executable.
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", exe, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(out))
	}

	//nolint:gosec // exe is built by this test and addr is a local ephemeral port.
	cmd := exec.CommandContext(ctx, exe,
		"--teams-access-token=test-token",
		"--mode=readwrite",
		"--transport=http",
		"--http-addr="+addr,
	)
	cmd.Env = os.Environ()
	var logs lockedBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("cmd.Start() error = %v", err)
	}
	exitCh := make(chan error, 1)
	go func() { exitCh <- cmd.Wait() }()
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			<-exitCh
		}
	}()

	endpoint := "http://" + addr
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-test", Version: "0.0.0"}, nil)
	var session *mcp.ClientSession
	deadline := time.Now().Add(20 * time.Second)
	for {
		select {
		case err := <-exitCh:
			t.Fatalf("teams-mcp exited before accepting HTTP connections: %v\n%s", err, logs.String())
		default:
		}
		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("client.Connect() error = %v\n%s", err, logs.String())
		}
		time.Sleep(300 * time.Millisecond)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	foundRead, foundWrite := false, false
	for _, tool := range result.Tools {
		if tool.Name == "teams_list_joined_teams" {
			foundRead = true
		}
		if tool.Name == "teams_send_chat_message" {
			foundWrite = true
		}
	}
	if !foundRead || !foundWrite {
		t.Errorf("tool registration: foundRead=%v foundWrite=%v", foundRead, foundWrite)
	}
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
