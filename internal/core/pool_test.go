package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"google.golang.org/protobuf/types/pluginpb"
)

// --- Mock types ---

type mockRegistry struct {
	getFn    func(ctx context.Context, group, name, version string) (Plugin, error)
	listFn   func(ctx context.Context, filter PluginFilter) ([]PluginInfo, error)
	createFn func(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error)
	updateFn func(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error)
	deleteFn func(ctx context.Context, group, name, version string) error
}

func (m *mockRegistry) Get(ctx context.Context, group, name, version string) (Plugin, error) {
	return m.getFn(ctx, group, name, version)
}

func (m *mockRegistry) List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error) {
	return m.listFn(ctx, filter)
}

func (m *mockRegistry) Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}

	return nil, nil
}

func (m *mockRegistry) Update(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, req)
	}

	return nil, nil
}

func (m *mockRegistry) Delete(ctx context.Context, group, name, version string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, group, name, version)
	}

	return nil
}

type mockPlugin struct {
	generateFn func(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
	infoFn     func(ctx context.Context) *PluginInfo
}

func (m *mockPlugin) Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	return m.generateFn(ctx, req)
}

func (m *mockPlugin) Info(ctx context.Context) *PluginInfo {
	return m.infoFn(ctx)
}

// mockMetrics is a no-op implementation of Metrics for testing.
type mockMetrics struct{}

func (mockMetrics) GenerateCode(_ context.Context, _ PluginInfo) error                     { return nil }
func (mockMetrics) ObserveGenerationDuration(_ context.Context, _ string, _ time.Duration) {}
func (mockMetrics) IncGenerationErrors(_ context.Context, _ string, _ string)              {}
func (mockMetrics) IncGenerationRetries(_ context.Context, _ string)                       {}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// --- Test 1: Config normalization ---

