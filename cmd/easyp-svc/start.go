package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/ratelimit"
	"github.com/hellofresh/health-go/v5"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sethvargo/go-envconfig"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	adapter_audit "github.com/easyp-tech/service/internal/adapters/audit"
	adapter_metrics "github.com/easyp-tech/service/internal/adapters/metrics"
	"github.com/easyp-tech/service/internal/adapters/registry"
	"github.com/easyp-tech/service/internal/api"
	"github.com/easyp-tech/service/internal/config"
	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
	"github.com/easyp-tech/service/internal/database/connectors"
	"github.com/easyp-tech/service/internal/database/goosemigrate"
	"github.com/easyp-tech/service/internal/flags"
	"github.com/easyp-tech/service/internal/grpchelper"
	"github.com/easyp-tech/service/internal/license"
	"github.com/easyp-tech/service/internal/monitor"
	"github.com/easyp-tech/service/internal/ratelimiter"
	"github.com/easyp-tech/service/internal/telemetry"
)

const (
	// serviceNamespace is the fixed namespace for Prometheus metrics.
	serviceNamespace         = "easyp"
	exitCode                 = 2
	configFileSize           = 1024 * 1024
	auditChannelCapacity     = 1000
	componentShutdownTimeout = 5 * time.Second
)

// grpcMetrics implements grpchelper.Metrics for the recovery interceptor.
type grpcMetrics struct {
	panics prometheus.Counter
}

func (m *grpcMetrics) PanicsTotal() prometheus.Counter { //nolint:ireturn // interface requires prometheus.Counter
	return m.panics
}

