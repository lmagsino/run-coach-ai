package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/lmagsino/run-coach-ai/backend/internal/agent"
	"github.com/lmagsino/run-coach-ai/backend/internal/chart"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/mcpclient"
	"github.com/lmagsino/run-coach-ai/backend/internal/mockchat"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

type chatRequest struct {
	Question string `json:"question"`
}

type chatResponse struct {
	Answer string `json:"answer"`
	// Sources actually queried while answering, in call order. Empty means
	// Claude answered without reaching for any tool.
	Sources []string `json:"sources"`
	// Chart is the one validated chart the reply carries, or null (ADR 0005).
	Chart *chart.Spec `json:"chart"`
}

// stepEvent narrates one tool call on the SSE stream. There is deliberately no
// label field: turning "get_sleep_data" into "Checking your Garmin sleep" is
// presentation, and DESIGN.md owns that copy, so the frontend maps it.
type stepEvent struct {
	Source string `json:"source"`
	Tool   string `json:"tool"`
	// "active" once dispatched, then "done" or "failed".
	State string `json:"state"`
}

// chatTimeout is generous because the tool-calling loop makes several model and
// MCP round-trips before it can answer.
const chatTimeout = 120 * time.Second

// readRequest decodes and validates the request body, writing the error response
// itself and returning ok=false if it is unusable.
func readRequest(w http.ResponseWriter, r *http.Request) (chatRequest, bool) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "body must be JSON with a non-empty \"question\"",
		})
		return chatRequest{}, false
	}
	return req, true
}

// prepareAgent opens the runner's Garmin source and returns a ready Agent plus
// a cleanup func. On failure it returns an HTTP status and a message for the
// caller to render — as JSON for /api/chat, as an SSE error event mid-stream.
func (s *Server) prepareAgent(ctx context.Context, runnerID int64) (*agent.Agent, func(), int, error) {
	if s.cfg.AnthropicAPIKey == "" {
		return nil, nil, http.StatusServiceUnavailable, errors.New("ANTHROPIC_API_KEY is not set")
	}
	session, cleanup, err := s.garmin.Connect(ctx, runnerID)
	if errors.Is(err, garmin.ErrNotConnected) {
		return nil, nil, http.StatusPreconditionRequired, errors.New(fixNone)
	}
	if err != nil {
		_ = s.store.SetGarminStatus(ctx, runnerID, store.GarminUnavailable, err.Error(), false)
		return nil, nil, http.StatusBadGateway, err
	}
	source := agent.Source{Name: mcpclient.SourceGarmin, Session: session, Allow: mcpclient.ReadOnly}
	ag := agent.New(s.anthropic, anthropic.Model(s.cfg.AnthropicModel), source).WithEffort(s.cfg.AnthropicEffort)
	return ag, cleanup, 0, nil
}

// recordOutcome updates the Garmin row from how a reply went: a clean reply
// marks it connected, an auth failure marks it expired, anything else
// unavailable. The chip reads the row, never Garmin.
func (s *Server) recordOutcome(ctx context.Context, runnerID int64, result *agent.Result, err error) {
	if s.cfg.MockMode {
		return
	}
	switch {
	case err == nil && result != nil:
		failedAll := len(result.ToolCalls) > 0
		for _, c := range result.ToolCalls {
			if !c.Failed {
				failedAll = false
			}
		}
		if failedAll {
			_ = s.store.SetGarminStatus(ctx, runnerID, store.GarminUnavailable, "every Garmin tool call failed", false)
			return
		}
		_ = s.store.SetGarminStatus(ctx, runnerID, store.GarminConnected, "", true)
	case err != nil:
		classified := garmin.Classify(err)
		status := store.GarminUnavailable
		if errors.Is(classified, garmin.ErrAuth) {
			status = store.GarminExpired
		}
		_ = s.store.SetGarminStatus(ctx, runnerID, status, err.Error(), false)
	}
}

