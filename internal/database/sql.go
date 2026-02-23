package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/easyp-tech/service/internal/database/internal"
)

var (
	ErrPanic          = errors.New("panic")
	ErrRollbackFailed = errors.New("rollback failed")
)

// Default values for config.
const (
	DefaultSetConnMaxLifetime    = time.Second * 60
	DefaultSetConnMaxIdleTime    = time.Second * 10
	DefaultSetMaxOpenConnections = 50
	DefaultSetMaxIdleConnections = 50
)

// SQLConfig for set additional properties.
type SQLConfig struct {
	ReturnErrs            []error
	Metrics               MetricCollector
	TracerProvider        trace.TracerProvider
	SetConnMaxLifetime    time.Duration
	SetConnMaxIdleTime    time.Duration
	SetMaxOpenConnections int
	SetMaxIdleConnections int
}

func (c SQLConfig) setDefault() SQLConfig {
	if c.Metrics == nil {
		c.Metrics = NoMetric{}
	}
	if c.SetConnMaxLifetime == 0 {
		c.SetConnMaxLifetime = DefaultSetConnMaxLifetime
	}
	if c.SetConnMaxIdleTime == 0 {
		c.SetConnMaxIdleTime = DefaultSetConnMaxIdleTime
	}
	if c.SetMaxOpenConnections == 0 {
		c.SetMaxOpenConnections = DefaultSetMaxOpenConnections
	}
	if c.SetMaxIdleConnections == 0 {
		c.SetMaxIdleConnections = DefaultSetMaxIdleConnections
	}

	return c
}

// Connector for making connection.
type Connector interface {
	// DSN returns connection string.
	DSN() (string, error)
}

// SQL is a wrapper for sql database.
type SQL struct {
	conn       *sqlx.DB
	returnErrs []error
	metrics    MetricCollector
	tracer     trace.Tracer
}

// NewSQL build and returns new SQL client.
func NewSQL(ctx context.Context, driver string, cfg SQLConfig, connector Connector) (*SQL, error) {
	cfg = cfg.setDefault()

	dsn, err := connector.DSN()
	if err != nil {
		return nil, fmt.Errorf("connector.DSN: %w", err)
	}

	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	err = conn.PingContext(ctx)
	for err != nil {
		nextErr := conn.PingContext(ctx)
		if errors.Is(nextErr, context.DeadlineExceeded) || errors.Is(nextErr, context.Canceled) {
			return nil, fmt.Errorf("db.PingContext: %w", err)
		}
		err = nextErr
	}

	if cfg.TracerProvider == nil {
		cfg.TracerProvider = otel.GetTracerProvider()
	}

	db := &SQL{
		conn:       sqlx.NewDb(conn, driver),
		returnErrs: cfg.ReturnErrs,
		metrics:    cfg.Metrics,
		tracer:     cfg.TracerProvider.Tracer("database"),
	}

	db.conn.SetConnMaxLifetime(cfg.SetConnMaxLifetime)
	db.conn.SetConnMaxIdleTime(cfg.SetConnMaxIdleTime)
	db.conn.SetMaxOpenConns(cfg.SetMaxOpenConnections)
	db.conn.SetMaxIdleConns(cfg.SetMaxIdleConnections)

	return db, nil
}

// UnderlyingDB returns the underlying *sql.DB for use by metrics collectors
// and other components that need direct access to the connection pool stats.
func (db *SQL) UnderlyingDB() *sql.DB {
	return db.conn.DB
}

// Close implements io.Closer.
func (db *SQL) Close() error {
	err := db.conn.Close()
	if err != nil {
		return fmt.Errorf("db.conn.Close: %w", err)
	}

	return nil
}

// NoTx provides DAL method wrapper with:
// - converting sqlx errors which are actually bugs into panics,
// - general metrics for DAL methods,
// - wrapping errors with DAL method name.
func (db *SQL) NoTx(f func(*sqlx.DB) error) (err error) {
	methodName := internal.CallerMethodName(1)

	return db.metrics.Collecting(methodName, func() error {
		err := f(db.conn)
		if err != nil {
			err = fmt.Errorf("%s: %w", methodName, err)
		}

		return err
	})()
}

// Tx provides DAL method wrapper with:
// - converting sqlx errors which are actually bugs into panics,
// - general metrics for DAL methods,
// - wrapping errors with DAL method name,
// - transaction.
func (db *SQL) Tx(ctx context.Context, opts *sql.TxOptions, f func(*sqlx.Tx) error) (err error) {
	methodName := internal.CallerMethodName(1)

	ctx, span := db.tracer.Start(ctx, methodName)
	defer span.End()

	return db.metrics.Collecting(methodName, func() error {
		tx, err := db.conn.BeginTxx(ctx, opts)
		if err == nil { //nolint:nestif // No idea how to simplify.
			defer func() {
				if err := recover(); err != nil {
					errRollback := tx.Rollback()
					if errRollback != nil {
						err = fmt.Errorf("%w: %v: %w", ErrRollbackFailed, err, errRollback)
					}
					span.RecordError(fmt.Errorf("%w: %v", ErrPanic, err))
					span.SetStatus(codes.Error, "panic")
					panic(err)
				}
			}()
			err = f(tx)
			if err == nil {
				err = tx.Commit()
			} else {
				rollbackErr := tx.Rollback()
				if rollbackErr != nil {
					return fmt.Errorf("tx.Rollback: %w", rollbackErr)
				}
			}
		}
		if err != nil {
			err = fmt.Errorf("%s: %w", methodName, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	})()
}

// NoTxContext provides DAL method wrapper with:
// - converting sqlx errors which are actually bugs into panics,
// - general metrics for DAL methods,
// - wrapping errors with DAL method name,
// - tracing.
func (db *SQL) NoTxContext(ctx context.Context, f func(*sqlx.DB) error) (err error) {
	methodName := internal.CallerMethodName(1)

	_, span := db.tracer.Start(ctx, methodName)
	defer span.End()

	return db.metrics.Collecting(methodName, func() error {
		err := f(db.conn)
		if err != nil {
			err = fmt.Errorf("%s: %w", methodName, err)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	})()
}
