package anilistrl

import (
	"context"
	"sync"
	"time"
)

const minGap = 1500 * time.Millisecond

var (
	mu       sync.Mutex
	nextSlot time.Time
)

func Wait(ctx context.Context) error {
	mu.Lock()
	now := time.Now()
	if nextSlot.Before(now) {
		nextSlot = now
	}
	wait := nextSlot.Sub(now)
	nextSlot = nextSlot.Add(minGap)
	mu.Unlock()

	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func Backoff(retryAfterSeconds int) {
	if retryAfterSeconds <= 0 {
		retryAfterSeconds = 5
	}
	extend := time.Now().Add(time.Duration(retryAfterSeconds) * time.Second)
	mu.Lock()
	if extend.After(nextSlot) {
		nextSlot = extend
	}
	mu.Unlock()
}
