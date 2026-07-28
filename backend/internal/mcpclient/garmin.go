package mcpclient

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultGarminTools is the allowlist of garmin_mcp tools we expose to Claude.
//
// Upstream ships 110+ tools (nutrition, badges, gear, workout authoring, …).
// Sending all of them on every request would cost a lot of tokens and give the
// model a large surface of irrelevant choices, so we narrow it to what a
// running coach reasons over: training sessions plus the recovery and
// physiology signals Strava has no equivalent of.
//
// garmin_mcp reads this as GARMIN_ENABLED_TOOLS and silently ignores names it
// does not recognise, so a typo here shows up as a missing tool rather than an
// error. Phase 5 checks the resolved list against the live container.
var DefaultGarminTools = []string{
	"get_activities",
	"get_sleep_data",
	"get_heart_rate_variability_summary",
	"get_hrv_trend",
	"get_stress_summary",
	"get_daily_stress",
	"get_training_load_trend",
	"get_vo2_max_trend",
	"get_body_composition",
	"get_steps_data",
}

// Garmin launches the garmin_mcp server as a subprocess and connects to it over
// stdio. Unlike Strava there is no per-request token: the container
// authenticates itself from OAuth tokens cached in a mounted volume (written
// once by `garmin-mcp-auth`), so Connect takes no credentials.
type Garmin struct {
	command string
	args    []string

	// EnabledTools is passed through as GARMIN_ENABLED_TOOLS. Empty means no
	// filtering — every upstream tool is exposed.
	EnabledTools []string
}

// NewGarmin builds a client that runs `command args...` to start the MCP server.
// In local dev that command is a `docker run -i --rm ...` invocation; see
// GARMIN_MCP_COMMAND in .env.example.
func NewGarmin(command string, args ...string) *Garmin {
	return &Garmin{command: command, args: args, EnabledTools: DefaultGarminTools}
}

// Connect spawns the garmin_mcp subprocess and returns an initialized MCP
// session. The caller must Close the session.
func (g *Garmin) Connect(ctx context.Context) (*mcp.ClientSession, error) {
	var env []string
	if len(g.EnabledTools) > 0 {
		env = append(env, "GARMIN_ENABLED_TOOLS="+strings.Join(g.EnabledTools, ","))
	}
	return connect(ctx, SourceGarmin, g.command, g.args, env)
}