func TestNewWorkerPool_ConfigNormalization(t *testing.T) {
	logger := noopLogger()
	reg := &mockRegistry{}

	tests := []struct {
		name string
		in   WorkerPoolConfig
		want WorkerPoolConfig
	}{
		{
			name: "valid config unchanged",
			in: WorkerPoolConfig{
				Workers:           4,
				QueueSize:         16,
				GenerationTimeout: 60 * time.Second,
				MaxRetries:        3,
				ShutdownTimeout:   10 * time.Second,
			},
			want: WorkerPoolConfig{
				Workers:           4,
				QueueSize:         16,
				GenerationTimeout: 60 * time.Second,
				MaxRetries:        3,
				ShutdownTimeout:   10 * time.Second,
			},
		},
		{
			name: "workers=0 normalized to 1",
			in:   WorkerPoolConfig{Workers: 0},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "workers=-5 normalized to 1",
			in:   WorkerPoolConfig{Workers: -5},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "queue_size=-1 normalized to 0",
			in:   WorkerPoolConfig{Workers: 1, QueueSize: -1},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "queue_size=0 stays 0",
			in:   WorkerPoolConfig{Workers: 1, QueueSize: 0},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "generation_timeout=0 normalized to 120s",
			in:   WorkerPoolConfig{Workers: 1, GenerationTimeout: 0},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "max_retries=0 normalized to 2",
			in:   WorkerPoolConfig{Workers: 1, MaxRetries: 0},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
		{
			name: "shutdown_timeout=0 normalized to 30s",
			in:   WorkerPoolConfig{Workers: 1, ShutdownTimeout: 0},
			want: WorkerPoolConfig{Workers: 1, QueueSize: 0, GenerationTimeout: 120 * time.Second, MaxRetries: 2, ShutdownTimeout: 30 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewWorkerPool(reg, tt.in, logger, mockMetrics{}, nil, "")
			got := pool.cfg

			if got.Workers != tt.want.Workers {
				t.Errorf("Workers = %d, want %d", got.Workers, tt.want.Workers)
			}
			if got.QueueSize != tt.want.QueueSize {
				t.Errorf("QueueSize = %d, want %d", got.QueueSize, tt.want.QueueSize)
			}
			if got.GenerationTimeout != tt.want.GenerationTimeout {
				t.Errorf("GenerationTimeout = %v, want %v", got.GenerationTimeout, tt.want.GenerationTimeout)
			}
			if got.MaxRetries != tt.want.MaxRetries {
				t.Errorf("MaxRetries = %d, want %d", got.MaxRetries, tt.want.MaxRetries)
			}
			if got.ShutdownTimeout != tt.want.ShutdownTimeout {
				t.Errorf("ShutdownTimeout = %v, want %v", got.ShutdownTimeout, tt.want.ShutdownTimeout)
			}
		})
	}
}

// --- Test 2: Get success round-trip ---

func TestWorkerPool_Get_Success(t *testing.T) {
	wantResp := &pluginpb.CodeGeneratorResponse{
		Error: new(""),
	}

	mp := &mockPlugin{
		generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
			return wantResp, nil
		},
		infoFn: func(_ context.Context) *PluginInfo {
			return &PluginInfo{Name: "test-plugin"}
		},
	}

	reg := &mockRegistry{
		getFn: func(_ context.Context, group, name, version string) (Plugin, error) {
			if group != "grpc" || name != "go" || version != "v1.0.0" {
				t.Errorf("unexpected params: %s/%s:%s", group, name, version)
			}

			return mp, nil
		},
	}

	pool := NewWorkerPool(reg, WorkerPoolConfig{Workers: 1, QueueSize: 1}, noopLogger(), mockMetrics{}, nil, "")
	pool.Start(context.Background())
	defer pool.Shutdown(time.Second)

	plugin, err := pool.Get(context.Background(), "grpc", "go", "v1.0.0")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify it's a poolPlugin wrapping our mock
	pp, ok := plugin.(*poolPlugin)
	if !ok {
		t.Fatalf("expected *poolPlugin, got %T", plugin)
	}
	if pp.inner != mp {
		t.Fatal("poolPlugin does not wrap the expected mock plugin")
	}

	// Verify Generate passes through
	resp, err := plugin.Generate(context.Background(), &pluginpb.CodeGeneratorRequest{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp != wantResp {
		t.Errorf("Generate() response mismatch")
	}
}

// --- Test 3: Get backpressure ---

func TestWorkerPool_Get_Backpressure(t *testing.T) {
	workerStarted := make(chan struct{})
	blocked := make(chan struct{})

	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) {
			// Signal that the worker has started processing
			select {
			case workerStarted <- struct{}{}:
			default:
			}
			// Block until test releases
			<-blocked

			return &mockPlugin{
				generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
					return &pluginpb.CodeGeneratorResponse{}, nil
				},
				infoFn: func(_ context.Context) *PluginInfo { return &PluginInfo{} },
			}, nil
		},
	}

	// workers=1, queue_size=1 — buffer holds 1 job
	pool := NewWorkerPool(reg, WorkerPoolConfig{Workers: 1, QueueSize: 1}, noopLogger(), mockMetrics{}, nil, "")
	pool.Start(context.Background())
	defer func() {
		close(blocked)
		pool.Shutdown(2 * time.Second)
	}()

	// First job: worker picks it up and blocks in getFn
	go func() {
		_, _ = pool.Get(context.Background(), "g", "n", "v")
	}()

	// Wait for the worker to actually start processing the first job
	<-workerStarted

	// Second job: fills the buffer (queue_size=1)
	go func() {
		_, _ = pool.Get(context.Background(), "g", "n", "v")
	}()

	// Give time for the second job to land in the buffered channel
	time.Sleep(50 * time.Millisecond)

	// Third job: worker busy + buffer full → ErrServerOverloaded
	_, err := pool.Get(context.Background(), "g", "n", "v")
	if !errors.Is(err, ErrServerOverloaded) {
		t.Errorf("expected ErrServerOverloaded, got %v", err)
	}
}

// --- Test 4: Get after shutdown ---

