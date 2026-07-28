package mcpclient

import (
	"context"

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
	return connect(ctx, SourceStrava, s.command, s.args, []string{"STRAVA_ACCESS_TOKEN=" + accessToken})
}
