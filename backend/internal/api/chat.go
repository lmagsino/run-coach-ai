package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/agent"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/mockchat"
	"github.com/lmagsino/run-coach-ai/backend/internal/strava"
)

type chatRequest struct {
	Question string `json:"question"`
}

type chatResponse struct {
	Answer string `json:"answer"`
	// Sources actually queried while answering, in call order. Empty means
	// Claude answered without reaching for any tool.
	Sources []string `json:"sources"`
}

// stepEvent narrates one tool call on the SSE stream. There is deliberately no
// label field: turning "get_sleep_data" into "Checking your Garmin sleep & HRV"
// is presentation, and DESIGN.md owns that copy, so the frontend maps it.
type stepEvent struct {
	Source string `json:"source"`
	Tool   string `json:"tool"`
	// "active" once dispatched, then "done" or "failed".
	State string `json:"state"`
}

// chatTimeout is generous because the tool-calling loop makes several model and
// MCP round-trips before it can answer.
const chatTimeout = 90 * time.Second

// readQuestion decodes and validates the request body, writing the error
// response itself and returning ok=false if it is unusable.
func readQuestion(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "body must be JSON with a non-empty \"question\"",
		})
		return "", false
	}
	return req.Question, true
}

// prepareAgent connects every configured MCP source and returns a ready Agent
// plus a cleanup func. On failure it returns an HTTP status and message for the
// caller to render — as JSON for /chat, as an SSE error event for /chat/stream.
func (s *Server) prepareAgent(ctx context.Context) (*agent.Agent, func(), int, error) {
	if s.cfg.AnthropicAPIKey == "" {
		return nil, nil, http.StatusServiceUnavailable, errors.New("ANTHROPIC_API_KEY is not set")
	}

	tok, err := s.strava.ValidToken(ctx)
	if errors.Is(err, strava.ErrNoToken) {
		return nil, nil, http.StatusPreconditionRequired,
			errors.New("no Strava connection yet — authorize first at /auth/strava/login")
	}
	if err != nil {
		return nil, nil, http.StatusBadGateway, err
	}

	stravaSession, err := s.stravaMCP.Connect(ctx, tok.AccessToken)
	if err != nil {
		return nil, nil, http.StatusBadGateway, err
	}
	closers := []func(){func() { stravaSession.Close() }}
	cleanup := func() {
		for _, c := range closers {
			c()
		}
	}
	sources := []agent.Source{{Name: mcpclient.SourceStrava, Session: stravaSession}}

	// Garmin joins the tool set only when configured. A configured-but-broken
	// container is a real error, not something to quietly answer around:
	// dropping it would leave the agent silently unable to see recovery data.
	if s.garminMCP != nil {
		garminSession, err := s.garminMCP.Connect(ctx)
		if err != nil {
			cleanup()
			return nil, nil, http.StatusBadGateway, err
		}
		closers = append(closers, func() { garminSession.Close() })
		sources = append(sources, agent.Source{Name: mcpclient.SourceGarmin, Session: garminSession})
	}

	return agent.New(s.anthropic, anthropic.Model(s.cfg.AnthropicModel), sources...), cleanup, 0, nil
}

// handleChat answers a training question in a single round-trip. Kept alongside
// the streaming endpoint because it is the curl-friendly shape the CLI checks and
// CLAUDE.md document; the UI uses /chat/stream so it can narrate progress.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	question, ok := readQuestion(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), chatTimeout)
	defer cancel()

	if s.cfg.MockMode {
		scenario := mockchat.Match(question)
		writeJSON(w, http.StatusOK, chatResponse{
			Answer: scenario.Answer, Sources: scenario.Sources(),
		})
		return
	}

	ag, cleanup, status, err := s.prepareAgent(ctx)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	defer cleanup()

	result, err := ag.Answer(ctx, question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, chatResponse{Answer: result.Answer, Sources: result.Sources()})
}

// handleChatStream answers the same question over SSE, emitting a step event as
// each tool call starts and finishes so the UI can show which sources are being
// consulted while it happens (spec §5, DESIGN.md §5).
//
// It is a POST rather than the GET that EventSource wants, so the question stays
// in the body: putting user text in a query string would leak it into access logs
// and cap its length. The frontend reads the stream with fetch instead.
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	question, ok := readQuestion(w, r)
	if !ok {
		return
	}

	// Checked before any SSE header is written — once the stream opens there is
	// no status code left to fail with.
	sse, ok := newSSEWriter(w)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "streaming unsupported by this server",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), chatTimeout)
	defer cancel()

	if s.cfg.MockMode {
		streamMock(ctx, sse, question)
		return
	}

	ag, cleanup, _, err := s.prepareAgent(ctx)
	if err != nil {
		// The stream has already begun, so failures are in-band from here on.
		sse.send("error", map[string]string{"error": err.Error()})
		return
	}
	defer cleanup()

	// Observers run synchronously on Answer's goroutine, which is this handler's
	// goroutine, so writing to the stream from here needs no extra locking.
	ag.Observe(func(ev agent.Event) {
		state := "active"
		if ev.Kind == agent.EventToolEnd {
			state = "done"
			if ev.Failed {
				state = "failed"
			}
		}
		sse.send("step", stepEvent{Source: ev.Source, Tool: ev.Tool, State: state})
	})

	result, err := ag.Answer(ctx, question)
	if err != nil {
		sse.send("error", map[string]string{"error": err.Error()})
		return
	}
	sse.send("answer", chatResponse{Answer: result.Answer, Sources: result.Sources()})
}

// streamMock replays a canned scenario with the event sequence and rough timing
// a real answer produces, so the UI is exercised the way Phase 5 will drive it.
func streamMock(ctx context.Context, sse *sseWriter, question string) {
	scenario := mockchat.Match(question)

	for _, step := range scenario.Steps {
		sse.send("step", stepEvent{Source: step.Source, Tool: step.Tool, State: "active"})
		select {
		case <-time.After(mockchat.StepDelay):
		case <-ctx.Done():
			// Client navigated away, or the timeout fired. Stop pretending to work.
			return
		}
		state := "done"
		if step.Failed {
			state = "failed"
		}
		sse.send("step", stepEvent{Source: step.Source, Tool: step.Tool, State: state})
	}

	sse.send("answer", chatResponse{Answer: scenario.Answer, Sources: scenario.Sources()})
}
