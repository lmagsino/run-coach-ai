package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

const (
	sessionCookie = "runcoach_session"
	sessionTTL    = 30 * 24 * time.Hour
)

type ctxKey int

const runnerKey ctxKey = iota

// runnerID is the authenticated runner for this request. Handlers never look
// at the cookie themselves and never ask "is this the owner" (ADR 0002).
func runnerID(r *http.Request) int64 {
	id, _ := r.Context().Value(runnerKey).(int64)
	return id
}

// withSession resolves the session cookie to a runner id for every /api route
// except login and health. Mock mode skips the cookie and uses the owner, so
// the UI can be driven with nothing configured; the responses say mock:true.
func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/login" || r.URL.Path == "/api/signup" || r.URL.Path == "/api/healthz":
			next.ServeHTTP(w, r)
			return
		case s.cfg.MockMode:
			owner, err := s.store.Owner(r.Context())
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), runnerKey, owner.ID)))
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
			return
		}
		id, err := s.store.LookupSession(r.Context(), c.Value)
		if errors.Is(err, store.ErrNoSession) {
			clearSessionCookie(w, r)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired, sign in again"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), runnerKey, id)))
	})
}

// secure reports whether the cookie should carry Secure: on TLS, or behind a
// host that terminated TLS and says so. Local http dev gets a plain cookie.
func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, id string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: id, Path: "/", HttpOnly: true, Secure: secure(r),
		SameSite: http.SameSiteLaxMode, MaxAge: int(sessionTTL.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: secure(r),
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// loginLimiter blocks an address after a handful of failed attempts in a
// minute: wrong passwords and wrong invite codes. Successes are not counted,
// so a runner who signs in, out and in again is never locked out. In memory,
// so it resets on restart; enough to stop guessing on a public URL.
type loginLimiter struct {
	mu    sync.Mutex
	hits  map[string][]time.Time
	limit int
	win   time.Duration
}

func newLoginLimiter(limit int, win time.Duration) *loginLimiter {
	return &loginLimiter{hits: map[string][]time.Time{}, limit: limit, win: win}
}

// blocked reports whether key has used up its failures for the window.
func (l *loginLimiter) blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recentLocked(key, now)) >= l.limit
}

// fail records one failed attempt for key.
func (l *loginLimiter) fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.recentLocked(key, now), now)
}

func (l *loginLimiter) recentLocked(key string, now time.Time) []time.Time {
	var recent []time.Time
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.win {
			recent = append(recent, t)
		}
	}
	l.hits[key] = recent
	return recent
}

// clientKey is the address the limiter counts. Behind a proxy that sets
// X-Forwarded-For, the first hop is the client.
func clientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
