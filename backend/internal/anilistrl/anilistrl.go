package anilistrl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrRateLimited is returned after AniList 429 (HTTP or GraphQL) and the
// in-request retry has already waited out the cooldown.
var ErrRateLimited = errors.New("anilist: rate limited")

const (
	MinGap          = 3 * time.Second
	DefaultCooldown = 60 * time.Second
)

var (
	mu              sync.Mutex
	minGap          = MinGap
	defaultCooldown = DefaultCooldown
	nextSlot        time.Time
	coolUntil       time.Time
)

func Wait(ctx context.Context) error {
	mu.Lock()
	now := time.Now()
	earliest := nextSlot
	if coolUntil.After(earliest) {
		earliest = coolUntil
	}
	if earliest.Before(now) {
		earliest = now
	}
	wait := earliest.Sub(now)
	nextSlot = earliest.Add(minGap)
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
	d := time.Duration(retryAfterSeconds) * time.Second
	mu.Lock()
	if d <= 0 {
		d = defaultCooldown
	}
	until := time.Now().Add(d)
	if until.After(coolUntil) {
		coolUntil = until
	}
	mu.Unlock()
}

func CoolingDown() bool {
	mu.Lock()
	defer mu.Unlock()
	return time.Now().Before(coolUntil)
}

func Clear() {
	mu.Lock()
	coolUntil = time.Time{}
	mu.Unlock()
}

// Check records a cooldown and returns ErrRateLimited when AniList rate-limited
// this response. HTTP 429 and GraphQL errors with status 429 both count.
func Check(status int, retryAfter string, body []byte) error {
	if status == http.StatusTooManyRequests {
		n, _ := strconv.Atoi(strings.TrimSpace(retryAfter))
		Backoff(n)
		return ErrRateLimited
	}
	if graphqlLimited(body) {
		Backoff(0)
		return ErrRateLimited
	}
	return nil
}

func graphqlLimited(body []byte) bool {
	var env struct {
		Errors []struct {
			Message string `json:"message"`
			Status  int    `json:"status"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return strings.Contains(strings.ToLower(string(body)), "too many requests")
	}
	for _, e := range env.Errors {
		if e.Status == 429 || strings.Contains(strings.ToLower(e.Message), "too many requests") {
			return true
		}
	}
	return false
}

// TestingAdjust resets limiter state. Tests only.
func TestingAdjust(gap, cooldown time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	minGap = gap
	defaultCooldown = cooldown
	nextSlot = time.Time{}
	coolUntil = time.Time{}
}
