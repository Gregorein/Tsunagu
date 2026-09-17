package anilistrl

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestBackoffZeroUsesDefaultCooldown(t *testing.T) {
	TestingAdjust(time.Millisecond, 80*time.Millisecond)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	Backoff(0)
	if !CoolingDown() {
		t.Fatal("expected cooldown after Backoff(0)")
	}
	start := time.Now()
	if err := Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 60*time.Millisecond {
		t.Fatalf("Wait returned after %v, want ~80ms cooldown", d)
	}
}

func TestCheckHTTP429(t *testing.T) {
	TestingAdjust(time.Millisecond, 50*time.Millisecond)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	err := Check(http.StatusTooManyRequests, "", []byte(`{"errors":[{"message":"Too Many Requests.","status":429}]}`))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
	if !CoolingDown() {
		t.Fatal("expected cooldown")
	}
}

func TestCheckGraphQLBody429(t *testing.T) {
	TestingAdjust(time.Millisecond, 50*time.Millisecond)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	body := []byte(`{"data":null,"errors":[{"message":"Too Many Requests.","status":429}]}`)
	err := Check(http.StatusOK, "", body)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
	if !CoolingDown() {
		t.Fatal("expected cooldown for GraphQL 429")
	}
}

func TestCheckOK(t *testing.T) {
	TestingAdjust(0, time.Second)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	err := Check(http.StatusOK, "", []byte(`{"data":{"Viewer":{}}}`))
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if CoolingDown() {
		t.Fatal("no cooldown on success")
	}
}

func TestClearDropsCooldown(t *testing.T) {
	TestingAdjust(0, time.Hour)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	Backoff(0)
	if !CoolingDown() {
		t.Fatal("expected cooldown")
	}
	Clear()
	if CoolingDown() {
		t.Fatal("Clear should drop cooldown")
	}
}

func TestWaitCanceled(t *testing.T) {
	TestingAdjust(time.Hour, time.Hour)
	t.Cleanup(func() { TestingAdjust(MinGap, DefaultCooldown) })

	ctx, cancel := context.WithCancel(context.Background())
	Backoff(0)
	cancel()
	if err := Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
