// Command strava-mcp is a self-hosted MCP server exposing Strava data as tools,
// backed by the Strava REST API. It reads the athlete's access token from the
// STRAVA_ACCESS_TOKEN environment variable and speaks MCP over stdio, so the
// backend can launch it as a subprocess and connect as an MCP client.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listActivitiesInput is the typed input schema for the list_activities tool.
type listActivitiesInput struct {
	After   string `json:"after,omitempty" jsonschema:"only activities on or after this RFC3339 datetime (e.g. 2026-07-01T00:00:00Z)"`
	Before  string `json:"before,omitempty" jsonschema:"only activities before this RFC3339 datetime"`
	Page    int    `json:"page,omitempty" jsonschema:"page number, 1-based"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"activities per page (default 30, max 200)"`
}

type listActivitiesOutput struct {
	Activities []strava.Activity `json:"activities"`
}

func main() {
	token := os.Getenv("STRAVA_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("STRAVA_ACCESS_TOKEN is required")
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "strava-mcp", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_activities",
		Description: "List the authenticated athlete's Strava activities, most recent first. Supports date-range filtering (after/before) and pagination.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listActivitiesInput) (*mcp.CallToolResult, listActivitiesOutput, error) {
		p := strava.ListActivitiesParams{Page: in.Page, PerPage: in.PerPage}
		if in.After != "" {
			t, err := time.Parse(time.RFC3339, in.After)
			if err != nil {
				return nil, listActivitiesOutput{}, fmt.Errorf("invalid 'after' (want RFC3339): %w", err)
			}
			p.After = &t
		}
		if in.Before != "" {
			t, err := time.Parse(time.RFC3339, in.Before)
			if err != nil {
				return nil, listActivitiesOutput{}, fmt.Errorf("invalid 'before' (want RFC3339): %w", err)
			}
			p.Before = &t
		}
		acts, err := strava.ListActivities(ctx, token, p)
		if err != nil {
			return nil, listActivitiesOutput{}, err
		}
		return nil, listActivitiesOutput{Activities: acts}, nil
	})

	// log goes to stderr, keeping stdout clean for the JSON-RPC stream.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("strava-mcp server: %v", err)
	}
}
