package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/ratelimit"
	"github.com/hellofresh/health-go/v5"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sethvargo/go-envconfig"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	adapter_audit "github.com/easyp-tech/service/internal/adapters/audit"
	adapter_metrics "github.com/easyp-tech/service/internal/adapters/metrics"
	"github.com/easyp-tech/service/internal/adapters/registry"
	"github.com/easyp-tech/service/internal/api"
	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
	"github.com/easyp-tech/service/internal/database/connectors"
	"github.com/easyp-tech/service/internal/database/migrations"
	"github.com/easyp-tech/service/internal/flags"
	"github.com/easyp-tech/service/internal/grpchelper"
	"github.com/easyp-tech/service/internal/license"
	"github.com/easyp-tech/service/internal/mcpserver"
	"github.com/easyp-tech/service/internal/monitor"
	"github.com/easyp-tech/service/internal/ratelimiter"
	"github.com/easyp-tech/service/internal/telemetry"
)

const (
	exitCode       = 2
	configFileSize = 1024 * 1024
)

type (
	config struct {
		Server     server           `yaml:"server" env:", prefix=SERVER_"`
		DB         dbConfig         `yaml:"db" env:", prefix=DB_"`
		Registry   registryConfig   `yaml:"registry" env:", prefix=REGISTRY_"`
		Telemetry  telemetryConfig  `yaml:"telemetry" env:", prefix=TELEMETRY_"`
		WorkerPool workerPoolConfig `yaml:"worker_pool" env:", prefix=WORKER_POOL_"`
		License    licenseConfig    `yaml:"license" env:", prefix=LICENSE_"`
		RateLimit  rateLimitConfig  `yaml:"rate_limit" env:", prefix=RATE_LIMIT_"`
	}
	server struct {
		Host string `yaml:"host" env:"HOST, default=0.0.0.0"`
		Port ports  `yaml:"port" env:", prefix=PORT_"`
	}
	ports struct {
		GRPC   string `yaml:"grpc" env:"GRPC, default=23410"`
		Metric string `yaml:"metric" env:"METRIC, default=23411"`
		Health string `yaml:"health" env:"HEALTH, default=23412"`
		MCP    string `yaml:"mcp" env:"MCP, default=23413"`
	}
	dbConfig struct {
		MigrateDir string `yaml:"migrate_dir" env:"MIGRATE_DIR, default=migrate"`
		Driver     string `yaml:"driver" env:"DRIVER, default=postgres"`
		Postgres   string `yaml:"postgres" env:"POSTGRES_DSN"`
	}
	registryConfig struct {
		Domain string `yaml:"domain" env:"DOMAIN, default=localhost:5005"`
	}
	telemetryConfig struct {
		OTLPEndpoint      string `yaml:"otlp_endpoint" env:"OTEL_EXPORTER_OTLP_ENDPOINT, default=localhost:4317"`
		PyroscopeEndpoint string `yaml:"pyroscope_endpoint" env:"PYROSCOPE_ENDPOINT, default=http://localhost:4040"`
	}
	workerPoolConfig struct {
		Workers           int           `yaml:"workers" env:"WORKERS,default=4"`
		QueueSize         int           `yaml:"queue_size" env:"QUEUE_SIZE,default=16"`
		GenerationTimeout time.Duration `yaml:"generation_timeout" env:"GENERATION_TIMEOUT,default=120s"`
		MaxRetries        int           `yaml:"max_retries" env:"MAX_RETRIES,default=2"`
		ShutdownTimeout   time.Duration `yaml:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT,default=30s"`
	}
	licenseConfig struct {
		Key  string `yaml:"key" env:"KEY"`
		File string `yaml:"file" env:"FILE"`
	}
	rateLimitConfig struct {
		RequestsPerSecond float64       `yaml:"requests_per_second" env:"REQUESTS_PER_SECOND,default=10.0"`
		Burst             int           `yaml:"burst" env:"BURST,default=20"`
		CleanupInterval   time.Duration `yaml:"cleanup_interval" env:"CLEANUP_INTERVAL,default=10m"`
	}
)

var (
	cfgFile  = &flags.File{DefaultPath: "", MaxSize: configFileSize}
	logLevel = &flags.Level{Level: slog.LevelDebug}

	licensePublicKey string // set via -ldflags "-X main.licensePublicKey=..."
)

// grpcMetrics implements grpchelper.Metrics for the recovery interceptor.
type grpcMetrics struct {
	panics prometheus.Counter
}

func (m *grpcMetrics) PanicsTotal() prometheus.Counter {
	return m.panics
}

