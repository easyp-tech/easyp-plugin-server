// Package audit provides an audit log adapter for PostgreSQL.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	"github.com/easyp-tech/service/internal/core"
	"github.com/easyp-tech/service/internal/database"
)

var _ core.AuditLog = &Store{}

// Store реализует core.AuditLog для PostgreSQL.
type Store struct {
	db     *database.SQL
	logger *slog.Logger
}

// New создаёт новый Store.
func New(db *database.SQL, logger *slog.Logger) *Store {
	return &Store{
		db:     db,
		logger: logger,
	}
}

// Save реализует core.AuditLog.
func (s *Store) Save(ctx context.Context, entry core.AuditEntry) error {
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		s.logger.Error("audit metadata marshal failed", "error", err)
		metadata = []byte("{}")
	}

	const query = `INSERT INTO audit_log (
	id, operation_type, plugin_name, caller_address, status,
	error_code, error_message, duration_ms, metadata, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	err = s.db.NoTxContext(ctx, func(db *sqlx.DB) error {
		_, execErr := db.ExecContext(ctx, query,
			entry.ID,
			entry.OperationType,
			entry.PluginName,
			entry.CallerAddress,
			entry.Status,
			entry.ErrorCode,
			entry.ErrorMessage,
			entry.DurationMs,
			metadata,
			entry.CreatedAt,
		)

		if execErr != nil {
			return fmt.Errorf("db.ExecContext: %w", execErr)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("s.db.NoTxContext: %w", err)
	}

	return nil
}
