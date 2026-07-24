// Package agent runs a Claude tool-calling loop whose tools are backed by the
// Strava MCP session. Claude decides which tools to call; the agent executes
// them against the MCP server and feeds results back until Claude answers.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const systemPrompt = `You are RunCoach, a running coach assistant. Answer the user's ` +
	`questions about their training using ONLY the data returned by the Strava tools. ` +
	`Ground every claim in what the tools return — cite specific numbers (distances, ` +
	`paces, dates). If the tools return no data, say so plainly; never invent activities ` +
	`or fall back on generic training advice.`

const maxTurns = 8

// Agent bridges the Anthropic Messages API to an MCP tool session.
type Agent struct {
	anthropic anthropic.Client
	model     anthropic.Model
	session   *mcp.ClientSession
}

// New builds an Agent over an already-connected MCP session.
func New(client anthropic.Client, model anthropic.Model, session *mcp.ClientSession) *Agent {
	return &Agent{anthropic: client, model: model, session: session}
}

// Answer runs the tool-calling loop and returns a grounded natural-language answer.
func (a *Agent) Answer(ctx context.Context, question string) (string, error) {
	tools, err := a.buildTools(ctx)
	if err != nil {
		return "", err
	}

	// Give Claude the current date so it can resolve relative ranges like
	// "last week" into the RFC3339 after/before the Strava tool expects.
	system := systemPrompt + fmt.Sprintf("\n\nThe current date and time is %s. "+
		"Use it to resolve relative dates (e.g. \"last week\", \"this month\").",
		time.Now().UTC().Format(time.RFC3339))

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}

	for turn := 0; turn < maxTurns; turn++ {
		resp, err := a.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     a.model,
			MaxTokens: 1024,
			System:    []anthropic.TextBlockParam{{Text: system}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return "", fmt.Errorf("claude message: %w", err)
		}
		messages = append(messages, resp.ToParam())

		if resp.StopReason != anthropic.StopReasonToolUse {
			return textOf(resp), nil
		}

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			tu, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			out, callErr := a.callTool(ctx, tu.Name, tu.JSON.Input.Raw())
			results = append(results, anthropic.NewToolResultBlock(tu.ID, out, callErr != nil))
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
	return "", fmt.Errorf("gave up after %d tool-use turns without a final answer", maxTurns)
}

// buildTools converts the MCP server's tools into Anthropic tool definitions.
func (a *Agent) buildTools(ctx context.Context) ([]anthropic.ToolUnionParam, error) {
	list, err := a.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list mcp tools: %w", err)
	}

	tools := make([]anthropic.ToolUnionParam, 0, len(list.Tools))
	for _, t := range list.Tools {
		schema := anthropic.ToolInputSchemaParam{}
		if m, ok := t.InputSchema.(map[string]any); ok {
			schema.Properties = m["properties"]
			if req, ok := m["required"].([]any); ok {
				for _, r := range req {
					if s, ok := r.(string); ok {
						schema.Required = append(schema.Required, s)
					}
				}
			}
		}
		tools = append(tools, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: schema,
		}})
	}
	return tools, nil
}

// callTool invokes an MCP tool and returns its textual result.
func (a *Agent) callTool(ctx context.Context, name, rawArgs string) (string, error) {
	var args map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return fmt.Sprintf("invalid tool arguments: %v", err), err
		}
	}
	res, err := a.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Sprintf("tool error: %v", err), err
	}
	text := contentText(res)
	if res.IsError {
		return text, fmt.Errorf("tool %q reported an error", name)
	}
	return text, nil
}

func contentText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func textOf(resp *anthropic.Message) string {
	var b strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return b.String()
}
