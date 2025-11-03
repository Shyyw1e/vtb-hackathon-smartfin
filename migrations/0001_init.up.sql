-- 0001_init.up.sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    name            TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bank_links (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_code           TEXT NOT NULL CHECK (bank_code IN ('vbank', 'abank', 'sbank')),
    consent_id          TEXT,
    access_token_enc    BYTEA,
    refresh_token_enc   BYTEA,
    token_expires_at    TIMESTAMPTZ,
    status              TEXT DEFAULT 'active' CHECK (status IN ('active','revoked','expired')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bank_links_user ON bank_links(user_id);
CREATE INDEX IF NOT EXISTS idx_bank_links_bank ON bank_links(bank_code);

CREATE TABLE user_devices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id       TEXT NOT NULL,
    platform        TEXT CHECK (platform IN ('android','ios','web')),
    push_token      TEXT,
    app_version     TEXT,
    locale          TEXT,
    is_active       BOOLEAN DEFAULT true,
    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices(user_id);

CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_code_src   TEXT NOT NULL,
    account_src     TEXT NOT NULL,
    bank_code_dst   TEXT NOT NULL,
    account_dst     TEXT NOT NULL,
    amount          NUMERIC(14,2) NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'RUB',
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','completed','failed')),
    idem_key        TEXT,
    ext_payment_id  TEXT,
    error           JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_created ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_idem ON payments(idem_key);

CREATE TABLE vrp_consents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_code       TEXT NOT NULL,
    consent_id      TEXT NOT NULL,
    single_limit    NUMERIC(14,2),
    period_limit    NUMERIC(14,2),
    period          TEXT,
    expires_at      TIMESTAMPTZ,
    status          TEXT DEFAULT 'active' CHECK (status IN ('active','revoked','expired')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vrp_consents_user ON vrp_consents(user_id);
CREATE INDEX IF NOT EXISTS idx_vrp_consents_bank ON vrp_consents(bank_code);

CREATE TABLE vrp_schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    merchant_key    TEXT NOT NULL,
    amount_est      NUMERIC(14,2),
    next_charge     TIMESTAMPTZ,
    period_days     INT,
    source          TEXT DEFAULT 'detected',
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vrp_schedules_user_date ON vrp_schedules(user_id, next_charge);

CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    meta            JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id);

CREATE TABLE idempotency_keys (
    key             TEXT PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ
);