// triage screens and scopes a question with the small model. Its usage is
// folded into the reply's; its failure is logged and the question goes on.
func (s *Server) triage(ctx context.Context, prior []agent.PriorTurn, question string) agent.Triage {
	t, err := agent.RunTriage(ctx, s.anthropic, anthropic.Model(s.cfg.AnthropicTriageModel), prior, question, time.Now())
	if err != nil {
		log.Printf("triage: %v (continuing unscoped)", err)
	}
	return t
}

// handleChat answers one question with no chat and no history: the
// curl-friendly shape for the live pass. It still counts against the cap.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	id := runnerID(r)
	if status, err := s.checkCap(r.Context(), id); err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), chatTimeout)
	defer cancel()

	if s.cfg.MockMode {
		scenario := mockchat.Match(req.Question)
		_ = s.store.AddUsage(ctx, id, time.Now(), store.Usage{Questions: 1})
		writeJSON(w, http.StatusOK, chatResponse{
			Answer: scenario.Answer, Sources: scenario.Sources(), Chart: scenario.ValidChart(),
		})
		return
	}

	triage := s.triage(ctx, nil, req.Question)
	if !triage.OnTopic {
		_ = s.store.AddUsage(ctx, id, time.Now(), store.Usage{Questions: 1})
		writeJSON(w, http.StatusOK, chatResponse{Answer: triage.Decline})
		return
	}

	ag, cleanup, status, err := s.prepareAgent(ctx, id)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	defer cleanup()

	result, err := ag.AnswerScoped(ctx, nil, req.Question, triage.Scope)
	s.recordOutcome(ctx, id, result, err)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.AddUsage(ctx, id, time.Now(), usageOf(result))
	writeJSON(w, http.StatusOK, chatResponse{Answer: result.Answer, Sources: result.Sources(), Chart: result.Chart})
}

// checkCap returns 429 once the runner has asked today's allowance. It is a
// cost guard, so it counts questions asked, not answers delivered.
func (s *Server) checkCap(ctx context.Context, runnerID int64) (int, error) {
	runner, err := s.store.RunnerByID(ctx, runnerID)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	usage, err := s.store.Usage(ctx, runnerID, time.Now())
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if usage.Questions >= runner.DailyQuestionCap {
		return http.StatusTooManyRequests, errors.New("daily cap reached; it resets at midnight UTC")
	}
	return 0, nil
}

func usageOf(result *agent.Result) store.Usage {
	return store.Usage{
		Questions:         1,
		InputTokens:       result.Usage.InputTokens,
		CachedInputTokens: result.Usage.CachedInputTokens,
		OutputTokens:      result.Usage.OutputTokens,
	}
}

// streamMock replays a canned scenario with the event sequence and rough timing
// a real answer produces, so the UI is exercised the way a live reply drives it.
func streamMock(ctx context.Context, sse *sseWriter, question string) (chatResponse, bool) {
	scenario := mockchat.Match(question)

	for _, step := range scenario.Steps {
		sse.send("step", stepEvent{Source: step.Source, Tool: step.Tool, State: "active"})
		select {
		case <-time.After(mockchat.StepDelay):
		case <-ctx.Done():
			// Client navigated away, or the timeout fired. Stop pretending to work.
			return chatResponse{}, false
		}
		state := "done"
		if step.Failed {
			state = "failed"
		}
		sse.send("step", stepEvent{Source: step.Source, Tool: step.Tool, State: state})
	}

	// The same gate the live path applies: an invalid chart emits no frame and
	// the prose still arrives, so the drop path is exercisable in mock mode.
	chartSpec := scenario.ValidChart()
	if chartSpec != nil {
		sse.send("chart", chartSpec)
	}
	resp := chatResponse{Answer: scenario.Answer, Sources: scenario.Sources(), Chart: chartSpec}
	sse.send("answer", resp)
	return resp, true
}
