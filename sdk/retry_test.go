package sdk

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockInvoker returns a grpc.UnaryInvoker that returns errors from the given
// slice on each successive call and counts total invocations. After the slice
// is exhausted every subsequent call returns nil (success).
func mockInvoker(errs []error) (grpc.UnaryInvoker, *atomic.Int32) {
	var calls atomic.Int32
	return func(_ context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		idx := int(calls.Add(1)) - 1
		if idx < len(errs) {
			return errs[idx]
		}
		return nil
	}, &calls
}

func TestRetry_SuccessOnFirstTry(t *testing.T) {
	interceptor := retryUnaryInterceptor(3, 1*time.Millisecond, 50*time.Millisecond)
	invoker, calls := mockInvoker(nil) // no errors — immediate success

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 invocation, got %d", got)
	}
}

func TestRetry_TransientThenSuccess(t *testing.T) {
	interceptor := retryUnaryInterceptor(3, 1*time.Millisecond, 50*time.Millisecond)
	invoker, calls := mockInvoker([]error{
		status.Error(codes.Unavailable, "unavailable"),
	})

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, invoker)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 invocations, got %d", got)
	}
}

func TestRetry_AllRetriesExhausted(t *testing.T) {
	maxRetries := 3
	interceptor := retryUnaryInterceptor(maxRetries, 1*time.Millisecond, 50*time.Millisecond)

	errs := []error{
		status.Error(codes.Unavailable, "err-1"),
		status.Error(codes.DeadlineExceeded, "err-2"),
		status.Error(codes.ResourceExhausted, "err-3"),
		status.Error(codes.Unavailable, "err-4"), // last error
	}
	invoker, calls := mockInvoker(errs)

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, invoker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should return the last error (err-4).
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unavailable || st.Message() != "err-4" {
		t.Fatalf("expected Unavailable/err-4, got %s/%s", st.Code(), st.Message())
	}

	// Total calls = maxRetries + 1 (initial attempt + 3 retries).
	if got := calls.Load(); got != int32(maxRetries+1) {
		t.Fatalf("expected %d invocations, got %d", maxRetries+1, got)
	}
}

func TestRetry_NonTransientError_NoRetry(t *testing.T) {
	interceptor := retryUnaryInterceptor(3, 1*time.Millisecond, 50*time.Millisecond)
	invoker, calls := mockInvoker([]error{
		status.Error(codes.NotFound, "not found"),
	})

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, invoker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %s", st.Code())
	}

	// Non-transient error — no retry, only 1 call.
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 invocation, got %d", got)
	}
}

func TestRetry_ContextCancelledDuringBackoff(t *testing.T) {
	interceptor := retryUnaryInterceptor(3, 500*time.Millisecond, 5*time.Second)
	invoker, calls := mockInvoker([]error{
		status.Error(codes.Unavailable, "unavailable"),
		status.Error(codes.Unavailable, "unavailable"), // won't be reached
	})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context shortly after the first failure, during the backoff wait.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := interceptor(ctx, "/test.Method", nil, nil, nil, invoker)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Only the first call should have been made; the retry was aborted during backoff.
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 invocation, got %d", got)
	}
}
