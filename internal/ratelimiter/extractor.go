package ratelimiter

import (
	"context"
	"net"

	"google.golang.org/grpc/peer"
)

// KeyExtractor определяет стратегию извлечения ключа rate limiting из контекста.
// Возвращает ключ (например, client IP) или пустую строку, если ключ не найден.
// При пустом ключе rate limiting пропускается (fail-open).
type KeyExtractor func(ctx context.Context) string

// PeerIPExtractor извлекает client IP из peer.FromContext().
// Это реализация по умолчанию, использующая IP, установленный realip interceptor.
func PeerIPExtractor(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}

	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}

	return host
}
