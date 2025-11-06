package db

import (
	"context"

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