// runServiceStart initializes and runs the complete service
func runServiceStart(ctxParent context.Context, cfgPath string, logLevelStr string) error {
	// Parse log level
	var slogLvl slog.Level
	if err := slogLvl.UnmarshalText([]byte(logLevelStr)); err != nil {
		slogLvl = slog.LevelDebug // fallback
	}

	log := buildLogger(slogLvl)

	ctxParent = monitor.WithContext(ctxParent, log.With(
		slog.String("version", "dev"),
		slog.String("app", serviceNamespace),
	))

	ctx, cancel := signal.NotifyContext(ctxParent, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	defer cancel()

	go forceShutdown(ctx)

	cfg := config.Config{}

	if cfgPath != "" {
		// Use flags.File logic manually
		f := &flags.File{DefaultPath: "", MaxSize: configFileSize}
		if err := f.Set(cfgPath); err != nil {
			return fmt.Errorf("read config file: %w", err)
		}

		err := yaml.NewDecoder(f).Decode(&cfg)
		if err != nil {
			return fmt.Errorf("yaml.NewDecoder.Decode: %w", err)
		}
	} else {
		err := envconfig.Process(ctx, &cfg)
		if err != nil {
			return fmt.Errorf("envconfig.Process: %w", err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

	err := run(ctx, cfg, reg)
	if err != nil {
		log.Error("shutdown", "error", err)
		os.Exit(exitCode)
	}

	return nil
}

func run(ctx context.Context, cfg config.Config, reg *prometheus.Registry) error {
	namespace := serviceNamespace
	log := monitor.FromContext(ctx)

	// Initialize telemetry (before anything else)
	telCfg := telemetry.Config{
		OTLPEndpoint:      cfg.Telemetry.OTLPEndpoint,
		ServiceName:       namespace,
		PyroscopeEndpoint: cfg.Telemetry.PyroscopeEndpoint,
	}
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true, Level: slog.LevelDebug})
	shutdownTelemetry, telLog, err := telemetry.Init(ctx, telCfg, baseHandler)
	if err != nil {
		return fmt.Errorf("telemetry.Init: %w", err)
	}
	log = telLog
	ctx = monitor.WithContext(ctx, log)

	err = goosemigrate.Up(ctx, cfg.DB.Postgres)
	if err != nil {
		return fmt.Errorf("goosemigrate.Up: %w", err)
	}

	dalMetrics := database.NewMetrics(reg, namespace, "repo", new(core.Registry))
	sqlCfg := database.SQLConfig{Metrics: dalMetrics}
	db, err := database.NewSQL(ctx, cfg.DB.Driver, sqlCfg, &connectors.Raw{Query: cfg.DB.Postgres})
	if err != nil {
		return fmt.Errorf("database.NewSQL: %w", err)
	}

	repo, err := registry.New(ctx, db, cfg.Registry.PluginsDir, cfg.Registry.MaxOutputSize)
	if err != nil {
		return fmt.Errorf("registry.New: %w", err)
	}

	defer func() {
		err := repo.Close()
		if err != nil {
			log.Error("close database connection", "error", err)
		}
	}()

	dbCollector := adapter_metrics.NewDBCollector(repo.DB().UnderlyingDB(), namespace)
	reg.MustRegister(dbCollector)

	businessCollector := adapter_metrics.NewBusinessMetricsCollector(repo.DB().UnderlyingDB(), namespace, log)
	reg.MustRegister(businessCollector)

	auditStore := adapter_audit.New(repo.DB(), log)
	auditWorker, auditCh := adapter_audit.NewWorker(auditStore, auditChannelCapacity, log, reg, namespace)

	go auditWorker.Run(ctx)

	defer func() {
		lost := auditWorker.Shutdown(componentShutdownTimeout)
		if lost > 0 {
			log.Warn("audit events lost on shutdown", "count", lost)
		}
	}()

	licenseClient := license.NewMockLicenseClient()
	lm, err := license.NewManager(ctx, licenseClient, license.Config{
		CacheTTL: cfg.License.CacheTTL,
	}, log, reg, namespace)
	if err != nil {
		log.Warn("license initialization error, continuing in community mode", "error", err)
	}
	lm.StartRefreshWatcher(ctx)

	gate := license.NewFeatureGate(lm)

	rl := ratelimiter.New(ratelimiter.Config{
		RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
		Burst:             cfg.RateLimit.Burst,
		CleanupInterval:   cfg.RateLimit.CleanupInterval,
	}, gate, nil, log, reg)
	rl.StartCleanup(ctx)

	tracedRegistry := telemetry.NewTracingRegistry(repo)

	wpWorkers := cfg.WorkerPool.Workers
	if licenseWorkers := gate.MaxWorkers(); licenseWorkers > 0 {
		wpWorkers = licenseWorkers
	}

	metricsAdapter := adapter_metrics.New(reg, namespace)
	pool := core.NewWorkerPool(tracedRegistry, core.WorkerPoolConfig{
		Workers:           wpWorkers,
		QueueSize:         cfg.WorkerPool.QueueSize,
		GenerationTimeout: cfg.WorkerPool.GenerationTimeout,
		MaxRetries:        cfg.WorkerPool.MaxRetries,
		ShutdownTimeout:   cfg.WorkerPool.ShutdownTimeout,
	}, log, metricsAdapter, reg, namespace)
	pool.Start(ctx)

	defer func() {
		lost := pool.Shutdown(cfg.WorkerPool.ShutdownTimeout)
		if lost > 0 {
			log.Warn("generation jobs lost on shutdown", "count", lost)
		}
	}()

	module := core.New(metricsAdapter, pool, gate, auditCh, log)
	tracedCore := telemetry.NewTracingCore(module)
	serverMetrics := grpchelper.NewServerMetrics(reg, "easyp", "api")

	panicsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "easyp",
		Name:      "panics_total",
		Help:      "Total number of panics recovered in gRPC handlers.",
	})
	reg.MustRegister(panicsCounter)

	licenseInterceptor := api.NewLicenseInterceptor(gate, log)

	grpcSrv, healthSrv := grpchelper.NewServer(
		&grpcMetrics{panics: panicsCounter},
		log,
		serverMetrics,
		api.ErrorToStatus,
		[]grpc.UnaryServerInterceptor{
			ratelimit.UnaryServerInterceptor(rl),
			licenseInterceptor.UnaryServerInterceptor(),
		},
		[]grpc.StreamServerInterceptor{
			ratelimit.StreamServerInterceptor(rl),
			licenseInterceptor.StreamServerInterceptor(),
		},
	)
	serverMetrics.InitializeMetrics(grpcSrv)

	apiSrv := api.New(grpcSrv, healthSrv, tracedCore, log)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.GRPC))
		if err != nil {
			return fmt.Errorf("net.Listen gRPC: %w", err)
		}
		log.Info("starting gRPC server", "addr", lis.Addr().String())

		go func() {
			<-ctx.Done()
			grpcSrv.GracefulStop()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), componentShutdownTimeout)
			defer cancel()
			err := shutdownTelemetry(shutdownCtx)
			if err != nil {
				log.Error("telemetry shutdown error", "error", err)
			}
		}()

		err = grpcSrv.Serve(lis)
		if err != nil {
			return fmt.Errorf("grpcSrv.Serve: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		srv := &http.Server{
			Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.Metric),
			Handler:           mux,
			ReadHeaderTimeout: componentShutdownTimeout,
		}
		log.Info("starting metrics server", "addr", srv.Addr)

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), componentShutdownTimeout)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("metrics server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		healthSrv, err := health.New(health.WithChecks(health.Config{
			Name:    "postgres",
			Timeout: time.Second,
			Check:   repo.Health,
		}))
		if err != nil {
			return fmt.Errorf("health.New: %w", err)
		}

		srv := &http.Server{
			Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.Health),
			Handler:           healthSrv.Handler(),
			ReadHeaderTimeout: componentShutdownTimeout,
		}
		log.Info("starting health server", "addr", srv.Addr)

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), componentShutdownTimeout)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server error: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/mcp", apiSrv.MCPHandler())

		srv := &http.Server{
			Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.MCP),
			Handler:           mux,
			ReadHeaderTimeout: componentShutdownTimeout,
		}
		log.Info("starting mcp server", "addr", srv.Addr, "path", "/mcp")

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), componentShutdownTimeout)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("mcp server error: %w", err)
		}
		return nil
	})

	err = eg.Wait()
	if err != nil {
		return fmt.Errorf("errgroup: %w", err)
	}

	return nil
}

func buildLogger(level slog.Level) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				AddSource: true,
				Level:     level,
			},
		),
	)
}

func forceShutdown(ctx context.Context) {
	log := monitor.FromContext(ctx)
	const shutdownDelay = 15 * time.Second
	<-ctx.Done()
	<-time.After(shutdownDelay)
	log.Error("failed to graceful shutdown")
	os.Exit(exitCode)
}
