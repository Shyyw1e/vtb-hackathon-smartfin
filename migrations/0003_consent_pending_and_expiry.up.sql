ALTER TABLE bank_links
  DROP CONSTRAINT IF EXISTS bank_links_status_check,
  ADD CONSTRAINT bank_links_status_check
    CHECK (status IN ('active','revoked','expired','pending'));

ALTER TABLE bank_links
  ADD COLUMN IF NOT EXISTS consent_expires_at TIMESTAMPTZ NULL;

ALTER TABLE bank_links
  ADD CONSTRAINT IF NOT EXISTS uniq_bank_links_user_bank
    UNIQUE (user_id, bank_code);
