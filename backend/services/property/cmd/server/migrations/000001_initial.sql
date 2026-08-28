-- Unital property service — initial schema

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
-- Current party per (unit_id, role): only one row with to_at IS NULL.
CREATE UNIQUE INDEX IF NOT EXISTS idx_unit_parties_current
    ON unit_parties(unit_id, role) WHERE to_at IS NULL;

CREATE TABLE IF NOT EXISTS transfer_records (
    id              TEXT PRIMARY KEY,
    unit_id         TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    previous_user_id TEXT NOT NULL DEFAULT '',
    new_user_id     TEXT NOT NULL,
    effective_date  TIMESTAMPTZ NOT NULL,
    contract_number TEXT NOT NULL DEFAULT '',
    recorded_by     TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_transfer_records_unit_id ON transfer_records(unit_id);

CREATE TABLE IF NOT EXISTS contracts (
    id              TEXT PRIMARY KEY,
    number          TEXT NOT NULL UNIQUE,
    type            TEXT NOT NULL CHECK (type IN ('purchase','rental','transfer')),
    unit_id         TEXT NOT NULL REFERENCES units(id) ON DELETE CASCADE,
    first_party_id  TEXT NOT NULL,
    second_party_id TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    amount          BIGINT NOT NULL DEFAULT 0,
    deposit_amount  BIGINT NOT NULL DEFAULT 0,
    start_date      TIMESTAMPTZ NOT NULL,
    end_date        TIMESTAMPTZ,
    duration_months INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'draft',
    first_signed    BOOLEAN NOT NULL DEFAULT FALSE,
    second_signed   BOOLEAN NOT NULL DEFAULT FALSE,
    signed_date     TIMESTAMPTZ,
    created_by      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_contracts_unit_id ON contracts(unit_id);

CREATE TABLE IF NOT EXISTS contract_sequences (
    date_key TEXT PRIMARY KEY,
    last_seq INT NOT NULL DEFAULT 0
);
