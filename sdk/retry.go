package sdk

import (
	"context"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// retryUnaryInterceptor returns a gRPC unary client interceptor that retries
// transient errors with exponential backoff and jitter.
//
// Transient codes: UNAVAILABLE, DEADLINE_EXCEEDED, RESOURCE_EXHAUSTED.
// Delay formula: min(baseDelay * 2^attempt + jitter, maxDelay), jitter up to 25%.
func retryUnaryInterceptor(maxRetries int, baseDelay, maxDelay time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var lastErr error
		for attempt := range maxRetries + 1 {
			lastErr = invoker(ctx, method, req, reply, cc, opts...)
			if lastErr == nil {
				return nil
			}

			if !isTransient(lastErr) {
				return lastErr
			}

			// Don't sleep after the last attempt.
			if attempt == maxRetries {
				break
			}

			delay := backoffDelay(baseDelay, maxDelay, attempt)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		return lastErr
	}
}

// isTransient reports whether err is a transient gRPC error that should be retried.
func isTransient(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// backoffDelay calculates the delay for the given attempt using exponential
// backoff with random jitter up to 25%.
//
//	delay = min(baseDelay * 2^attempt + jitter, maxDelay)
func backoffDelay(baseDelay, maxDelay time.Duration, attempt int) time.Duration {
	delay := baseDelay << attempt // baseDelay * 2^attempt

	// Add jitter: random value up to 25% of current delay.
	jitter := time.Duration(rand.Int64N(int64(delay/4) + 1))
	delay += jitter

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
