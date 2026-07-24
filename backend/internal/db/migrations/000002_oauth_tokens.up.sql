-- OAuth tokens, one row per provider. Single-user for v1 (Strava now), so the
-- provider name is the primary key. Garmin uses a different auth model and is
-- handled separately in Phase 3.
CREATE TABLE oauth_tokens (
    provider      TEXT PRIMARY KEY,               -- 'strava'
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type    TEXT NOT NULL DEFAULT 'Bearer',
    expires_at    TIMESTAMPTZ,                    -- access token expiry
    scope         TEXT,
    athlete_id    TEXT,
    raw           JSONB,                           -- full token response
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
