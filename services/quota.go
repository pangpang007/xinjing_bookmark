package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const quotaKeyPrefix = "interpret:quota:"

var errQuotaStoreUnavailable = errors.New("interpret quota store unavailable")

type quotaBackend interface {
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

type redisQuotaBackend struct {
	rdb *redis.Client
}

func (b redisQuotaBackend) Incr(ctx context.Context, key string) (int64, error) {
	return b.rdb.Incr(ctx, key).Result()
}

func (b redisQuotaBackend) Decr(ctx context.Context, key string) (int64, error) {
	return b.rdb.Decr(ctx, key).Result()
}

func (b redisQuotaBackend) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return b.rdb.Expire(ctx, key, ttl).Err()
}

type memQuotaBackend struct {
	mu      sync.Mutex
	counts  map[string]int64
	expires map[string]time.Duration
}

func newMemQuotaBackend() *memQuotaBackend {
	return &memQuotaBackend{
		counts:  make(map[string]int64),
		expires: make(map[string]time.Duration),
	}
}

func (m *memQuotaBackend) Incr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[key]++
	return m.counts[key], nil
}

func (m *memQuotaBackend) Decr(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[key]--
	return m.counts[key], nil
}

func (m *memQuotaBackend) Expire(_ context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expires[key] = ttl
	return nil
}

// InterpretQuota counts successful POST /interpret calls per user per calendar day
// in the configured timezone (default Asia/Shanghai).
type InterpretQuota struct {
	store quotaBackend
	loc   *time.Location
	limit int
}

func NewInterpretQuota(rdb *redis.Client, loc *time.Location, limit int) *InterpretQuota {
	if limit <= 0 {
		return nil
	}
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	q := &InterpretQuota{loc: loc, limit: limit}
	if rdb != nil {
		q.store = redisQuotaBackend{rdb: rdb}
	} else {
		log.Println("[WARN] interpret daily limit enabled but redis is unavailable; quota will not be enforced")
	}
	return q
}

// NewMemoryInterpretQuota is an in-process limiter for tests.
func NewMemoryInterpretQuota(loc *time.Location, limit int) *InterpretQuota {
	if limit <= 0 {
		return nil
	}
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &InterpretQuota{store: newMemQuotaBackend(), loc: loc, limit: limit}
}

func quotaKey(userID int64, day time.Time) string {
	return fmt.Sprintf("%s%d:%s", quotaKeyPrefix, userID, day.Format("2006-01-02"))
}

func ttlUntilEndOfDay(now time.Time, loc *time.Location) time.Duration {
	now = now.In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	ttl := end.Sub(now)
	if ttl < time.Second {
		return time.Second
	}
	return ttl
}

// Incr consumes one slot for today.
// exceeded means the increment was rolled back because the user is already at the limit.
// A non-nil err means the counter could not be updated; callers should fail-open.
func (q *InterpretQuota) Incr(ctx context.Context, userID int64) (used int, exceeded bool, err error) {
	if q == nil || q.limit <= 0 {
		return 0, false, nil
	}
	if q.store == nil {
		return 0, false, errQuotaStoreUnavailable
	}

	now := time.Now().In(q.loc)
	key := quotaKey(userID, now)
	n, err := q.store.Incr(ctx, key)
	if err != nil {
		return 0, false, err
	}
	if n == 1 {
		if expErr := q.store.Expire(ctx, key, ttlUntilEndOfDay(now, q.loc)); expErr != nil {
			log.Printf("[WARN] interpret quota expire: %v", expErr)
		}
	}
	if n > int64(q.limit) {
		if _, decrErr := q.store.Decr(ctx, key); decrErr != nil {
			return int(n), true, decrErr
		}
		return int(n), true, nil
	}
	return int(n), false, nil
}
