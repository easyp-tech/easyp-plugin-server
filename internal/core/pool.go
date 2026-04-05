package core

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/pyroscope-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/pluginpb"
)

// WorkerPoolConfig содержит параметры конфигурации worker pool.
type WorkerPoolConfig struct {
	Workers           int
	QueueSize         int
	GenerationTimeout time.Duration
	MaxRetries        int
	ShutdownTimeout   time.Duration
}

// job представляет единицу работы для воркера.
type job struct {
	ctx           context.Context
	pluginGroup   string
	pluginName    string
	pluginVersion string
	result        chan<- jobResult
}

// jobResult содержит результат обработки задания.
type jobResult struct {
	plugin Plugin
	err    error
}

// WorkerPool управляет пулом горутин для ограничения параллелизма Docker-выполнения.
// Реализует интерфейс Registry, оборачивая реальный Registry.
type WorkerPool struct {
	inner         Registry
	jobs          chan job
	cfg           WorkerPoolConfig
	logger        *slog.Logger
	metrics       Metrics
	wg            sync.WaitGroup
	closed        atomic.Bool
	activeWorkers prometheus.Gauge
	rejectedTotal prometheus.Counter
	jobsTotal     prometheus.Counter
	tracer        trace.Tracer
}

// poolPlugin оборачивает реальный Plugin, добавляя таймаут и retry при вызове Generate.
type poolPlugin struct {
	inner   Plugin
	cfg     WorkerPoolConfig
	logger  *slog.Logger
	metrics Metrics
}

// NewWorkerPool создаёт WorkerPool с нормализованной конфигурацией.
func NewWorkerPool(inner Registry, cfg WorkerPoolConfig, logger *slog.Logger, metrics Metrics, reg *prometheus.Registry, namespace string) *WorkerPool {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.QueueSize < 0 {
		cfg.QueueSize = 0
	}
	if cfg.GenerationTimeout == 0 {
		cfg.GenerationTimeout = 120 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 2
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}

	jobs := make(chan job, cfg.QueueSize)

	activeWorkers := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "pool_active_workers",
		Help:      "Number of workers currently processing jobs.",
	})

	rejectedTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pool_rejected_total",
		Help:      "Total number of jobs rejected due to full queue.",
	})

	jobsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pool_jobs_total",
		Help:      "Total number of jobs accepted into the queue.",
	})

	queueDepth := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "pool_queue_depth",
		Help:      "Current number of jobs in the worker pool queue.",
	}, func() float64 {
		return float64(len(jobs))
	})

	if reg != nil {
		reg.MustRegister(queueDepth, activeWorkers, rejectedTotal, jobsTotal)
	}

	return &WorkerPool{
		inner:         inner,
		jobs:          jobs,
		cfg:           cfg,
		logger:        logger,
		metrics:       metrics,
		activeWorkers: activeWorkers,
		rejectedTotal: rejectedTotal,
		jobsTotal:     jobsTotal,
		tracer:        otel.Tracer("pool"),
	}
}

