package migrations

import "errors"

// Errors.
var (
	ErrInvalidMigrationExt  = errors.New("invalid migration ext")
	ErrInvalidMigrationName = errors.New("invalid migration name")
)
