// Package agent runs a Claude tool-calling loop whose tools are backed by one or
// more MCP sessions — Strava, Garmin, or both. Claude decides which tools to
// call; the agent routes each call to the session that owns it and feeds results
// back until Claude answers.
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
	`questions about their training using ONLY the data returned by the tools. ` +
	`Ground every claim in what the tools return — cite specific numbers (distances, ` +
	`paces, dates). If the tools return no data, say so plainly; never invent activities ` +
	`or fall back on generic training advice.`

const maxTurns = 8

// toolNameSep separates a source name from the tool name in the namespaced tool
// names given to Claude, e.g. "garmin__get_sleep_data". Both servers expose an
// activities listing, so unprefixed names would collide; the separator also
// tells the model which source it is reaching for.
const toolNameSep = "__"

// Source is one MCP-backed tool source available to the agent. Name must be a
// short identifier safe for a tool name (see mcpclient.SourceStrava/SourceGarmin).
type Source struct {
	Name    string
	Session *mcp.ClientSession
}

// ToolCall records one tool invocation the agent made on Claude's behalf.
// Callers use it to show which sources a given answer actually drew on.
type ToolCall struct {
	Source string
	Tool   string
	Failed bool
}

// Result is a finished answer plus the trail of tool calls that produced it.
type Result struct {
	Answer    string
	ToolCalls []ToolCall
}

// Sources returns the distinct source names the answer drew on, in call order.
func (r *Result) Sources() []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range r.ToolCalls {
		if !seen[c.Source] {
			seen[c.Source] = true
			out = append(out, c.Source)
		}
	}
	return out
}

// Agent bridges the Anthropic Messages API to one or more MCP tool sessions.
type Agent struct {
	anthropic anthropic.Client
	model     anthropic.Model
	sources   []Source
}

// New builds an Agent over already-connected MCP sessions. Order is preserved
// in the tool list handed to Claude.
func New(client anthropic.Client, model anthropic.Model, sources ...Source) *Agent {
	return &Agent{anthropic: client, model: model, sources: sources}
}

// toolRef points a namespaced tool name back at the session that owns it.
type toolRef struct {
	source  string
	tool    string
	session *mcp.ClientSession
}

// Answer runs the tool-calling loop and returns a grounded natural-language answer.
func (a *Agent) Answer(ctx context.Context, question string) (*Result, error) {
	tools, router, err := a.buildTools(ctx)
	if err != nil {
		return nil, err
	}

	// Give Claude the current date so it can resolve relative ranges like
	// "last week" into the RFC3339 after/before the tools expect.
	system := systemPrompt + fmt.Sprintf("\n\nThe current date and time is %s. "+
		"Use it to resolve relative dates (e.g. \"last week\", \"this month\").",
		time.Now().UTC().Format(time.RFC3339))

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
	}
	result := &Result{}

	for turn := 0; turn < maxTurns; turn++ {
		resp, err := a.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     a.model,
			MaxTokens: 1024,
			System:    []anthropic.TextBlockParam{{Text: system}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			return nil, fmt.Errorf("claude message: %w", err)
		}
		messages = append(messages, resp.ToParam())

		if resp.StopReason != anthropic.StopReasonToolUse {
			result.Answer = textOf(resp)
			return result, nil
		}

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			tu, ok := block.AsAny().(anthropic.ToolUseBlock)
			if !ok {
				continue
			}
			ref, known := router[tu.Name]
			if !known {
				results = append(results, anthropic.NewToolResultBlock(tu.ID,
					fmt.Sprintf("no such tool %q", tu.Name), true))
				continue
			}
			out, callErr := callTool(ctx, ref, tu.JSON.Input.Raw())
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				Source: ref.source, Tool: ref.tool, Failed: callErr != nil,
			})
			results = append(results, anthropic.NewToolResultBlock(tu.ID, out, callErr != nil))
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
	return nil, fmt.Errorf("gave up after %d tool-use turns without a final answer", maxTurns)
}

// buildTools converts every source's MCP tools into Anthropic tool definitions
// with source-namespaced names, plus the routing table to undo the namespacing.
func (a *Agent) buildTools(ctx context.Context) ([]anthropic.ToolUnionParam, map[string]toolRef, error) {
	var tools []anthropic.ToolUnionParam
	router := map[string]toolRef{}

	for _, src := range a.sources {
		list, err := src.Session.ListTools(ctx, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("list %s mcp tools: %w", src.Name, err)
		}
		for _, t := range list.Tools {
			name := src.Name + toolNameSep + t.Name
			router[name] = toolRef{source: src.Name, tool: t.Name, session: src.Session}
			tools = append(tools, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
				Name:        name,
				Description: anthropic.String(fmt.Sprintf("[%s] %s", src.Name, t.Description)),
				InputSchema: inputSchema(t),
			}})
		}
	}
	return tools, router, nil
}

// inputSchema translates an MCP tool's JSON Schema into the Anthropic shape.
func inputSchema(t *mcp.Tool) anthropic.ToolInputSchemaParam {
	schema := anthropic.ToolInputSchemaParam{}
	m, ok := t.InputSchema.(map[string]any)
	if !ok {
		return schema
	}
	schema.Properties = m["properties"]
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	return schema
}

// callTool invokes an MCP tool on its owning session and returns its textual result.
func callTool(ctx context.Context, ref toolRef, rawArgs string) (string, error) {
	var args map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return fmt.Sprintf("invalid tool arguments: %v", err), err
		}
	}
	res, err := ref.session.CallTool(ctx, &mcp.CallToolParams{Name: ref.tool, Arguments: args})
	if err != nil {
		return fmt.Sprintf("tool error: %v", err), err
	}
	text := contentText(res)
	if res.IsError {
		return text, fmt.Errorf("%s tool %q reported an error", ref.source, ref.tool)
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
