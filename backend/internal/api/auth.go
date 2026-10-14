package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lmagsino/run-coach-ai/backend/internal/auth"
	"github.com/lmagsino/run-coach-ai/backend/internal/garmin"
	"github.com/lmagsino/run-coach-ai/backend/internal/store"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a \"password\""})
		return
	}
	if !s.logins.allow(clientKey(r), time.Now()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts; wait a minute"})
		return
	}
	id, err := s.auth.Authenticate(r.Context(), auth.Credentials{Password: body.Password})
	if errors.Is(err, auth.ErrDenied) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong password"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sid, err := s.store.CreateSession(r.Context(), id, sessionTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	setSessionCookie(w, r, sid)
	_ = s.store.Audit(r.Context(), id, "sign_in", nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(r.Context(), c.Value)
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// garminState is the chip and the fix sentence, derived from the stored row
// and never from a live Garmin call (that would make every page load a
// Garmin request).
type garminState struct {
	Status       string     `json:"status"` // none | connected | expired | unavailable
	Fix          string     `json:"fix,omitempty"`
	ImportedAt   *time.Time `json:"imported_at,omitempty"`
	LastOKAt     *time.Time `json:"last_ok_at,omitempty"`
	ExpiresEstAt *time.Time `json:"expires_est_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}

const (
	fixNone        = "Garmin is not connected. Run garmin-mcp-auth on your laptop and paste the token bundle on the Connection page."
	fixExpired     = "Your Garmin tokens have expired. Run garmin-mcp-auth on your laptop and paste the new bundle on the Connection page."
	fixUnavailable = "Garmin did not answer. Try again in a few minutes; if it keeps failing, upstream may need an update."
)

func (s *Server) garminStateFor(r *http.Request, id int64) (garminState, error) {
	if s.cfg.MockMode {
		return garminState{Status: store.GarminConnected}, nil
	}
	conn, err := s.store.GarminConnection(r.Context(), id)
	if err != nil {
		return garminState{}, err
	}
	if conn == nil {
		return garminState{Status: "none", Fix: fixNone}, nil
	}
	st := garminState{
		Status: conn.Status, ImportedAt: &conn.ImportedAt, LastOKAt: conn.LastOKAt,
		ExpiresEstAt: conn.ExpiresEstAt, LastError: conn.LastError,
	}
	switch conn.Status {
	case store.GarminExpired:
		st.Fix = fixExpired
	case store.GarminUnavailable:
		st.Fix = fixUnavailable
	}
	return st, nil
}

// handleMe tells the UI who is signed in, how Garmin is, and today's usage.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	id := runnerID(r)
	owner, err := s.store.Owner(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st, err := s.garminStateFor(r, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	usage, err := s.store.Usage(r.Context(), id, time.Now())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runner": map[string]any{"id": id, "display_name": owner.DisplayName},
		"garmin": st,
		"usage":  map[string]any{"today": usage.Questions, "cap": owner.DailyQuestionCap},
		"mock":   s.cfg.MockMode,
	})
}

// importGarminBundle stores the bundle from RUNCOACH_GARMIN_BUNDLE or the
// bundle file for the owner on start, so a laptop that ran auth.sh (or a host
// with the env var set) is connected without a paste. A bundle identical to
// the stored one is left alone; anything else is probed and replaces it.
func (s *Server) importGarminBundle(ctx context.Context) error {
	raw := s.cfg.GarminBundle
	source := "RUNCOACH_GARMIN_BUNDLE"
	if raw == "" && s.cfg.GarminBundleFile != "" {
		b, err := os.ReadFile(s.cfg.GarminBundleFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		raw, source = string(b), s.cfg.GarminBundleFile
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if s.sealer == nil {
		return errors.New("a bundle is configured but RUNCOACH_TOKEN_KEY is not set")
	}
	bundle, err := garmin.NormalizeBundle(raw)
	if err != nil {
		return err
	}
	owner, err := s.store.Owner(ctx)
	if err != nil {
		return err
	}
	if conn, err := s.store.GarminConnection(ctx, owner.ID); err == nil && conn != nil && conn.Status == store.GarminConnected {
		if stored, err := s.sealer.Open(conn.Blob); err == nil && string(stored) == bundle {
			log.Printf("garmin bundle from %s already stored", source)
			return nil
		}
	}
	name, err := s.garmin.Probe(ctx, bundle)
	if err != nil {
		return fmt.Errorf("bundle from %s rejected: %w", source, err)
	}
	if err := s.storeBundle(ctx, owner.ID, bundle, name); err != nil {
		return err
	}
	log.Printf("garmin bundle from %s imported (connected as %q)", source, name)
	return nil
}

// storeBundle seals and upserts a probed bundle for a runner.
func (s *Server) storeBundle(ctx context.Context, runnerID int64, bundle, name string) error {
	blob, err := s.sealer.Seal([]byte(bundle))
	if err != nil {
		return err
	}
	now := time.Now()
	exp := now.Add(garmin.TokenLifetime)
	if err := s.store.UpsertGarminConnection(ctx, store.GarminConnection{
		RunnerID: runnerID, Kind: store.ConnectionLocalTokens, Blob: blob, KeyVersion: 1,
		Status: store.GarminConnected, ImportedAt: now, LastOKAt: &now, ExpiresEstAt: &exp,
	}); err != nil {
		return err
	}
	return s.store.Audit(ctx, runnerID, "tokens_imported", map[string]any{"garmin_name": name})
}

// handleGarminTokens takes the pasted bundle, probes it, and stores it sealed.
func (s *Server) handleGarminTokens(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Bundle string `json:"bundle"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "body must be JSON with a \"bundle\""})
		return
	}
	if s.cfg.MockMode {
		writeJSON(w, http.StatusOK, map[string]any{"status": store.GarminConnected, "mock": true})
		return
	}
	if s.sealer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "RUNCOACH_TOKEN_KEY is not set on the server"})
		return
	}
	bundle, err := garmin.NormalizeBundle(body.Bundle)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	name, err := s.garmin.Probe(r.Context(), bundle)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "the bundle did not work: " + err.Error()})
		return
	}
	id := runnerID(r)
	if err := s.storeBundle(r.Context(), id, bundle, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st, _ := s.garminStateFor(r, id)
	writeJSON(w, http.StatusOK, map[string]any{"garmin": st, "garmin_name": name})
}

func (s *Server) handleGarminTokensDelete(w http.ResponseWriter, r *http.Request) {
	id := runnerID(r)
	if err := s.store.DeleteGarminConnection(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.store.Audit(r.Context(), id, "tokens_removed", nil)
	st, _ := s.garminStateFor(r, id)
	writeJSON(w, http.StatusOK, map[string]any{"garmin": st})
}
