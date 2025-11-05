package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/db"
	"github.com/pashagolub/pgxmock"
	"github.com/stretchr/testify/require"
	"github.com/jackc/pgconn"
)

func TestUpsertDevice_Success(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	repo := db.NewDeviceRepo(mockPool)

	userID := "11111111-1111-1111-1111-111111111111"
	deviceID := "device-123"
	platform := "android"
	pushToken := "token-abc"

	mockPool.ExpectExec(`INSERT INTO user_devices`).WithArgs(userID, deviceID, platform, pushToken).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	ctx := context.Background()
	start := time.Now()
	err = repo.UpsertDevice(ctx, userID, deviceID, platform, pushToken)
	elapsed := time.Since(start)

	require.NoError(t, err, "Upsert should succeed")
	require.Less(t, elapsed, 2*time.Second, "operation must be fast")

	require.NoError(t, mockPool.ExpectationsWereMet())
}

func TestUpsertDevice_PgForeignKeyError_ReturnsErrInvalidID(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	repo := db.NewDeviceRepo(mockPool)

	userID := "00000000-0000-0000-0000-000000000000" 
	deviceID := "device-999"
	platform := "ios"
	pushToken := "token-x"

	pgErr := &pgconn.PgError{
		Code:    "23503",
		Message: "insert or update on table \"user_devices\" violates foreign key constraint",
		Detail:  "Key (user_id)=(0000...) is not present in table \"users\"",
	}

	mockPool.ExpectExec(`INSERT INTO user_devices`).WithArgs(userID, deviceID, platform, pushToken).WillReturnError(pgErr)

	ctx := context.Background()
	err = repo.UpsertDevice(ctx, userID, deviceID, platform, pushToken)

	require.Error(t, err)
	require.True(t, errors.Is(err, db.ErrInvalidID), "expected ErrInvalidID for PG 23503")

	require.NoError(t, mockPool.ExpectationsWereMet())
}
