CREATE TABLE IF NOT EXISTS sync_states (
    id               BIGSERIAL PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bank_code        TEXT NOT NULL, 
    last_sync_at     TIMESTAMPTZ,   
    last_cursor      TEXT,          
    last_txn_id      TEXT,          
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, bank_code)
);

CREATE INDEX IF NOT EXISTS idx_sync_states_user_bank ON sync_states(user_id, bank_code);
CREATE INDEX IF NOT EXISTS idx_sync_states_updated_at ON sync_states(updated_at DESC);
