package db

import (
	"context"
	"errors"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type bankLinksRepo struct {
	pool *pgxpool.Pool
}

func NewBankLinksRepo(pool *pgxpool.Pool) BankLinksRepo {
	return &bankLinksRepo{pool: pool}
}

func (r *bankLinksRepo) GetLink(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (*BankLink, error) {
	const q = `
SELECT user_id, bank_code, consent_id, status, consent_expires_at
FROM bank_links
WHERE user_id = $1 AND bank_code = $2
LIMIT 1;
`
	row := r.pool.QueryRow(ctx, q, userID, string(bank))

	var bl BankLink
	var consentID *string
	var status *string
	var exp *time.Time

	if err := row.Scan(&bl.UserID, &bl.BankCode, &consentID, &status, &exp); err != nil {
		// pgx: no rows → вернём nil, nil (пусть вызывающий решает создавать)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, nil
	}
	if consentID != nil {
		bl.ConsentID = *consentID
	}
	if status != nil {
		bl.Status = *status
	}
	if exp != nil {
		bl.ConsentExpiresAt = exp
	}
	return &bl, nil
}

func (r *bankLinksRepo) UpsertConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode, id string, status string, expiresAt *time.Time) error {
	// Требуется уникальность на (user_id, bank_code) — см. миграцию 0003
	const q = `
INSERT INTO bank_links (user_id, bank_code, consent_id, status, consent_expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, bank_code)
DO UPDATE SET consent_id = EXCLUDED.consent_id,
              status = EXCLUDED.status,
              consent_expires_at = EXCLUDED.consent_expires_at,
              updated_at = now();
`
	_, err := r.pool.Exec(ctx, q, userID, string(bank), id, status, expiresAt)
	return err
}

func (r *bankLinksRepo) UpdateConsentStatus(ctx context.Context, userID uuid.UUID, bank auth.BankCode, status string, expiresAt *time.Time) error {
	const q = `
UPDATE bank_links
SET status = $3,
    consent_expires_at = $4,
    updated_at = now()
WHERE user_id = $1 AND bank_code = $2;
`
	_, err := r.pool.Exec(ctx, q, userID, string(bank), status, expiresAt)
	return err
}

func (r *bankLinksRepo) ClearConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) error {
	const q = `
UPDATE bank_links
SET consent_id = NULL,
    status = 'expired',
    consent_expires_at = NULL,
    updated_at = now()
WHERE user_id = $1 AND bank_code = $2;
`
	_, err := r.pool.Exec(ctx, q, userID, string(bank))
	return err
}
