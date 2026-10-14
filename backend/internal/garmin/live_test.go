package garmin

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
)

// TestLiveToolSurface runs the real garmin-mcp with a real token bundle and
// checks the read surface against what upstream actually serves, not against
// the recorded list alone: the live list must match deploy/garmin-mcp/tools.txt
// (or the pin moved and the list needs regenerating), every tool the filter
// lets through must be a read, and one profile read must succeed. It makes one
// Garmin call and no model call. Opt in:
//
//	RUNCOACH_LIVE_GARMIN_TESTS=1 go test ./internal/garmin/ -run LiveToolSurface -v
//
// GARMIN_MCP_COMMAND (default: the local Docker image) and
// RUNCOACH_LIVE_GARMIN_BUNDLE_FILE (default: backend/.garmin_bundle.b64) say
// what to run and with which bundle.
func TestLiveToolSurface(t *testing.T) {
	if os.Getenv("RUNCOACH_LIVE_GARMIN_TESTS") != "1" {
		t.Skip("set RUNCOACH_LIVE_GARMIN_TESTS=1 to run against the real garmin-mcp")
	}
	_, thisFile, _, _ := runtime.Caller(0)
	backend := filepath.Join(filepath.Dir(thisFile), "..", "..")

	bundleFile := os.Getenv("RUNCOACH_LIVE_GARMIN_BUNDLE_FILE")
	if bundleFile == "" {
		bundleFile = filepath.Join(backend, ".garmin_bundle.b64")
	}
	raw, err := os.ReadFile(bundleFile)
	if err != nil {
		t.Fatalf("read the bundle: %v", err)
	}
	bundle, err := NormalizeBundle(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := TokenStore(bundle)
	if err != nil {
		t.Fatal(err)
	}
	command := os.Getenv("GARMIN_MCP_COMMAND")
	if command == "" {
		command = "docker run -i --rm -e GARMINTOKENS run-coach-ai/garmin-mcp:latest"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	start := time.Now()
	session, err := (&LocalSpawn{Command: command}).client("GARMINTOKENS=" + tokens).Connect(ctx)
	if err != nil {
		t.Fatalf("spawn %q: %v", command, err)
	}
	defer session.Close()
	spawn := time.Since(start)

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	var live []string
	for _, tool := range res.Tools {
		live = append(live, tool.Name)
	}
	slices.Sort(live)

	recorded := recordedTools(t, filepath.Join(backend, "..", "deploy", "garmin-mcp", "tools.txt"))
	var added, missing []string
	for _, n := range live {
		if !slices.Contains(recorded, n) {
			added = append(added, n)
		}
	}
	for _, n := range recorded {
		if !slices.Contains(live, n) {
			missing = append(missing, n)
		}
	}
	if len(added)+len(missing) > 0 {
		t.Errorf("live tools differ from tools.txt (regenerate it if the pin moved): added %v, missing %v", added, missing)
	}

	var exposed []string
	for _, n := range live {
		if mcpclient.ReadOnly(n) {
			exposed = append(exposed, n)
			if !strings.HasPrefix(n, "get_") && n != "count_activities" {
				t.Errorf("the filter lets through %q, which is not a read", n)
			}
		}
	}

	probeStart := time.Now()
	name, err := probe(ctx, session)
	if err != nil {
		t.Fatalf("profile read with the bundle: %v", err)
	}
	t.Logf("spawn %s, %d live tools, %d exposed, profile read in %s (name %q)",
		spawn.Round(time.Millisecond), len(live), len(exposed), time.Since(probeStart).Round(time.Millisecond), name)
}

func recordedTools(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if n := strings.TrimSpace(sc.Text()); n != "" && !strings.HasPrefix(n, "#") {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return names
}
