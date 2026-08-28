-- Unital shared schema: identity + property services.
-- Run on every postgres startup to ensure tables exist (CREATE IF NOT EXISTS is idempotent).

-- ── Identity: users & memberships ────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    full_name      TEXT NOT NULL DEFAULT '',
    phone          TEXT,
    national_code  TEXT,
    platform_role  TEXT NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_hash  TEXT PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS memberships (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    building_id UUID NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('manager','board_member','staff','owner','resident')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_building_id ON memberships(building_id);

-- ── Property: buildings, units, assets, parties, contracts ─────────────────────

CREATE TABLE IF NOT EXISTS buildings (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    code        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,
    address     TEXT NOT NULL DEFAULT '',
    floors      INT NOT NULL DEFAULT 0,
    features    JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_buildings_created_by ON buildings(created_by);

CREATE TABLE IF NOT EXISTS units (
    id          TEXT PRIMARY KEY,
    building_id TEXT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    floor       INT NOT NULL DEFAULT 0,
    number      TEXT NOT NULL,
    area_m2     DOUBLE PRECISION NOT NULL DEFAULT 0,
    rooms       INT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'vacant',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (building_id, number)
);
CREATE INDEX IF NOT EXISTS idx_units_building_id ON units(building_id);

CREATE TABLE IF NOT EXISTS assets (
    id          TEXT PRIMARY KEY,
    kind        TEXT NOT NULL CHECK (kind IN ('parking','warehouse')),
    building_id TEXT NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    code        TEXT NOT NULL,
    floor       INT NOT NULL DEFAULT 0,
    area_m2     DOUBLE PRECISION NOT NULL DEFAULT 0,
    unit_id     TEXT REFERENCES units(id) ON DELETE SET NULL,
    UNIQUE (kind, building_id, code)
);
CREATE INDEX IF NOT EXISTS idx_assets_building_id ON assets(building_id);
CREATE INDEX IF NOT EXISTS idx_assets_unit_id ON assets(unit_id);

CREATE TABLE IF NOT EXISTS unit_parties (
    id        TEXT PRIMARY KEY,
    unit_id   TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('owner','resident')),
    user_id   TEXT NOT NULL,
    from_at   TIMESTAMPTZ NOT NULL,
    to_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_unit_parties_unit_id ON unit_parties(unit_id);
CREATE INDEX IF NOT EXISTS idx_unit_parties_user_id ON unit_parties(user_id);

CREATE TABLE IF NOT EXISTS transfer_records (
    id               TEXT PRIMARY KEY,
    unit_id          TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    role             TEXT NOT NULL,
    previous_user_id  TEXT NOT NULL DEFAULT '',
    new_user_id      TEXT NOT NULL,
    effective_date   TIMESTAMPTZ NOT NULL,
    contract_number  TEXT NOT NULL DEFAULT '',
    recorded_by      TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_transfer_records_unit_id ON transfer_records(unit_id);

CREATE TABLE IF NOT EXISTS contracts (
    id               TEXT PRIMARY KEY,
    number           TEXT NOT NULL UNIQUE,
    type             TEXT NOT NULL CHECK (type IN ('purchase','rental','transfer')),
    unit_id          TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    first_party_id   TEXT NOT NULL,
    second_party_id  TEXT NOT NULL,
    title            TEXT NOT NULL DEFAULT '',
    amount           BIGINT NOT NULL DEFAULT 0,
    deposit_amount   BIGINT NOT NULL DEFAULT 0,
    start_date       TIMESTAMPTZ NOT NULL,
    end_date         TIMESTAMPTZ,
    duration_months  INT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'draft',
    first_signed     BOOLEAN NOT NULL DEFAULT FALSE,
    second_signed    BOOLEAN NOT NULL DEFAULT FALSE,
    signed_date      TIMESTAMPTZ,
    created_by       TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_contracts_unit_id ON contracts(unit_id);

CREATE TABLE IF NOT EXISTS contract_sequences (
    date_key TEXT PRIMARY KEY,
    last_seq INT NOT NULL DEFAULT 0
);
