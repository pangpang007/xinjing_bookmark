package services

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestQuotaKey(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	day := time.Date(2026, 8, 19, 23, 30, 0, 0, loc)
	got := quotaKey(42, day)
	want := "interpret:quota:42:2026-08-19"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTTLUntilEndOfDay(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Date(2026, 8, 19, 22, 0, 0, 0, loc)
	ttl := ttlUntilEndOfDay(now, loc)
	if ttl != 2*time.Hour {
		t.Fatalf("ttl=%s", ttl)
	}

	nearMidnight := time.Date(2026, 8, 19, 23, 59, 59, 500e6, loc)
	ttl = ttlUntilEndOfDay(nearMidnight, loc)
	if ttl < time.Second {
		t.Fatalf("ttl too small: %s", ttl)
	}
}

func TestInterpretQuotaIncr(t *testing.T) {
	q := NewMemoryInterpretQuota(time.FixedZone("CST", 8*3600), 3)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		used, exceeded, err := q.Incr(ctx, 7)
		if err != nil || exceeded || used != i {
			t.Fatalf("i=%d used=%d exceeded=%v err=%v", i, used, exceeded, err)
		}
	}
	_, exceeded, err := q.Incr(ctx, 7)
	if err != nil || !exceeded {
		t.Fatalf("expected exceeded err=%v", err)
	}

	store := q.store.(*memQuotaBackend)
	key := quotaKey(7, time.Now().In(q.loc))
	if store.counts[key] != 3 {
		t.Fatalf("rolled-back count=%d", store.counts[key])
	}
	if store.expires[key] < time.Second {
		t.Fatal("missing TTL")
	}

	used, exceeded, err := q.Incr(ctx, 8)
	if err != nil || exceeded || used != 1 {
		t.Fatalf("other user used=%d exceeded=%v err=%v", used, exceeded, err)
	}
}

func TestInterpretQuotaConcurrentCap(t *testing.T) {
	q := NewMemoryInterpretQuota(time.FixedZone("CST", 8*3600), 3)
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, exceeded, err := q.Incr(ctx, 1)
			if err != nil {
				t.Errorf("incr: %v", err)
				return
			}
			results <- !exceeded
		}()
	}
	wg.Wait()
	close(results)

	ok := 0
	for allowed := range results {
		if allowed {
			ok++
		}
	}
	if ok != 3 {
		t.Fatalf("allowed=%d want 3", ok)
	}
	store := q.store.(*memQuotaBackend)
	if store.counts[quotaKey(1, time.Now().In(q.loc))] != 3 {
		t.Fatalf("final count=%d", store.counts[quotaKey(1, time.Now().In(q.loc))])
	}
}

func TestNewInterpretQuotaDisabled(t *testing.T) {
	if NewInterpretQuota(nil, nil, 0) != nil {
		t.Fatal("limit 0 should disable quota")
	}
	q := NewInterpretQuota(nil, nil, 3)
	if q == nil {
		t.Fatal("expected limiter")
	}
	_, _, err := q.Incr(context.Background(), 1)
	if err != errQuotaStoreUnavailable {
		t.Fatalf("err=%v", err)
	}
}
