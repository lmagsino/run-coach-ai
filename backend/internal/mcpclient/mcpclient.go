// Package mcpclient connects the backend to the Garmin MCP server as a client.
//
// The upstream Taxuspt/garmin_mcp server speaks MCP over stdio. It is launched
// as a subprocess and connected to for each reply (ADR 0004); the backend never
// vendors or patches it.
package mcpclient

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LooksLikeError reports whether a tool result that upstream marked as a
// success is really a failure. garmin_mcp returns most failures as ordinary
// text ("Error retrieving user profile: Garmin login failed. ...") with
// is_error false, so a caller that trusts the flag would count a dead login
// as a clean read.
func LooksLikeError(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, "Error") || strings.HasPrefix(t, "ERROR") ||
		strings.Contains(t, "Garmin login failed") || strings.Contains(t, "OAuth tokens not found")
}

// SourceGarmin names the one data source. It is also the prefix the agent uses
// to namespace tool names (garmin__get_sleep_data), so it must stay short and
// stable; a second source (the roadmap's gateway) would get its own.
const SourceGarmin = "garmin"

// connect runs `command args...` as a subprocess with exactly the given
// environment and returns an initialized MCP session over its stdin/stdout.
// The caller must Close the session.
func connect(ctx context.Context, source, command string, args, env []string) (*mcp.ClientSession, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = env

	client := mcp.NewClient(&mcp.Implementation{Name: "run-coach-ai", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s-mcp: %w", source, err)
	}
	return session, nil
}
