package sdk

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// captureHandler is a slog.Handler that stores the last log record's attributes.
type captureHandler struct {
	attrs map[string]slog.Value
}

func newCaptureHandler() *captureHandler {
	return &captureHandler{attrs: make(map[string]slog.Value)}
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		h.attrs[a.Key] = a.Value
		return true
	})
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

// mockMetricsCollector records the last call to RecordCall.
type mockMetricsCollector struct {
	method   string
	duration time.Duration
	code     codes.Code
	called   bool
}

func (m *mockMetricsCollector) RecordCall(method string, duration time.Duration, code codes.Code) {
	m.method = method
	m.duration = duration
	m.code = code
	m.called = true
}

func TestLoggingInterceptor_LogsMethodDurationCode(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	interceptor := loggingUnaryInterceptor(logger)
	invoker, _ := mockInvoker(nil) // success

	err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify method is logged.
	if v, ok := handler.attrs["method"]; !ok {
		t.Fatal("expected 'method' attribute in log")
	} else if v.String() != "/test.Service/Method" {
		t.Fatalf("expected method='/test.Service/Method', got %q", v.String())
	}

	// Verify duration is logged and non-negative.
	if v, ok := handler.attrs["duration"]; !ok {
		t.Fatal("expected 'duration' attribute in log")
	} else if v.Duration() < 0 {
		t.Fatalf("expected non-negative duration, got %v", v.Duration())
	}

	// Verify code is logged.
	if v, ok := handler.attrs["code"]; !ok {
		t.Fatal("expected 'code' attribute in log")
	} else if v.String() != codes.OK.String() {
		t.Fatalf("expected code='OK', got %q", v.String())
	}
}

func TestLoggingInterceptor_LogsErrorCode(t *testing.T) {
	handler := newCaptureHandler()
	logger := slog.New(handler)

	interceptor := loggingUnaryInterceptor(logger)
	invoker, _ := mockInvoker([]error{status.Error(codes.NotFound, "not found")})

	err := interceptor(context.Background(), "/test.Service/Fail", nil, nil, nil, invoker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if v, ok := handler.attrs["code"]; !ok {
		t.Fatal("expected 'code' attribute in log")
	} else if v.String() != codes.NotFound.String() {
		t.Fatalf("expected code='NotFound', got %q", v.String())
	}

	if v, ok := handler.attrs["method"]; !ok {
		t.Fatal("expected 'method' attribute in log")
	} else if v.String() != "/test.Service/Fail" {
		t.Fatalf("expected method='/test.Service/Fail', got %q", v.String())
	}
}

func TestMetricsInterceptor_RecordsCall(t *testing.T) {
	collector := &mockMetricsCollector{}
	interceptor := metricsUnaryInterceptor(collector)
	invoker, _ := mockInvoker(nil) // success

	err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if !collector.called {
		t.Fatal("expected RecordCall to be called")
	}
	if collector.method != "/test.Service/Method" {
		t.Fatalf("expected method='/test.Service/Method', got %q", collector.method)
	}
	if collector.code != codes.OK {
		t.Fatalf("expected code=OK, got %s", collector.code)
	}
	if collector.duration < 0 {
		t.Fatalf("expected non-negative duration, got %v", collector.duration)
	}
}

func TestMetricsInterceptor_RecordsErrorCode(t *testing.T) {
	collector := &mockMetricsCollector{}
	interceptor := metricsUnaryInterceptor(collector)
	invoker, _ := mockInvoker([]error{status.Error(codes.Internal, "internal error")})

	err := interceptor(context.Background(), "/test.Service/Fail", nil, nil, nil, invoker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !collector.called {
		t.Fatal("expected RecordCall to be called")
	}
	if collector.method != "/test.Service/Fail" {
		t.Fatalf("expected method='/test.Service/Fail', got %q", collector.method)
	}
	if collector.code != codes.Internal {
		t.Fatalf("expected code=Internal, got %s", collector.code)
	}
}

func TestWithUnaryInterceptor_AddsToConfig(t *testing.T) {
	cfg := defaultConfig()

	called := false
	custom := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		called = true
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	WithUnaryInterceptor(custom).apply(cfg)

	if len(cfg.unaryInterceptors) != 1 {
		t.Fatalf("expected 1 interceptor, got %d", len(cfg.unaryInterceptors))
	}

	// Verify the interceptor is functional by invoking it.
	invoker, _ := mockInvoker(nil)
	_ = cfg.unaryInterceptors[0](context.Background(), "/test", nil, nil, nil, invoker)
	if !called {
		t.Fatal("expected custom interceptor to be called")
	}
}

func TestWithLoggingInterceptor_AddsToConfig(t *testing.T) {
	cfg := defaultConfig()

	logger := slog.New(newCaptureHandler())
	WithLoggingInterceptor(logger).apply(cfg)

	if len(cfg.unaryInterceptors) != 1 {
		t.Fatalf("expected 1 interceptor, got %d", len(cfg.unaryInterceptors))
	}
}

func TestWithMetricsInterceptor_AddsToConfig(t *testing.T) {
	cfg := defaultConfig()

	collector := &mockMetricsCollector{}
	WithMetricsInterceptor(collector).apply(cfg)

	if len(cfg.unaryInterceptors) != 1 {
		t.Fatalf("expected 1 interceptor, got %d", len(cfg.unaryInterceptors))
	}
}
