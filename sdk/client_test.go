package sdk

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig_TimeoutValues(t *testing.T) {
	cfg := defaultConfig()

	if cfg.generateCodeTimeout != 30*time.Second {
		t.Fatalf("expected generateCodeTimeout=30s, got %v", cfg.generateCodeTimeout)
	}
	if cfg.listPluginsTimeout != 10*time.Second {
		t.Fatalf("expected listPluginsTimeout=10s, got %v", cfg.listPluginsTimeout)
	}
}

func TestWithTimeout_NoUserDeadline(t *testing.T) {
	c := &Client{cfg: &config{}}
	defaultTimeout := 5 * time.Second

	ctx, cancel := c.withTimeout(context.Background(), defaultTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}

	expected := time.Now().Add(defaultTimeout)
	diff := deadline.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Fatalf("deadline off by %v (tolerance 100ms)", diff)
	}
}

func TestWithTimeout_UserDeadlineEarlier(t *testing.T) {
	c := &Client{cfg: &config{}}
	defaultTimeout := 10 * time.Second
	userDeadline := time.Now().Add(2 * time.Second)

	userCtx, userCancel := context.WithDeadline(context.Background(), userDeadline)
	defer userCancel()

	ctx, cancel := c.withTimeout(userCtx, defaultTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}

	// User deadline (2s) is earlier than default (10s), so user deadline should be preserved.
	diff := deadline.Sub(userDeadline)
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Fatalf("expected user deadline to be preserved, diff=%v", diff)
	}
}

func TestWithTimeout_UserDeadlineLater(t *testing.T) {
	c := &Client{cfg: &config{}}
	defaultTimeout := 2 * time.Second
	userDeadline := time.Now().Add(30 * time.Second)

	userCtx, userCancel := context.WithDeadline(context.Background(), userDeadline)
	defer userCancel()

	ctx, cancel := c.withTimeout(userCtx, defaultTimeout)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected context to have a deadline")
	}

	// User deadline (30s) is later than default (2s), so default timeout should be used.
	expected := time.Now().Add(defaultTimeout)
	diff := deadline.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Fatalf("expected default timeout deadline, diff=%v", diff)
	}
}

func TestWithTimeout_CancelFuncWorks(t *testing.T) {
	c := &Client{cfg: &config{}}

	ctx, cancel := c.withTimeout(context.Background(), 5*time.Second)
	cancel()

	select {
	case <-ctx.Done():
		// expected — context should be cancelled
	default:
		t.Fatal("expected context to be done after cancel")
	}
}
