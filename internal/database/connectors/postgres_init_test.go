package connectors_test

import "github.com/easyp-tech/service/internal/database/connectors"

var (
	fullPostgresConfig = connectors.PostgresDB{
		User:     "user",
		Password: "password",
		Host:     "127.0.0.1",
		Port:     26257,
		Database: "defaultdb",
		Parameters: &connectors.PostgresDBParameters{
			ApplicationName: "application_name",
			Mode:            connectors.PostgresSSLDisable,
			SSLRootCert:     "path/to/ssl/root",
			SSLCert:         "path/to/ssl/cert",
			SSLKey:          "path/to/ssl/key",
		},
	}
)
