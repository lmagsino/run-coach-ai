package mcpclient

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestStravaMCPListTools verifies the MCP plumbing end-to-end without needing
// Strava credentials: it builds the strava-mcp server, connects to it over
// stdio with a dummy token, and confirms the list_activities tool is exposed.
// (Tool discovery is pure MCP and never calls the Strava API.)
func TestStravaMCPListTools(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..") // backend/

	bin := filepath.Join(t.TempDir(), "strava-mcp")
	build := exec.Command("go", "build", "-o", bin, "./cmd/strava-mcp")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build strava-mcp: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := NewStrava(bin).Connect(ctx, "dummy-token")
	if err != nil {
		t.Fatalf("connect strava-mcp: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	for _, tool := range tools.Tools {
		if tool.Name == "list_activities" {
			return // success
		}
	}
	t.Fatalf("expected a list_activities tool, got %+v", tools.Tools)
}
