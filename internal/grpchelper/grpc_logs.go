package grpchelper

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"

	"github.com/easyp-tech/service/internal/monitor"
)

const panicReason = "panic_reason"

func recoveryFunc(m Metrics, err error) grpc_recovery.RecoveryHandlerFuncContext {
	return func(ctx context.Context, p any) error {
		m.PanicsTotal().Inc()

		l := monitor.FromContext(ctx)
		l.Error("panic",
			slog.Any(panicReason, p),
			slog.String("stacktrace", string(debug.Stack())),
		)

		return err
	}
}

func interceptorLogger(_ *slog.Logger) logging.Logger { //nolint:ireturn // Required by interceptor interface.
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		logger := monitor.FromContext(ctx)
		switch lvl {
		case logging.LevelDebug:
			logger.Debug(msg, fields...)
		case logging.LevelInfo:
			logger.Info(msg, fields...)
		case logging.LevelWarn:
			logger.Warn(msg, fields...)
		case logging.LevelError:
			logger.Error(msg, fields...)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}
