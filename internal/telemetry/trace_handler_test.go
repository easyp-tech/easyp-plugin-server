package telemetry

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// captureHandler is a minimal slog.Handler that captures the last record.
type captureHandler struct {
	record  slog.Record
	enabled bool
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return h.enabled }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.record = r
	return nil
}
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func TestTraceHandler_Handle_ValidSpanContext(t *testing.T) {
	inner := &captureHandler{enabled: true}
	handler := NewTraceHandler(inner)

	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	r := slog.Record{}
	r.AddAttrs(slog.String("existing", "value"))

	if err := handler.Handle(ctx, r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	attrs := collectAttrs(inner.record)

	if v, ok := attrs["trace_id"]; !ok || v != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("trace_id = %q, want %q", v, "0af7651916cd43dd8448eb211c80319c")
	}
	if v, ok := attrs["span_id"]; !ok || v != "00f067aa0ba902b7" {
		t.Errorf("span_id = %q, want %q", v, "00f067aa0ba902b7")
	}
	if v, ok := attrs["existing"]; !ok || v != "value" {
		t.Errorf("existing attr = %q, want %q", v, "value")
	}
}

func TestTraceHandler_Handle_NoSpanContext(t *testing.T) {
	inner := &captureHandler{enabled: true}
	handler := NewTraceHandler(inner)

	r := slog.Record{}
	r.AddAttrs(slog.String("key", "val"))

	if err := handler.Handle(context.Background(), r); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	attrs := collectAttrs(inner.record)

	if _, ok := attrs["trace_id"]; ok {
		t.Error("trace_id should not be present without valid SpanContext")
	}
	if _, ok := attrs["span_id"]; ok {
		t.Error("span_id should not be present without valid SpanContext")
	}
	if v := attrs["key"]; v != "val" {
		t.Errorf("key = %q, want %q", v, "val")
	}
}

func TestTraceHandler_Enabled_DelegatesToInner(t *testing.T) {
	inner := &captureHandler{enabled: false}
	handler := NewTraceHandler(inner)

	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled should return false when inner returns false")
	}

	inner.enabled = true
	if !handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled should return true when inner returns true")
	}
}

func TestTraceHandler_WithAttrs_ReturnsNewTraceHandler(t *testing.T) {
	inner := &captureHandler{enabled: true}
	handler := NewTraceHandler(inner)

	wrapped := handler.WithAttrs([]slog.Attr{slog.String("a", "b")})
	if _, ok := wrapped.(*TraceHandler); !ok {
		t.Errorf("WithAttrs returned %T, want *TraceHandler", wrapped)
	}
}

func TestTraceHandler_WithGroup_ReturnsNewTraceHandler(t *testing.T) {
	inner := &captureHandler{enabled: true}
	handler := NewTraceHandler(inner)

	wrapped := handler.WithGroup("grp")
	if _, ok := wrapped.(*TraceHandler); !ok {
		t.Errorf("WithGroup returned %T, want *TraceHandler", wrapped)
	}
}

// collectAttrs extracts all attributes from a slog.Record into a map.
func collectAttrs(r slog.Record) map[string]string {
	m := make(map[string]string)
	r.Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value.String()
		return true
	})
	return m
}
