package core

import "context"

type callerIPKey struct{}

// WithCallerIP returns a new context with the caller IP address stored.
func WithCallerIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, callerIPKey{}, ip)
}

// CallerIPFromContext extracts the caller IP address from the context.
// Returns "unknown" if not set.
func CallerIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(callerIPKey{}).(string); ok && ip != "" {
		return ip
	}
	return "unknown"
}
