DELETE FROM runners WHERE kind = 'member';
ALTER TABLE runners DROP COLUMN last_login_at;
ALTER TABLE runners DROP COLUMN status;
ALTER TABLE runners DROP COLUMN password_hash;
ALTER TABLE runners DROP COLUMN email;
ALTER TABLE runners DROP CONSTRAINT runners_kind_check;
ALTER TABLE runners ADD CONSTRAINT runners_kind_check CHECK (kind IN ('owner', 'garmin'));
