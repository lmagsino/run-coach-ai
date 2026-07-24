// Package mcpclient connects the backend to self-hosted MCP servers as a client.
package mcpclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Strava launches the self-hosted strava-mcp server as a subprocess and connects
// to it over stdio. The athlete's access token is passed to the subprocess via
// the STRAVA_ACCESS_TOKEN environment variable.
type Strava struct {
	command string
	args    []string
}

// NewStrava builds a client that runs `command args...` to start the MCP server.
func NewStrava(command string, args ...string) *Strava {
	return &Strava{command: command, args: args}
}

// Connect spawns the strava-mcp subprocess with the given access token and
// returns an initialized MCP session. The caller must Close the session.
func (s *Strava) Connect(ctx context.Context, accessToken string) (*mcp.ClientSession, error) {
	cmd := exec.Command(s.command, s.args...)
	cmd.Env = append(os.Environ(), "STRAVA_ACCESS_TOKEN="+accessToken)

	client := mcp.NewClient(&mcp.Implementation{Name: "run-coach-ai", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect strava-mcp: %w", err)
	}
	return session, nil
}
