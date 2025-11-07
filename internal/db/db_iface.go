package db

import (
	"context"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
)

type DBPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

type TxIface interface {
  Commit(ctx context.Context) error
  Rollback(ctx context.Context) error
}

type PoolIface interface {
  Begin(ctx context.Context) (pgx.Tx, error)
  Ping(ctx context.Context) error
}


type BankLink struct {
	UserID           uuid.UUID
	BankCode         auth.BankCode
	ConsentID        string
	Status           string
	ConsentExpiresAt *time.Time
}

type BankLinksRepo interface {
	GetLink(ctx context.Context, userID uuid.UUID, bank auth.BankCode) (*BankLink, error)
	UpsertConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode, id string, status string, expiresAt *time.Time) error
	UpdateConsentStatus(ctx context.Context, userID uuid.UUID, bank auth.BankCode, status string, expiresAt *time.Time) error
	ClearConsent(ctx context.Context, userID uuid.UUID, bank auth.BankCode) error
}
