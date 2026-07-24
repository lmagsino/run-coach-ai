// Command strava-check proves the Strava MCP path end-to-end: it loads a valid
// Strava access token, launches the self-hosted strava-mcp server, connects as
// an MCP client, and calls list_activities — printing what comes back.
//
// Usage: go run ./cmd/strava-check
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/config"
	"github.com/lmagsino/run-coach-ai/backend/internal/db"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	client := strava.NewClient(cfg.StravaClientID, cfg.StravaClientSecret, cfg.StravaRedirectURI, strava.NewTokenStore(pool))
	tok, err := client.ValidToken(ctx)
	if err != nil {
		log.Fatalf("no valid Strava token (connect Strava first via /auth/strava/login): %v", err)
	}

	fields := strings.Fields(cfg.StravaMCPCommand)
	session, err := mcpclient.NewStrava(fields[0], fields[1:]...).Connect(ctx, tok.AccessToken)
	if err != nil {
		log.Fatalf("connect strava-mcp: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("list tools: %v", err)
	}
	fmt.Printf("strava-mcp tools: ")
	for i, t := range tools.Tools {
		if i > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s", t.Name)
	}
	fmt.Println()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_activities",
		Arguments: map[string]any{"per_page": 5},
	})
	if err != nil {
		log.Fatalf("call list_activities: %v", err)
	}
	if res.IsError {
		log.Fatalf("list_activities returned an error result")
	}

	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Println(prettyJSON(tc.Text))
		}
	}
}

func prettyJSON(s string) string {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s
	}
	return string(out)
}
