package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/agent"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

// maxTitleRunes bounds the title taken from the first question.
const maxTitleRunes = 60

type chatJSON struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toChatJSON(c store.Chat) chatJSON {
	return chatJSON{ID: c.ID, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

type messageJSON struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
	Chart     json.RawMessage `json:"chart,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

func storeErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such chat"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func (s *Server) handleChatsList(w http.ResponseWriter, r *http.Request) {
	chats, err := s.store.ListChats(r.Context(), runnerID(r))
	if err != nil {
		storeErr(w, err)
		return
	}
	out := make([]chatJSON, 0, len(chats))
	for _, c := range chats {
		out = append(out, toChatJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out})
}

func (s *Server) handleChatsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	c, err := s.store.CreateChat(r.Context(), runnerID(r), strings.TrimSpace(body.Title))
	if err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toChatJSON(c))
}

func (s *Server) handleChatRename(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a non-empty \"title\""})
		return
	}
	if err := s.store.RenameChat(r.Context(), runnerID(r), r.PathValue("id"), strings.TrimSpace(body.Title)); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteChat(r.Context(), runnerID(r), r.PathValue("id")); err != nil {
		storeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChatMessages(w http.ResponseWriter, r *http.Request) {
	msgs, err := s.store.ListMessages(r.Context(), runnerID(r), r.PathValue("id"))
	if err != nil {
		storeErr(w, err)
		return
	}
	out := make([]messageJSON, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageJSON{ID: m.ID, Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls, Chart: m.Chart, CreatedAt: m.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": out})
}

// priorTurns rebuilds the agent's history from the stored messages: completed
// user/assistant pairs only, final text only. The client sends nothing.
func priorTurns(msgs []store.Message) []agent.PriorTurn {
	var out []agent.PriorTurn
	for i := 0; i+1 < len(msgs); i++ {
		if msgs[i].Role == "user" && msgs[i+1].Role == "assistant" && msgs[i+1].Content != "" {
			out = append(out, agent.PriorTurn{Question: msgs[i].Content, Answer: msgs[i+1].Content})
			i++
		}
	}
	return out
}

func titleFrom(question string) string {
	t := strings.Join(strings.Fields(question), " ")
	if r := []rune(t); len(r) > maxTitleRunes {
		t = strings.TrimSpace(string(r[:maxTitleRunes-1])) + "…"
	}
	return t
}

// handleChatStream answers a question in a saved chat over SSE: step events as
// tools run, at most one chart, then the whole answer (ADR 0009). Everything
// that can fail with a status code is checked before the stream opens.
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	req, ok := readRequest(w, r)
	if !ok {
		return
	}
	id := runnerID(r)
	chatID := r.PathValue("id")
	chat, err := s.store.GetChat(r.Context(), id, chatID)
	if err != nil {
		storeErr(w, err)
		return
	}
	if status, err := s.checkCap(r.Context(), id); err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	// Checked before the stream opens, so a missing key is a status code and
	// never a screener call.
	if !s.cfg.MockMode && s.cfg.AnthropicAPIKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ANTHROPIC_API_KEY is not set"})
		return
	}
	if _, busy := s.inflight.LoadOrStore(id, true); busy {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "a reply is already in progress"})
		return
	}
	defer s.inflight.Delete(id)

	sse, ok := newSSEWriter(w)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported by this server"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), chatTimeout)
	defer cancel()
	go sse.keepalive(ctx, keepaliveEvery)

	// The question is saved before the reply is attempted, so a failed reply
	// still shows what was asked; the cap counts it too.
	prior, _ := s.store.ListMessages(ctx, id, chatID)
	if _, err := s.store.AppendMessage(ctx, id, store.Message{ChatID: chatID, Role: "user", Content: req.Question}); err != nil {
		sse.send("error", map[string]string{"error": err.Error()})
		return
	}
	if chat.Title == "" {
		_ = s.store.RenameChat(ctx, id, chatID, titleFrom(req.Question))
	}

	var resp chatResponse
	var result *agent.Result
	if s.cfg.MockMode {
		var delivered bool
		if resp, delivered = streamMock(ctx, sse, req.Question); !delivered {
			return
		}
		result = &agent.Result{Answer: resp.Answer}
	} else {
		// Screen and scope first: an off-topic question is declined here, with
		// no Garmin spawn and no coach call behind it.
		sse.send("step", stepEvent{Source: agent.CoachSource, Tool: "triage", State: "active"})
		triage := s.triage(ctx, priorTurns(prior), req.Question)
		sse.send("step", stepEvent{Source: agent.CoachSource, Tool: "triage", State: "done"})
		if !triage.OnTopic {
			resp = chatResponse{Answer: triage.Decline}
			sse.send("answer", resp)
			result = &agent.Result{Answer: resp.Answer}
			s.finishReply(ctx, id, chatID, resp, result)
			return
		}

		ag, cleanup, _, err := s.prepareAgent(ctx, id)
		if err != nil {
			// The stream has already begun, so failures are in-band from here on.
			sse.send("error", map[string]string{"error": err.Error()})
			return
		}
		defer cleanup()
		ag.Observe(func(ev agent.Event) {
			if ev.Kind == agent.EventChart {
				sse.send("chart", ev.Chart)
				return
			}
			state := "active"
			if ev.Kind == agent.EventToolEnd {
				state = "done"
				if ev.Failed {
					state = "failed"
				}
			}
			sse.send("step", stepEvent{Source: ev.Source, Tool: ev.Tool, State: state})
		})
		result, err = ag.AnswerScoped(ctx, priorTurns(prior), req.Question, triage.Scope)
		s.recordOutcome(ctx, id, result, err)
		if err != nil {
			sse.send("error", map[string]string{"error": err.Error()})
			return
		}
		resp = chatResponse{Answer: result.Answer, Sources: result.Sources(), Chart: result.Chart}
		sse.send("answer", resp)
	}
	s.finishReply(ctx, id, chatID, resp, result)
}

// finishReply saves the assistant turn and counts the usage.
func (s *Server) finishReply(ctx context.Context, runnerID int64, chatID string, resp chatResponse, result *agent.Result) {
	toolCalls, _ := json.Marshal(result.ToolCalls)
	var chartJSON []byte
	if resp.Chart != nil {
		chartJSON, _ = json.Marshal(resp.Chart)
	}
	_, _ = s.store.AppendMessage(ctx, runnerID, store.Message{
		ChatID: chatID, Role: "assistant", Content: resp.Answer, ToolCalls: toolCalls, Chart: chartJSON,
	})
	_ = s.store.AddUsage(ctx, runnerID, time.Now(), usageOf(result))
	log.Printf("reply: %d tool calls, chart=%t, tokens in=%d cached=%d out=%d",
		len(result.ToolCalls), resp.Chart != nil, result.Usage.InputTokens, result.Usage.CachedInputTokens, result.Usage.OutputTokens)
}
