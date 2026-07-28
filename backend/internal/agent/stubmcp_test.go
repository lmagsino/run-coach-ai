package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This file provides in-process stub MCP servers so the agent's tool plumbing
// and cross-source behaviour can be tested without Docker, credentials, or any
// network call. Real Strava/Garmin verification happens in Phase 5.

// stubTool is one canned tool on a stub MCP server: every call returns the same
// text, regardless of arguments.
type stubTool struct {
	name        string
	description string
	result      string
}

// stubCall records one tool invocation a stub server received.
type stubCall struct {
	Tool string
	Args map[string]any
}

// callLog collects the calls a stub server received. Safe for concurrent use
// because the MCP session runs its handlers on its own goroutine.
type callLog struct {
	mu    sync.Mutex
	calls []stubCall
}

func (l *callLog) add(c stubCall) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, c)
}

// Calls returns a copy of the recorded calls, in the order they arrived.
func (l *callLog) Calls() []stubCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]stubCall(nil), l.calls...)
}

// Tools returns just the names of the tools that were called.
func (l *callLog) Tools() []string {
	var out []string
	for _, c := range l.Calls() {
		out = append(out, c.Tool)
	}
	return out
}

// startStub runs an in-process MCP server exposing tools over an in-memory
// transport and returns a connected client session plus the log of calls it
// receives. The session is closed when the test ends.
func startStub(t *testing.T, serverName string, tools ...stubTool) (*mcp.ClientSession, *callLog) {
	t.Helper()

	log := &callLog{}
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: "test"}, nil)

	for _, tool := range tools {
		result := tool.result
		name := tool.name
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: tool.description},
			func(_ context.Context, _ *mcp.CallToolRequest, in map[string]any) (*mcp.CallToolResult, any, error) {
				log.add(stubCall{Tool: name, Args: in})
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: result}},
				}, nil, nil
			})
	}

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// The server must be connected before the client: the client initializes
	// the MCP session as part of connecting.
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect stub %s server: %v", serverName, err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect stub %s client: %v", serverName, err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session, log
}

// jsonTool builds a stubTool whose result is v marshalled as JSON — the shape
// real MCP tools return, so Claude sees realistic tool output.
func jsonTool(t *testing.T, name, description string, v any) stubTool {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal stub result for %s: %v", name, err)
	}
	return stubTool{name: name, description: description, result: string(b)}
}
