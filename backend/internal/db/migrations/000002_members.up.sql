-- Other runners (Phase 9, ADR 0010): a runner can sign in with an email and a
-- password. Emails are stored lowercased by the application; the unique
-- constraint is what makes a second sign-up with the same address fail.
ALTER TABLE runners DROP CONSTRAINT runners_kind_check;
ALTER TABLE runners ADD CONSTRAINT runners_kind_check CHECK (kind IN ('owner', 'member', 'garmin'));
ALTER TABLE runners ADD COLUMN email TEXT UNIQUE;
ALTER TABLE runners ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE runners ADD COLUMN status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled'));
ALTER TABLE runners ADD COLUMN last_login_at TIMESTAMPTZ;