// Start запускает N горутин-воркеров.
func (p *WorkerPool) Start(ctx context.Context) {
	for range p.cfg.Workers {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

func (p *WorkerPool) worker(ctx context.Context) {
	defer p.wg.Done()
	for j := range p.jobs {
		p.processJob(ctx, j)
	}
}

func (p *WorkerPool) processJob(_ context.Context, j job) {
	p.activeWorkers.Inc()
	defer p.activeWorkers.Dec()

	pyroscope.TagWrapper(j.ctx, pyroscope.Labels("operation", "worker.process_job"), func(ctx context.Context) {
		plugin, err := p.inner.Get(ctx, j.pluginGroup, j.pluginName, j.pluginVersion)
		if err != nil {
			j.result <- jobResult{err: err}
			return
		}

		wrapped := &poolPlugin{
			inner:   plugin,
			cfg:     p.cfg,
			logger:  p.logger,
			metrics: p.metrics,
		}

		j.result <- jobResult{plugin: wrapped}
	})
}

// Get создаёт задание и отправляет его в очередь с backpressure.
func (p *WorkerPool) Get(ctx context.Context, pluginGroup, pluginName, pluginVersion string) (Plugin, error) {
	ctx, span := p.tracer.Start(ctx, "pool.Get",
		trace.WithAttributes(
			attribute.String("plugin.group", pluginGroup),
			attribute.String("plugin.name", pluginName),
			attribute.String("plugin.version", pluginVersion),
		))
	defer span.End()

	if p.closed.Load() {
		return nil, ErrShuttingDown
	}

	resultCh := make(chan jobResult, 1)
	j := job{
		ctx:           ctx,
		pluginGroup:   pluginGroup,
		pluginName:    pluginName,
		pluginVersion: pluginVersion,
		result:        resultCh,
	}

	select {
	case p.jobs <- j:
		// Job принят в очередь
		p.jobsTotal.Inc()
	default:
		p.logger.Warn("job queue full, rejecting request")
		p.rejectedTotal.Inc()
		span.AddEvent("pool.rejected")
		return nil, ErrServerOverloaded
	}

	queueStart := time.Now()

	select {
	case res := <-resultCh:
		span.SetAttributes(attribute.Int64("pool.queue_wait_ms", time.Since(queueStart).Milliseconds()))
		return res.plugin, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// List проксирует запрос напрямую во внутренний Registry без очереди.
func (p *WorkerPool) List(ctx context.Context, filter PluginFilter) ([]PluginInfo, error) {
	return p.inner.List(ctx, filter)
}

// Create проксирует запрос напрямую во внутренний Registry без очереди.
func (p *WorkerPool) Create(ctx context.Context, req CreatePluginRequest) (*PluginInfo, error) {
	return p.inner.Create(ctx, req)
}

// Update проксирует запрос напрямую во внутренний Registry без очереди.
func (p *WorkerPool) Update(ctx context.Context, req UpdatePluginRequest) (*PluginInfo, error) {
	return p.inner.Update(ctx, req)
}

// Delete проксирует запрос напрямую во внутренний Registry без очереди.
func (p *WorkerPool) Delete(ctx context.Context, group, name, version string) error {
	return p.inner.Delete(ctx, group, name, version)
}

// Generate выполняет генерацию кода с таймаутом и retry.
func (pp *poolPlugin) Generate(ctx context.Context, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	genCtx, cancel := context.WithTimeout(ctx, pp.cfg.GenerationTimeout)
	defer cancel()

	info := pp.inner.Info(ctx)
	pluginName := info.Group + "/" + info.Name + ":" + info.Version

	start := time.Now()

	var resp *pluginpb.CodeGeneratorResponse
	var err error

	maxAttempts := pp.cfg.MaxRetries + 1
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			pp.metrics.ObserveGenerationDuration(ctx, pluginName, time.Since(start))
			return nil, ctxErr
		}

		resp, err = pp.inner.Generate(genCtx, req)
		if err == nil {
			pp.metrics.ObserveGenerationDuration(ctx, pluginName, time.Since(start))
			return resp, nil
		}

		if !isTransient(err) || attempt == maxAttempts {
			break
		}

		pp.metrics.IncGenerationRetries(ctx, pluginName)

		pp.logger.Warn("retrying generation",
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"error", err,
		)
	}

	pp.metrics.ObserveGenerationDuration(ctx, pluginName, time.Since(start))

	if err != nil {
		errorType := "permanent"
		if isTransient(err) {
			errorType = "transient"
		}
		pp.metrics.IncGenerationErrors(ctx, pluginName, errorType)
	}

	return resp, err
}

// Info проксирует запрос во внутренний Plugin.
func (pp *poolPlugin) Info(ctx context.Context) *PluginInfo {
	return pp.inner.Info(ctx)
}

// isTransient определяет, является ли ошибка временной.
func isTransient(err error) bool {
	if err == nil {
		return false
	}

	// context.DeadlineExceeded НЕ транзиентная
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// exec.ExitError с определёнными кодами выхода Docker
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 125, 126, 127:
			return true
		}
	}

	// Подстроки, указывающие на транзиентные ошибки
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "daemon"):
		return true
	case strings.Contains(msg, "temporary failure"):
		return true
	}

	return false
}

// Shutdown закрывает канал заданий и ожидает завершения воркеров.
// Возвращает количество потерянных заданий.
func (p *WorkerPool) Shutdown(timeout time.Duration) int {
	p.closed.Store(true)
	close(p.jobs)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return 0
	case <-time.After(timeout):
		lost := len(p.jobs)
		p.logger.Warn("shutdown timeout, jobs lost", "count", lost)
		return lost
	}
}
