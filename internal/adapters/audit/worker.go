package audit

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/easyp-tech/service/internal/core"
)

// Worker читает аудит-события из канала и записывает их в хранилище.
type Worker struct {
	store      core.AuditLog
	entries    <-chan core.AuditEntry
	entriesCh  chan core.AuditEntry
	logger     *slog.Logger
	done       chan struct{}
	eventsLost prometheus.Counter
	tracer     trace.Tracer
}

// NewWorker создаёт воркер с буферизированным каналом.
// Возвращает воркер и write-end канала для отправки аудит-событий.
func NewWorker(store core.AuditLog, bufferSize int, logger *slog.Logger, reg *prometheus.Registry, namespace string) (*Worker, chan<- core.AuditEntry) {
	ch := make(chan core.AuditEntry, bufferSize)

	eventsLost := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "audit_events_lost_total",
		Help:      "Total number of audit events lost.",
	})

	queueDepth := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "audit_queue_depth",
		Help:      "Current number of audit events in the queue.",
	}, func() float64 {
		return float64(len(ch))
	})

	if reg != nil {
		reg.MustRegister(queueDepth, eventsLost)
	}

	w := &Worker{
		store:      store,
		entries:    ch,
		entriesCh:  ch,
		logger:     logger,
		done:       make(chan struct{}),
		eventsLost: eventsLost,
		tracer:     otel.Tracer("audit"),
	}
	return w, ch
}

// Run запускает воркер. Блокирует до закрытия канала entries.
func (w *Worker) Run(ctx context.Context) {
	defer close(w.done)

	for entry := range w.entries {
		w.saveEntry(ctx, entry)
	}
}

// saveEntry сохраняет аудит-запись с трейсингом.
func (w *Worker) saveEntry(ctx context.Context, entry core.AuditEntry) {
	ctx, span := w.tracer.Start(ctx, "audit.save",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("audit.operation_type", entry.OperationType),
			attribute.String("audit.entry_id", entry.ID.String()),
		))
	defer span.End()

	if err := w.store.Save(ctx, entry); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		w.logger.Error("audit save failed", "error", err, "entry_id", entry.ID)
	}
}

// Shutdown закрывает канал и ожидает завершения записи с таймаутом.
// Возвращает количество потерянных событий (0 если все записаны вовремя).
func (w *Worker) Shutdown(timeout time.Duration) int {
	close(w.entriesCh)

	select {
	case <-w.done:
		return 0
	case <-time.After(timeout):
		lost := len(w.entries)
		w.eventsLost.Add(float64(lost))
		w.logger.Warn("audit shutdown timeout, events lost", "count", lost)
		return lost
	}
}
