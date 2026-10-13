-- Fresh schema for the personal MVP (ADR 0002, 0008). Every table that holds a
-- runner's data carries runner_id from the start; the owner is row one.

CREATE TABLE runners (
    id                 BIGSERIAL PRIMARY KEY,
    kind               TEXT NOT NULL CHECK (kind IN ('owner', 'garmin')),
    garmin_profile_id  TEXT UNIQUE,
    display_name       TEXT NOT NULL,
    daily_question_cap INT NOT NULL DEFAULT 30,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO runners (kind, display_name) VALUES ('owner', 'Owner');

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    runner_id    BIGINT NOT NULL REFERENCES runners(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_runner ON sessions (runner_id);

-- One Garmin connection per runner. blob is the encrypted token bundle
-- (AES-256-GCM under RUNCOACH_TOKEN_KEY, key_version for rotation).
CREATE TABLE garmin_connections (
    runner_id      BIGINT PRIMARY KEY REFERENCES runners(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL CHECK (kind IN ('local_tokens', 'gateway_token', 'byo_url')),
    blob           BYTEA NOT NULL,
    key_version    INT NOT NULL,
    status         TEXT NOT NULL CHECK (status IN ('connected', 'expired', 'unavailable')),
    imported_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_ok_at     TIMESTAMPTZ,
    expires_est_at TIMESTAMPTZ,
    last_error     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE chat_sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runner_id  BIGINT NOT NULL REFERENCES runners(id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_chat_sessions_runner ON chat_sessions (runner_id, updated_at DESC);

-- tool_calls is the trail behind an assistant message; chart is the one
-- validated chart spec it carried, or NULL.
CREATE TABLE chat_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    runner_id       BIGINT NOT NULL REFERENCES runners(id) ON DELETE CASCADE,
    role            TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content         TEXT NOT NULL DEFAULT '',
    tool_calls      JSONB,
    chart           JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_chat_messages_session ON chat_messages (chat_session_id, created_at);
CREATE INDEX idx_chat_messages_runner ON chat_messages (runner_id);

-- Per-runner, per-day counters: the cap and the cost guard (ADR 0002, 0006).
CREATE TABLE usage_days (
    runner_id           BIGINT NOT NULL REFERENCES runners(id) ON DELETE CASCADE,
    day                 DATE NOT NULL,
    questions           INT NOT NULL DEFAULT 0,
    input_tokens        BIGINT NOT NULL DEFAULT 0,
    cached_input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens       BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (runner_id, day)
);

CREATE TABLE audit_log (
    id        BIGSERIAL PRIMARY KEY,
    runner_id BIGINT REFERENCES runners(id) ON DELETE SET NULL,
    action    TEXT NOT NULL,
    at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    detail    JSONB
);
CREATE INDEX idx_audit_log_runner ON audit_log (runner_id, at DESC);
