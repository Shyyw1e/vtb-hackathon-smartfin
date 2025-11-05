package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"

	"github.com/jackc/pgconn"
	//"github.com/jackc/pgx/v5"
	//"github.com/jackc/pgx/v5/pgxpool"
)

type Device struct {
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	Platform   string    `json:"platform"`
	PushToken  string    `json:"push_token,omitempty"`
	IsActive   bool      `json:"is_active"`
	LastSeenAt time.Time `json:"last_seen_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

var (
	ErrConflict  = errors.New("conflict")
	ErrNotFound  = errors.New("not found")
	ErrDB        = errors.New("db error")
	ErrInvalidID = errors.New("invalid id")
)

type DeviceRepo interface {
	UpsertDevice(ctx context.Context, userID, deviceID, platform, pushToken string) error
	GetDevicesByUser(ctx context.Context, userID string) ([]Device, error)
}

type pgDeviceRepo struct {
	pool DBPool
}

func NewDeviceRepo(pool DBPool) DeviceRepo {
	return &pgDeviceRepo{pool: pool}
}

func (r *pgDeviceRepo) UpsertDevice(ctx context.Context, userID, deviceID, platform, pushToken string) error {
	if userID == "" || deviceID == "" {
		return ErrInvalidID
	}

	opTimeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	q := `
INSERT INTO user_devices (user_id, device_id, platform, push_token, last_seen_at, is_active, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), true, now(), now())
ON CONFLICT (user_id, device_id)
  DO UPDATE SET
    push_token = EXCLUDED.push_token,
    platform = EXCLUDED.platform,
    last_seen_at = now(),
    is_active = true,
    updated_at = now();
`

	start := time.Now()
	_, err := r.pool.Exec(ctx, q, userID, deviceID, platform, pushToken)
	duration := time.Since(start)

	if duration > 300*time.Millisecond {
		logger.Log.Warnf("UpsertDevice slow: duration=%s user=%s device=%s", duration, userID, deviceID)
	}

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return ErrConflict
			case "23503":
				return ErrInvalidID
			default:
				logger.Log.Errorf("UpsertDevice: pg error code=%s message=%s detail=%s", pgErr.Code, pgErr.Message, pgErr.Detail)
				return fmt.Errorf("%w: %s", ErrDB, pgErr.Message)
			}
		}
		logger.Log.Errorf("UpsertDevice: exec failed: %v", err)
		return fmt.Errorf("%w: %v", ErrDB, err)
	}

	return nil
}

func (r *pgDeviceRepo) GetDevicesByUser(ctx context.Context, userID string) ([]Device, error) {
	if userID == "" {
		return nil, ErrInvalidID
	}

	opTimeout := 3 * time.Second
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	q := `
SELECT user_id, device_id, platform, push_token, is_active, last_seen_at, created_at, updated_at
FROM user_devices
WHERE user_id = $1
ORDER BY last_seen_at DESC
LIMIT 100;
`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			logger.Log.Errorf("GetDevicesByUser: pg error code=%s message=%s", pgErr.Code, pgErr.Message)
			return nil, fmt.Errorf("%w: %s", ErrDB, pgErr.Message)
		}
		logger.Log.Errorf("GetDevicesByUser: query failed: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrDB, err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.UserID, &d.DeviceID, &d.Platform, &d.PushToken, &d.IsActive, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			logger.Log.Errorf("GetDevicesByUser: scan failed: %v", err)
			return nil, fmt.Errorf("%w: %v", ErrDB, err)
		}
		devices = append(devices, d)
	}
	if rows.Err() != nil {
		logger.Log.Errorf("GetDevicesByUser: rows error: %v", rows.Err())
		return nil, fmt.Errorf("%w: %v", ErrDB, rows.Err())
	}

	if len(devices) == 0 {
		return nil, ErrNotFound
	}

	return devices, nil
}
