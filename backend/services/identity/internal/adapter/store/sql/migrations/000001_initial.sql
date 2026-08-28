-- Unital identity service — initial schema
-- +migrate Up

-- Users: one row per registered account.
CREATE TABLE IF NOT EXISTS users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    full_name      TEXT NOT NULL DEFAULT '',
    phone          TEXT,
    national_code  TEXT,
    -- PlatformRole is the user's global/company role (system_admin,
    -- supervisor, developer). Building-scoped roles live in memberships.
    platform_role  TEXT NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Refresh tokens: hashed token → user_id + expiry.
CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- Memberships: links a user to a building with a per-building role.
-- The role is NOT a platform role — it's scoped to the building.
CREATE TABLE IF NOT EXISTS memberships (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    building_id UUID NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('manager','board_member','staff','owner','resident')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_building_id ON memberships(building_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_memberships_user_building
    ON memberships(user_id, building_id);

-- +migrate Down

DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
