-- Chat sessions: one row per conversation.
CREATE TABLE chat_sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Chat messages: ordered turns within a session.
-- tool_calls holds the structured tool-use / tool-result payload (JSON) for
-- assistant/tool turns; NULL for plain user/assistant text.
CREATE TABLE chat_messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'tool', 'system')),
    content    TEXT NOT NULL DEFAULT '',
    tool_calls JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_messages_session ON chat_messages (session_id, created_at);

-- Activity cache: normalized activity rows from any source. Strava is wired up
-- in Phase 2; Garmin joins in Phase 3 under the same shape. The full source
-- payload is kept in `raw` so we can re-derive fields without re-fetching.
CREATE TABLE activity_cache (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source             TEXT NOT NULL CHECK (source IN ('strava', 'garmin')),
    source_activity_id TEXT NOT NULL,
    athlete_id         TEXT,
    type               TEXT,             -- e.g. Run, Ride, Swim
    name               TEXT,
    start_time         TIMESTAMPTZ,
    distance_m         DOUBLE PRECISION,
    moving_time_s      INTEGER,
    elapsed_time_s     INTEGER,
    total_elevation_m  DOUBLE PRECISION,
    average_speed_ms   DOUBLE PRECISION,
    average_hr         DOUBLE PRECISION,
    max_hr             DOUBLE PRECISION,
    raw                JSONB NOT NULL,
    cached_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source, source_activity_id)
);

CREATE INDEX idx_activity_cache_start ON activity_cache (source, start_time DESC);
