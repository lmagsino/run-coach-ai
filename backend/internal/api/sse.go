package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// keepaliveEvery is how often an idle stream gets a comment line. A reply can
// spend 15 to 40 seconds in the tool loop with nothing to say, and a hosting
// proxy that sees no bytes for a minute may close the connection.
const keepaliveEvery = 15 * time.Second

// sseWriter emits Server-Sent Events on a response. Each event is flushed
// immediately: the whole point is that the client sees a tool call start before
// the answer exists, so buffering would defeat it.
type sseWriter struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
}

// newSSEWriter sets the SSE headers and returns a writer, or false if the
// ResponseWriter cannot flush (no streaming possible, so the caller should fall
// back to a plain JSON response rather than emit events nobody will see until
// the end).
func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	// Proxies that buffer would hold the steps back until the answer lands.
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return &sseWriter{w: w, flusher: flusher}, true
}

// send writes one named event carrying data as JSON.
func (s *sseWriter) send(event string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		// Encoding our own event types can't realistically fail, and there is no
		// status code left to set mid-stream, so report it in-band as an error
		// event instead of dropping the frame silently.
		payload = []byte(`{"error":"failed to encode event"}`)
		event = "error"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, payload)
	s.flusher.Flush()
}

// keepalive writes a comment line every keepaliveEvery until ctx ends. Comment
// lines are ignored by every SSE parser, so the frontend sees nothing.
func (s *sseWriter) keepalive(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			fmt.Fprint(s.w, ": keepalive\n\n")
			s.flusher.Flush()
			s.mu.Unlock()
		}
	}
}
