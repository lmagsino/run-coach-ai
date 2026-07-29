package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseWriter emits Server-Sent Events on a response. Each event is flushed
// immediately: the whole point is that the client sees a tool call start before
// the answer exists, so buffering would defeat it.
type sseWriter struct {
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
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, payload)
	s.flusher.Flush()
}
