ALTER TABLE bank_links
  DROP CONSTRAINT IF EXISTS uniq_bank_links_user_bank;

ALTER TABLE bank_links
  DROP COLUMN IF EXISTS consent_expires_at;

ALTER TABLE bank_links
  DROP CONSTRAINT IF EXISTS bank_links_status_check,
  ADD CONSTRAINT bank_links_status_check
    CHECK (status IN ('active','revoked','expired'));
