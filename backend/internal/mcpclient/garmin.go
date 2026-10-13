package mcpclient

import (
	"context"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Garmin launches the upstream garmin-mcp server as a subprocess and connects
// to it over stdio. There is no per-request credential: the server authenticates
// itself from the OAuth token bundle in its token directory (GARMINTOKENS, or
// the volume mounted into the local container), written once by
// `garmin-mcp-auth` on the laptop. Connect therefore takes no credentials, and
// by design cannot pass any (see childEnv).
type Garmin struct {
	command string
	args    []string
}

// NewGarmin builds a client that runs `command args...` to start the MCP server.
// In local dev that command is a `docker run -i --rm ...` invocation; see
// GARMIN_MCP_COMMAND in .env.example.
func NewGarmin(command string, args ...string) *Garmin {
	return &Garmin{command: command, args: args}
}

// Connect spawns the garmin-mcp subprocess and returns an initialized MCP
// session. The caller must Close the session.
func (g *Garmin) Connect(ctx context.Context) (*mcp.ClientSession, error) {
	return connect(ctx, SourceGarmin, g.command, g.args, childEnv(os.Environ()))
}

// passthroughEnv is the only part of the parent environment a spawned server
// sees. Upstream falls back to an email-and-password Garmin sign-in when its
// tokens fail and GARMIN_EMAIL/GARMIN_PASSWORD are set, and a server-side
// sign-in is the one thing this service must never do (ADR 0003): Garmin
// rate-limits them per IP and a shared cloud egress gets blocked first. So the
// child environment is built from an allowlist, never inherited wholesale. The
// docker entries are what `docker run` itself needs in local dev.
var passthroughEnv = []string{
	"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL",
	"GARMINTOKENS", "GARMIN_IS_CN",
	"DOCKER_HOST", "DOCKER_CONFIG", "DOCKER_CONTEXT",
}

// childEnv filters a parent environment (os.Environ() shape, KEY=value) down to
// passthroughEnv.
func childEnv(parent []string) []string {
	allowed := map[string]bool{}
	for _, k := range passthroughEnv {
		allowed[k] = true
	}
	var out []string
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		if allowed[key] {
			out = append(out, kv)
		}
	}
	return out
}

// ReadOnly reports whether an upstream tool only reads the runner's data. The
// read surface is every tool this returns true for; the rest (writes, deletes,
// uploads, account changes) and the groups a running coach has no use for
// (nutrition, challenges, gear, courses, workout authoring, women's health,
// device administration) are never offered to the model (ADR 0004).
//
// It is a pattern over the name, applied to the live tool list, rather than an
// allowlist of names: upstream ignores unknown names in its own allowlist
// silently, and a pattern denies a brand-new write tool by its shape.
func ReadOnly(name string) bool {
	for _, p := range deniedPrefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	if deniedExact[name] {
		return false
	}
	for _, s := range deniedSubstrings {
		if strings.Contains(name, s) {
			return false
		}
	}
	return true
}

var deniedPrefixes = []string{
	"set_", "add_", "create_", "delete_", "upload_", "remove_", "log_", "update_",
	"download_", "schedule_", "unschedule_", "upsert_",
}

var deniedExact = map[string]bool{
	"request_reload":       true,
	"set_fit_download_dir": true,
	"search_foods":         true,
}

var deniedSubstrings = []string{
	"nutrition", "food", "hydration", "energy_balance",
	"badge", "challenge",
	"gear",
	"course",
	"workout",
	"menstrual", "pregnancy",
	"device_alarms", "device_settings", "device_solar", "blood_pressure",
}
