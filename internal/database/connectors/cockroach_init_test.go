package connectors_test

import "github.com/easyp-tech/service/internal/database/connectors"

var fullCockroachConfig = connectors.CockroachDB{
	User:     "user",
	Password: "password",
	Host:     "127.0.0.1",
	Port:     26257,
	Database: "defaultdb",
	Parameters: &connectors.CockroachDBParameters{
		ApplicationName: "application_name",
		Mode:            connectors.CockroachSSLDisable,
		SSLRootCert:     "path/to/ssl/root",
		SSLCert:         "path/to/ssl/cert",
		SSLKey:          "path/to/ssl/key",
		Options: &connectors.CockroachDBOptions{
			Cluster: "cluster_id",
			Variable: connectors.CockroachDBVariable{
				Name:  "name",
				Value: "value",
			},
		},
	},
}
