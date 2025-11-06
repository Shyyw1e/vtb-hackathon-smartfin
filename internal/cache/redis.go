package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type RedisStore interface {
    Ping(ctx context.Context) error
    SetJSON(ctx context.Context, key string, v interface{}, ttl time.Duration) error
    GetJSON(ctx context.Context, key string, dest interface{}) (found bool, err error)
    SetNXJSON(ctx context.Context, key string, v interface{}, ttl time.Duration) (ok bool, err error)
    Del(ctx context.Context, keys ...string) error
    IncrWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error)
    Publish(ctx context.Context, channel string, msg interface{}) error
    Subscribe(ctx context.Context, channels ...string) *redis.PubSub
    Close() error
}

type store struct {
	client *redis.Client
}

func New(ctx context.Context, addr, password string, db int) (*store, error) {
    opt := &redis.Options{
        Addr:         addr,
        Password:     password,
        DB:           db,
        DialTimeout:  2 * time.Second,
        ReadTimeout:  2 * time.Second,
        WriteTimeout: 2 * time.Second,
    	MinIdleConns: 2,
        PoolSize:     10,
    }

    c := redis.NewClient(opt)

    pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    if err := c.Ping(pingCtx).Err(); err != nil {
        logger.Log.Errorf("Redis:\tping failed: %v", err)
        return nil, err
    }

    logger.Log.Info("Redis:\tclient ready")
    return &store{client: c}, nil
}

func (s *store) Ping(ctx context.Context) error {
    if s == nil || s.client == nil {
        return fmt.Errorf("redis: nil client")
    }
    return s.client.Ping(ctx).Err()
}

func (s *store) Del(ctx context.Context, keys ...string) error {
    if len(keys) == 0 {
        return nil
    }
    n, err := s.client.Del(ctx, keys...).Result()
    if err != nil {
        return err
    }
    if n > 0 {
        logger.Log.Debugf("Redis:\tdeleted %d keys", n)
    }
    return nil
}

func (s *store) Close() error {
    if s == nil || s.client == nil {
        return nil
    }
    err := s.client.Close()
    if err != nil {
        logger.Log.Errorf("Redis:\tclose error: %v", err)
        return err
    }
    logger.Log.Info("Redis:\tclosed")
    return nil
}

func (s *store) SetJSON(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	b, err := json.Marshal(v)
	if err != nil {
		logger.Log.Errorf("Redis:\tfailed to marshal interface 'v': %v", err)
		return err
	}

	p := s.client.Set(ctx, key, b, ttl)
	if err := p.Err(); err != nil {
		logger.Log.Errorf("Redis:\tfailed to set JSON: %v", err)
		return err
	}

	return nil
}

func (s *store) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	b, err := s.client.Get(ctx, key).Bytes()
	if err != nil{
		if err == redis.Nil {
			logger.Log.Debug("Redis:\t Get key: not found")
			return false, nil
		}
		logger.Log.Errorf("Redis:\t Get key:%v", err)
		return false, err
	}

	if err := json.Unmarshal(b, dest); err != nil {
		logger.Log.Errorf("Redis:\tfailed to unmarshal: %v", err)
		return false, err
	}
	return true, nil
}

func (s *store) SetNXJSON(ctx context.Context, key string, v interface{}, ttl time.Duration) (bool, error) {
	b, err := json.Marshal(v)
	if err != nil {
		logger.Log.Errorf("Redis:\tfailed to marshal interface 'v': %v", err)
		return false, err
	}

	ok, err := s.client.SetNX(ctx, key, b, ttl).Result()
	if err != nil {
		logger.Log.Errorf("Redis:\tfailed to set NX: %v", err)
		return false, err
	}

	logger.Log.Infof("Redis:\tSet NX result: %v", ok)
	return ok, nil
}

func (s *store) Publish(ctx context.Context, channel string, msg interface{}) error {
    switch v := msg.(type) {
    case []byte:
        return s.client.Publish(ctx, channel, v).Err()
    case string:
        return s.client.Publish(ctx, channel, v).Err()
    default:
        b, err := json.Marshal(v)
        if err != nil {
            return err
        }
        return s.client.Publish(ctx, channel, b).Err()
    }
}


func (s *store) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
    return s.client.Subscribe(ctx, channels...)
}


func (s *store) IncrWithExpire(ctx context.Context, key string, expire time.Duration) (int64, error) {
    n, err := s.client.Incr(ctx, key).Result()
    if err != nil {
		logger.Log.Errorf("Redis:\tfailed to increment: %v", err)
        return 0, err
    }

    if n == 1 {
        _ = s.client.Expire(ctx, key, expire).Err()
    }

	logger.Log.Debug("Redis:\tIncremented")
    return n, nil
}