func TestWorkerPool_Get_ShuttingDown(t *testing.T) {
	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) {
			return &mockPlugin{}, nil
		},
	}

	pool := NewWorkerPool(reg, WorkerPoolConfig{Workers: 1, QueueSize: 1}, noopLogger(), mockMetrics{}, nil, "")
	pool.Start(context.Background())
	pool.Shutdown(time.Second)

	_, err := pool.Get(context.Background(), "g", "n", "v")
	if !errors.Is(err, ErrShuttingDown) {
		t.Errorf("expected ErrShuttingDown, got %v", err)
	}
}

// --- Test 5: poolPlugin Generate timeout ---

func TestPoolPlugin_Generate_Timeout(t *testing.T) {
	mp := &mockPlugin{
		generateFn: func(ctx context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
			// Sleep longer than the timeout
			select {
			case <-time.After(5 * time.Second):
				return &pluginpb.CodeGeneratorResponse{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		infoFn: func(_ context.Context) *PluginInfo {
			return &PluginInfo{Group: "test", Name: "plugin", Version: "v1.0.0"}
		},
	}

	pp := &poolPlugin{
		inner: mp,
		cfg: WorkerPoolConfig{
			GenerationTimeout: 10 * time.Millisecond,
			MaxRetries:        0,
		},
		logger:  noopLogger(),
		metrics: mockMetrics{},
	}

	_, err := pp.Generate(context.Background(), &pluginpb.CodeGeneratorRequest{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

// --- Test 6: poolPlugin Generate retry success ---

func TestPoolPlugin_Generate_RetrySuccess(t *testing.T) {
	callCount := 0
	wantResp := &pluginpb.CodeGeneratorResponse{Error: new("")}

	mp := &mockPlugin{
		generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: transient error
				return nil, errors.New("connection refused")
			}
			// Second call: success
			return wantResp, nil
		},
		infoFn: func(_ context.Context) *PluginInfo {
			return &PluginInfo{Group: "test", Name: "plugin", Version: "v1.0.0"}
		},
	}

	pp := &poolPlugin{
		inner: mp,
		cfg: WorkerPoolConfig{
			GenerationTimeout: 5 * time.Second,
			MaxRetries:        2,
		},
		logger:  noopLogger(),
		metrics: mockMetrics{},
	}

	resp, err := pp.Generate(context.Background(), &pluginpb.CodeGeneratorRequest{})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp != wantResp {
		t.Error("response mismatch")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// --- Test 7: poolPlugin Generate retry exhausted ---

func TestPoolPlugin_Generate_RetryExhausted(t *testing.T) {
	callCount := 0
	transientErr := errors.New("connection refused")

	mp := &mockPlugin{
		generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
			callCount++

			return nil, transientErr
		},
		infoFn: func(_ context.Context) *PluginInfo {
			return &PluginInfo{Group: "test", Name: "plugin", Version: "v1.0.0"}
		},
	}

	pp := &poolPlugin{
		inner: mp,
		cfg: WorkerPoolConfig{
			GenerationTimeout: 5 * time.Second,
			MaxRetries:        1,
		},
		logger:  noopLogger(),
		metrics: mockMetrics{},
	}

	_, err := pp.Generate(context.Background(), &pluginpb.CodeGeneratorRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != transientErr.Error() {
		t.Errorf("expected %q, got %q", transientErr.Error(), err.Error())
	}
	// MaxRetries=1 means maxAttempts=2
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 + 1 retry), got %d", callCount)
	}
}

// --- Test 8: List proxies directly ---

func TestWorkerPool_List_Proxy(t *testing.T) {
	wantPlugins := []PluginInfo{
		{Name: "go", Group: "protobuf", Version: "v1.0.0"},
		{Name: "python", Group: "grpc", Version: "v2.0.0"},
	}

	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) {
			t.Fatal("Get should not be called for List")

			return nil, nil
		},
		listFn: func(_ context.Context, filter PluginFilter) ([]PluginInfo, error) {
			if filter.Group != "protobuf" {
				t.Errorf("unexpected filter group: %s", filter.Group)
			}

			return wantPlugins, nil
		},
	}

	pool := NewWorkerPool(reg, WorkerPoolConfig{Workers: 1, QueueSize: 1}, noopLogger(), mockMetrics{}, nil, "")
	// Note: List works without Start — it proxies directly

	got, err := pool.List(context.Background(), PluginFilter{Group: "protobuf"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != len(wantPlugins) {
		t.Fatalf("List() returned %d plugins, want %d", len(got), len(wantPlugins))
	}
	for i, p := range got {
		if p.Name != wantPlugins[i].Name || p.Group != wantPlugins[i].Group {
			t.Errorf("plugin[%d] = %+v, want %+v", i, p, wantPlugins[i])
		}
	}
}

// --- Test 9: Shutdown clean ---

func TestWorkerPool_Shutdown_Clean(t *testing.T) {
	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) {
			return &mockPlugin{
				generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
					return &pluginpb.CodeGeneratorResponse{}, nil
				},
				infoFn: func(_ context.Context) *PluginInfo { return &PluginInfo{} },
			}, nil
		},
	}

	pool := NewWorkerPool(reg, WorkerPoolConfig{Workers: 2, QueueSize: 4}, noopLogger(), mockMetrics{}, nil, "")
	pool.Start(context.Background())

	// Process a few jobs
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := pool.Get(context.Background(), "g", "n", "v")
			if err != nil {
				t.Errorf("job %d: Get() error = %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	lost := pool.Shutdown(5 * time.Second)
	if lost != 0 {
		t.Errorf("Shutdown() lost = %d, want 0", lost)
	}
}

// --- Test 10: Sentinel errors are distinct ---

func TestSentinelErrors_Distinct(t *testing.T) {
	// Verify sentinel errors exist and are distinct from each other
	if errors.Is(ErrServerOverloaded, ErrShuttingDown) {
		t.Error("ErrServerOverloaded should not match ErrShuttingDown")
	}
	if errors.Is(ErrServerOverloaded, ErrNotFound) {
		t.Error("ErrServerOverloaded should not match ErrNotFound")
	}
	if errors.Is(ErrShuttingDown, ErrNotFound) {
		t.Error("ErrShuttingDown should not match ErrNotFound")
	}

	// Verify error messages are meaningful
	if ErrServerOverloaded.Error() != "server overloaded" {
		t.Errorf("ErrServerOverloaded message = %q", ErrServerOverloaded.Error())
	}
	if ErrShuttingDown.Error() != "server shutting down" {
		t.Errorf("ErrShuttingDown message = %q", ErrShuttingDown.Error())
	}
}

// --- Test 11: isTransient behavior ---

func TestIsTransient(t *testing.T) {
	// 1. context.DeadlineExceeded -> false
	if isTransient(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be transient")
	}

	// 2. Process exit codes (125, 126, 127) -> false on new behavior
	for _, code := range []int{125, 126, 127} {
		cmd := exec.CommandContext(context.Background(), "sh", "-c", fmt.Sprintf("exit %d", code))
		err := cmd.Run()
		if err == nil {
			t.Fatalf("expected command to fail with code %d", code)
		}
		if isTransient(err) {
			t.Errorf("exit code %d should not be transient under new disk-plugin design", code)
		}
	}

	// 3. Command killed by SIGKILL -> false
	cmd := exec.CommandContext(context.Background(), "sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start sleep: %v", err)
	}
	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("failed to send SIGKILL: %v", err)
	}
	err := cmd.Wait()
	if err == nil {
		t.Fatal("expected sleep command to fail after SIGKILL")
	}
	if isTransient(err) {
		t.Error("process killed by SIGKILL should not be transient")
	}

	// 4. Random error -> false
	if isTransient(errors.New("some random error")) {
		t.Error("random error should not be transient")
	}
}
