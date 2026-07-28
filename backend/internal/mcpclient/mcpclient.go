// Package mcpclient connects the backend to self-hosted MCP servers as a client.
//
// Every data source follows the same shape: a command that starts an MCP server
// speaking stdio, launched as a subprocess and connected to per request. Strava
// runs our own Go server (cmd/strava-mcp); Garmin runs the upstream
// Taxuspt/garmin_mcp container (deploy/garmin-mcp).
package mcpclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Source names. These are also the prefixes the agent uses to namespace tool
// names, so they must stay short and stable.
const (
	SourceStrava = "strava"
	SourceGarmin = "garmin"
)

// connect runs `command args...` as a subprocess with extraEnv appended to the
// inherited environment, and returns an initialized MCP session over its
// stdin/stdout. The caller must Close the session.
func connect(ctx context.Context, source, command string, args, extraEnv []string) (*mcp.ClientSession, error) {
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(), extraEnv...)

	client := mcp.NewClient(&mcp.Implementation{Name: "run-coach-ai", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect %s-mcp: %w", source, err)
	}
	return session, nil
}
