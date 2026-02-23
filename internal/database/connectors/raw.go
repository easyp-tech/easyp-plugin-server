package connectors

import (
	"github.com/easyp-tech/service/internal/database"
)

var _ database.Connector = (*Raw)(nil)

// Raw config for connecting to cockroachDB.
type Raw struct {
	Query string `json:"dsn" yaml:"dsn"`
}

// DSN convert struct to DSN and returns connection string.
func (r *Raw) DSN() (string, error) {
	return r.Query, nil
}
