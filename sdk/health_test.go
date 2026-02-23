package sdk

import (
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func TestWithHealthCheck_SetsConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.enableHealthCheck {
		t.Fatal("expected enableHealthCheck=false by default")
	}

	interval := 15 * time.Second
	opt := WithHealthCheck(interval)
	opt.apply(cfg)

	if !cfg.enableHealthCheck {
		t.Fatal("expected enableHealthCheck=true after WithHealthCheck")
	}
	if cfg.healthCheckInterval != interval {
		t.Fatalf("expected healthCheckInterval=%v, got %v", interval, cfg.healthCheckInterval)
	}
}

func TestWithKeepaliveParams_SetsConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.keepaliveParams != nil {
		t.Fatal("expected keepaliveParams=nil by default")
	}

	params := keepalive.ClientParameters{
		Time:                20 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}
	opt := WithKeepaliveParams(params)
	opt.apply(cfg)

	if cfg.keepaliveParams == nil {
		t.Fatal("expected keepaliveParams to be set")
	}
	if cfg.keepaliveParams.Time != params.Time {
		t.Fatalf("expected Time=%v, got %v", params.Time, cfg.keepaliveParams.Time)
	}
	if cfg.keepaliveParams.Timeout != params.Timeout {
		t.Fatalf("expected Timeout=%v, got %v", params.Timeout, cfg.keepaliveParams.Timeout)
	}
	if !cfg.keepaliveParams.PermitWithoutStream {
		t.Fatal("expected PermitWithoutStream=true")
	}
}

func TestHealthMonitor_Stop(t *testing.T) {
	conn, err := grpc.NewClient(
		"passthrough:///localhost:0",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	hm := &healthMonitor{
		conn:     conn,
		interval: 50 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		hm.start()
		close(done)
	}()

	// Let the monitor run for a couple of ticks.
	time.Sleep(120 * time.Millisecond)

	hm.stop()

	select {
	case <-done:
		// Goroutine exited — success.
	case <-time.After(2 * time.Second):
		t.Fatal("healthMonitor goroutine did not exit after stop()")
	}
}

func TestDefaultConfig_HealthCheckDisabled(t *testing.T) {
	cfg := defaultConfig()

	if cfg.enableHealthCheck {
		t.Fatal("expected enableHealthCheck=false by default")
	}
	if cfg.healthCheckInterval != 30*time.Second {
		t.Fatalf("expected default healthCheckInterval=30s, got %v", cfg.healthCheckInterval)
	}
	if cfg.keepaliveParams != nil {
		t.Fatal("expected keepaliveParams=nil by default")
	}
}