// featureGateAdapter adapts license.FeatureGate to core.FeatureGate interface.
type featureGateAdapter struct {
	gate *license.FeatureGate
}

func (a *featureGateAdapter) Enabled(feature int) bool {
	return a.gate.Enabled(license.Feature(feature))
}

func (a *featureGateAdapter) MaxWorkers() int {
	return a.gate.MaxWorkers()
}

func (a *featureGateAdapter) MaxPlugins() int {
	return a.gate.MaxPlugins()
}

func main() {
	flag.Var(cfgFile, "cfg", "path to config file")
	flag.Var(logLevel, "log_level", "log level")
	flag.Parse()

	log := buildLogger(logLevel.Level)

	appName := filepath.Base(os.Args[0])
	ctxParent := monitor.WithContext(context.Background(), log.With(
		slog.String("version", "dev"), // Replace with actual version logic if needed
		slog.String("app", appName),
	))

	ctx, cancel := signal.NotifyContext(ctxParent, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	defer cancel()

	go forceShutdown(ctx)

	if err := start(ctx, cfgFile, appName); err != nil {
		log.Error("shutdown", "error", err)
		os.Exit(exitCode)
	}
}

func start(ctx context.Context, cfgFile *flags.File, appName string) error {
	cfg := config{}

	if !cfgFile.IsNil() {
		err := yaml.NewDecoder(cfgFile).Decode(&cfg)
		if err != nil {
			return fmt.Errorf("yaml.NewDecoder.Decode: %w", err)
		}
	} else {
		err := envconfig.Process(ctx, &cfg)
		if err != nil {
			return fmt.Errorf("envconfig.Process: %w", err)
		}
	}

	reg := prometheus.NewRegistry() // Use standard registry
	// Add Go metrics
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	reg.MustRegister(prometheus.NewGoCollector())

	return run(ctx, cfg, reg, appName)
}

func run(ctx context.Context, cfg config, reg *prometheus.Registry, namespace string) error {
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
	// Use telemetry-enriched logger
	log = telLog
	// Update context with new logger
	ctx = monitor.WithContext(ctx, log)

	// Parse and run migrations before creating the database connection
	migrates, err := migrations.Parse(cfg.DB.MigrateDir)
	if err != nil {
		return fmt.Errorf("migrations.Parse: %w", err)
	}

	err = migrations.Run(ctx, cfg.DB.Driver, &connectors.Raw{Query: cfg.DB.Postgres}, migrations.Up, migrates)
	if err != nil {
		return fmt.Errorf("migrations.Run: %w", err)
	}

	// Create DAL metrics for database operations
	dalMetrics := database.NewMetrics(reg, namespace, "repo", new(core.Registry))

	// Create database connection via database.NewSQL
	sqlCfg := database.SQLConfig{Metrics: dalMetrics}
	db, err := database.NewSQL(ctx, cfg.DB.Driver, sqlCfg, &connectors.Raw{Query: cfg.DB.Postgres})
	if err != nil {
		return fmt.Errorf("database.NewSQL: %w", err)
	}

	r, err := registry.New(ctx, db, cfg.Registry.Domain)
	if err != nil {
		return fmt.Errorf("registry.New: %w", err)
	}

	defer func() {
		if err := r.Close(); err != nil {
			log.Error("close database connection", "error", err)
		}
	}()

	// Register DB connection pool metrics collector
	dbCollector := adapter_metrics.NewDBCollector(r.DB().UnderlyingDB(), namespace)
	reg.MustRegister(dbCollector)

	// Register business metrics collector
	businessCollector := adapter_metrics.NewBusinessMetricsCollector(r.DB().UnderlyingDB(), namespace, log)
	reg.MustRegister(businessCollector)

	auditStore := adapter_audit.New(r.DB(), log)
	auditWorker, auditCh := adapter_audit.NewWorker(auditStore, 1000, log, reg, namespace)

	go auditWorker.Run(ctx)

	defer func() {
		lost := auditWorker.Shutdown(5 * time.Second)
		if lost > 0 {
			log.Warn("audit events lost on shutdown", "count", lost)
		}
	}()

	// Create LicenseManager
	lm, err := license.NewLicenseManager(licensePublicKey, license.LicenseConfig{
		Key:  cfg.License.Key,
		File: cfg.License.File,
	}, log, reg, namespace)
	if err != nil {
		log.Warn("license initialization error, continuing in community mode", "error", err)
	}
	lm.StartExpirationWatcher(ctx)
	defer lm.Stop()

	// Create FeatureGate
	gate := license.NewFeatureGate(lm)

	// Create RateLimiter
	rl := ratelimiter.New(ratelimiter.Config{
		RequestsPerSecond: cfg.RateLimit.RequestsPerSecond,
		Burst:             cfg.RateLimit.Burst,
		CleanupInterval:   cfg.RateLimit.CleanupInterval,
	}, gate, nil, log, reg) // nil keyExtractor → PeerIPExtractor
	rl.StartCleanup(ctx)

	// Wrap Registry in tracing decorator
	tracedRegistry := telemetry.NewTracingRegistry(r)

	// Override WorkerPool workers from license limits
	wpWorkers := cfg.WorkerPool.Workers
	if licenseWorkers := gate.MaxWorkers(); licenseWorkers > 0 {
		wpWorkers = licenseWorkers
	}

	// Wrap TracingRegistry in WorkerPool (limit Docker parallelism)
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

	// Core gets pool as Registry
	module := core.New(metricsAdapter, pool, &featureGateAdapter{gate: gate})

	// Wrap Core in tracing decorator, pass to API
	tracedCore := telemetry.NewTracingCore(module)

	// Create gRPC server metrics
	serverMetrics := grpchelper.NewServerMetrics(reg, "easyp", "api")

	// Create panics counter
	panicsCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "easyp",
		Name:      "panics_total",
		Help:      "Total number of panics recovered in gRPC handlers.",
	})
	reg.MustRegister(panicsCounter)

	// Create audit interceptor
	auditInterceptor := api.NewAuditInterceptor(auditCh, log)

	// Create license interceptor (before audit in the chain)
	licenseInterceptor := api.NewLicenseInterceptor(gate, log)

	// Create gRPC server with full middleware stack
	grpcSrv, healthSrv := grpchelper.NewServer(
		&grpcMetrics{panics: panicsCounter},
		log,
		serverMetrics,
		api.ErrorToStatus,
		[]grpc.UnaryServerInterceptor{
			ratelimit.UnaryServerInterceptor(rl),
			licenseInterceptor.UnaryServerInterceptor(),
			auditInterceptor.UnaryServerInterceptor(),
		},
		[]grpc.StreamServerInterceptor{
			ratelimit.StreamServerInterceptor(rl),
			licenseInterceptor.StreamServerInterceptor(),
		},
	)
	serverMetrics.InitializeMetrics(grpcSrv)

	// Register API handlers
	api.New(grpcSrv, healthSrv, tracedCore)

	mcpSrv := mcpserver.New(tracedCore, log)

	g, ctx := errgroup.WithContext(ctx)

	// Run gRPC Server
	g.Go(func() error {
		lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.GRPC))
		if err != nil {
			return fmt.Errorf("net.Listen gRPC: %w", err)
		}
		log.Info("starting gRPC server", "addr", lis.Addr().String())

		go func() {
			<-ctx.Done()
			// Shutdown order: 1. Stop accepting new gRPC requests
			grpcSrv.GracefulStop()
			// 2. Shutdown telemetry (flush spans/metrics)
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdownTelemetry(shutdownCtx); err != nil {
				log.Error("telemetry shutdown error", "error", err)
			}
		}()

		if err := grpcSrv.Serve(lis); err != nil {
			return fmt.Errorf("grpcSrv.Serve: %w", err)
		}
		return nil
	})

	// Run Metrics Server
	g.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.Metric),
			Handler: mux,
		}

		log.Info("starting metrics server", "addr", srv.Addr)

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("metrics server error: %w", err)
		}
		return nil
	})

	// Run Health Server
	g.Go(func() error {
		// Simple health check handler
		h, err := health.New(health.WithChecks(health.Config{
			Name:    "postgres",
			Timeout: time.Second,
			Check:   r.Health,
		}))
		if err != nil {
			return fmt.Errorf("health.New: %w", err)
		}

		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.Health),
			Handler: h.Handler(),
		}

		log.Info("starting health server", "addr", srv.Addr)

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("health server error: %w", err)
		}
		return nil
	})

	// Run MCP Server over streamable HTTP transport.
	g.Go(func() error {
		mux := http.NewServeMux()
		mux.Handle("/mcp", mcpSrv.Handler())

		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.MCP),
			Handler: mux,
		}

		log.Info("starting mcp server", "addr", srv.Addr, "path", "/mcp")

		go func() {
			<-ctx.Done()
			ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctxShutdown)
		}()

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("mcp server error: %w", err)
		}
		return nil
	})

	return g.Wait()
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

	// Create a new context for the shutdown delay/logging since the parent ctx is done
	// But actually we just want to sleep.

	select {
	case <-time.After(shutdownDelay):
		log.Error("failed to graceful shutdown")
		os.Exit(exitCode)
	}
}
