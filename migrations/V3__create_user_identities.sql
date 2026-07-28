-- Migration: V3__create_user_identities
-- Created: 2026-07-27T21:45:00Z

CREATE TABLE user_identities (
    id               UUID PRIMARY KEY,
    app_id           UUID NOT NULL REFERENCES apps (id),
    user_id          UUID NOT NULL REFERENCES users (id),
    provider         TEXT NOT NULL,
    provider_subject TEXT NOT NULL,
    secret_hash      TEXT,
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);

CREATE UNIQUE INDEX user_identities_app_provider_subject_active_uq
    ON user_identities (app_id, provider, provider_subject)
    WHERE deleted_at IS NULL;

CREATE INDEX user_identities_user_id_idx
    ON user_identities (user_id)
    WHERE deleted_at IS NULL;

-- Move any existing password hashes, then drop the column.
INSERT INTO user_identities (id, app_id, user_id, provider, provider_subject, secret_hash, created_at, updated_at)
SELECT gen_random_uuid(), app_id, id, 'password', email, password_hash, created_at, updated_at
FROM users
WHERE password_hash IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE users DROP COLUMN password_hash;
