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
	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
	"github.com/easyp-tech/service/internal/database/connectors"
	"github.com/easyp-tech/service/internal/database/migrations"
	"github.com/easyp-tech/service/internal/flags"
	"github.com/easyp-tech/service/internal/grpchelper"
	"github.com/easyp-tech/service/internal/license"
	"github.com/easyp-tech/service/internal/monitor"
	"github.com/easyp-tech/service/internal/ratelimiter"
	"github.com/easyp-tech/service/internal/telemetry"
)

const (
	exitCode                 = 2
	configFileSize           = 1024 * 1024
	auditChannelCapacity     = 1000
	componentShutdownTimeout = 5 * time.Second
)

type (
	config struct {
		Server     server           `env:", prefix=SERVER_"      yaml:"server"`
		DB         dbConfig         `env:", prefix=DB_"          yaml:"db"`
		Registry   registryConfig   `env:", prefix=REGISTRY_"    yaml:"registry"`
		Telemetry  telemetryConfig  `env:", prefix=TELEMETRY_"   yaml:"telemetry"`
		WorkerPool workerPoolConfig `env:", prefix=WORKER_POOL_" yaml:"worker_pool"`
		License    licenseConfig    `env:", prefix=LICENSE_"     yaml:"license"`
		RateLimit  rateLimitConfig  `env:", prefix=RATE_LIMIT_"  yaml:"rate_limit"`
	}
	server struct {
		Host string `env:"HOST, default=0.0.0.0" yaml:"host"`
		Port ports  `env:", prefix=PORT_"        yaml:"port"`
	}
	ports struct {
		GRPC   string `env:"GRPC, default=23410"   yaml:"grpc"`
		Metric string `env:"METRIC, default=23411" yaml:"metric"`
		Health string `env:"HEALTH, default=23412" yaml:"health"`
		MCP    string `env:"MCP, default=23413"    yaml:"mcp"`
	}
	dbConfig struct {
		MigrateDir string `env:"MIGRATE_DIR, default=migrate" yaml:"migrate_dir"`
		Driver     string `env:"DRIVER, default=postgres"     yaml:"driver"`
		Postgres   string `env:"POSTGRES_DSN"                 yaml:"postgres"`
	}
	registryConfig struct {
		Domain string `env:"DOMAIN, default=localhost:5005" yaml:"domain"`
	}
	telemetryConfig struct {
		OTLPEndpoint      string `env:"OTEL_EXPORTER_OTLP_ENDPOINT, default=localhost:4317" yaml:"otlp_endpoint"`
		PyroscopeEndpoint string `env:"PYROSCOPE_ENDPOINT, default=http://localhost:4040"   yaml:"pyroscope_endpoint"`
	}
	workerPoolConfig struct {
		Workers           int           `env:"WORKERS,default=4"               yaml:"workers"`
		QueueSize         int           `env:"QUEUE_SIZE,default=16"           yaml:"queue_size"`
		GenerationTimeout time.Duration `env:"GENERATION_TIMEOUT,default=120s" yaml:"generation_timeout"`
		MaxRetries        int           `env:"MAX_RETRIES,default=2"           yaml:"max_retries"`
		ShutdownTimeout   time.Duration `env:"SHUTDOWN_TIMEOUT,default=30s"    yaml:"shutdown_timeout"`
	}
	licenseConfig struct {
		Key  string `env:"KEY"  yaml:"key"`
		File string `env:"FILE" yaml:"file"`
	}
	rateLimitConfig struct {
		RequestsPerSecond float64       `env:"REQUESTS_PER_SECOND,default=10.0" yaml:"requests_per_second"`
		Burst             int           `env:"BURST,default=20"                 yaml:"burst"`
		CleanupInterval   time.Duration `env:"CLEANUP_INTERVAL,default=10m"     yaml:"cleanup_interval"`
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

func (m *grpcMetrics) PanicsTotal() prometheus.Counter { //nolint:ireturn // interface requires prometheus.Counter
	return m.panics
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

	err := start(ctx, cfgFile, appName)
	if err != nil {
		log.Error("shutdown", "error", err)
		os.Exit(exitCode) //nolint:gocritic // forceShutdown is running and defer cancel() is intentionally bypassed
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
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(collectors.NewGoCollector())

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

	repo, err := registry.New(ctx, db, cfg.Registry.Domain)
	if err != nil {
		return fmt.Errorf("registry.New: %w", err)
	}

	defer func() {
		err := repo.Close()
		if err != nil {
			log.Error("close database connection", "error", err)
		}
	}()

	// Register DB connection pool metrics collector
	dbCollector := adapter_metrics.NewDBCollector(repo.DB().UnderlyingDB(), namespace)
	reg.MustRegister(dbCollector)

	// Register business metrics collector
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

	// Create LicenseManager
	lm, err := license.NewManager(licensePublicKey, license.Config{
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
	tracedRegistry := telemetry.NewTracingRegistry(repo)

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
	module := core.New(metricsAdapter, pool, gate, auditCh, log)

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

	// Create license interceptor
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
		},
		[]grpc.StreamServerInterceptor{
			ratelimit.StreamServerInterceptor(rl),
			licenseInterceptor.StreamServerInterceptor(),
		},
	)
	serverMetrics.InitializeMetrics(grpcSrv)

	// Register API handlers
	apiSrv := api.New(grpcSrv, healthSrv, tracedCore, log)

	eg, ctx := errgroup.WithContext(ctx)

	// Run gRPC Server
	eg.Go(func() error {
		lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port.GRPC))
		if err != nil {
			return fmt.Errorf("net.Listen gRPC: %w", err)
		}
		log.Info("starting gRPC server", "addr", lis.Addr().String())

		go func() {
			<-ctx.Done()
			// Shutdown order: 1. Stop accepting new gRPC requests
			grpcSrv.GracefulStop()
			// 2. Shutdown telemetry (flush spans/metrics)
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

	// Run Metrics Server
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

	// Run Health Server
	eg.Go(func() error {
		// Simple health check handler
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

	// Run MCP Server over streamable HTTP transport.
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

	// Create a new context for the shutdown delay/logging since the parent ctx is done
	// But actually we just want to sleep.

	<-time.After(shutdownDelay)
	log.Error("failed to graceful shutdown")
	os.Exit(exitCode)
}
