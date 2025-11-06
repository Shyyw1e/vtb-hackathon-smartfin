package db

import (
	"context"
	"fmt"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	if dsn == "" {
		logger.Log.Error("NewPool: empty dsn")
		return nil, fmt.Errorf("empty dsn")
	}

	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Log.Errorf("NewPool: failed to parse config: %v", err)
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	if maxConns > 0 {
		pcfg.MaxConns = maxConns
	}

	pcfg.MaxConnIdleTime = 5 * time.Minute
	pcfg.HealthCheckPeriod = 30 * time.Second
	if pcfg.ConnConfig != nil {
		if pcfg.ConnConfig.RuntimeParams == nil {
			pcfg.ConnConfig.RuntimeParams = map[string]string{}
		}
		pcfg.ConnConfig.RuntimeParams["application_name"] = "smartfin-api"
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		logger.Log.Errorf("NewPool: pgxpool.NewWithConfig failed: %v", err)
		return nil, fmt.Errorf("pgxpool new: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		logger.Log.Errorf("NewPool: ping failed: %v", err)
		return nil, fmt.Errorf("pgx ping: %w", err)
	}

	logger.Log.Info("NewPool: postgres pool created")
	return pool, nil
}

func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		logger.Log.Error("Ping: invalid pool (nil)")
		return fmt.Errorf("invalid pool: nil")
	}
	if err := pool.Ping(ctx); err != nil {
		logger.Log.Errorf("Ping: ping failed: %v", err)
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

func Close(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	pool.Close()
	logger.Log.Info("Close: postgres pool closed")
}

func WithTx(ctx context.Context, pool PoolIface, fn func(ctx context.Context, tx pgx.Tx) error) (retErr error) {
	if pool == nil {
		return fmt.Errorf("WithTx: pool is nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		logger.Log.Errorf("WithTx: begin failed: %v", err)
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.Background())
			panic(p)
		}
	}()

	defer func() {
		if retErr != nil {
			if rbErr := tx.Rollback(context.Background()); rbErr != nil && rbErr != pgx.ErrTxClosed {
				logger.Log.Errorf("WithTx: rollback failed: %v", rbErr)
			}
			return
		}
		if cmErr := tx.Commit(context.Background()); cmErr != nil {
			logger.Log.Errorf("WithTx: commit failed: %v", cmErr)
			retErr = fmt.Errorf("commit failed: %w", cmErr)
		}
	}()

	if err := fn(ctx, tx); err != nil {
		logger.Log.Errorf("WithTx: function returned error: %v", err)
		retErr = err
		return retErr
	}

	return nil
}
