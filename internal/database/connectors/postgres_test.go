package connectors_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/easyp-tech/service/internal/database/connectors"
)

func TestPostgresDB_Unmarshal(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path    string
		decoder func([]byte, any) error
	}{
		"json": {"testdata/postgres_db.json", json.Unmarshal},
		"yaml": {"testdata/postgres_db.yaml", yaml.Unmarshal},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)

			b, err := os.ReadFile(tc.path)
			r.NoError(err)
			value := connectors.PostgresDB{}
			err = tc.decoder(b, &value)
			r.NoError(err)
			r.Equal(fullPostgresConfig, value)
		})
	}
}

func TestPostgresDB_DSN(t *testing.T) {
	t.Parallel()

	type T = connectors.PostgresDB
	change := func(t T, fn func(*T)) T {
		var parameters *connectors.PostgresDBParameters
		if t.Parameters != nil {
			p := *t.Parameters
			parameters = &p
		}

		t.Parameters = parameters

		fn(&t)

		return t
	}

	var (
		allDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLKey       = change(fullPostgresConfig, func(t *T) { t.Parameters.SSLKey = "" })
		withoutParametersSSLKeyDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLCert       = change(fullPostgresConfig, func(t *T) { t.Parameters.SSLCert = "" })
		withoutParametersSSLCertDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLRoot       = change(fullPostgresConfig, func(t *T) { t.Parameters.SSLRootCert = "" })
		withoutParametersSSLRootDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable"

		withoutParametersSSLMod       = change(fullPostgresConfig, func(t *T) { t.Parameters.Mode = 0 })
		withoutParametersSSLModDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersApplicationName       = change(fullPostgresConfig, func(t *T) { t.Parameters.ApplicationName = "" })
		withoutParametersApplicationNameDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParameters       = change(fullPostgresConfig, func(t *T) { t.Parameters = nil })
		withoutParametersDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb"
	)

	testCases := map[string]struct {
		cfg T
		exp string
	}{
		"all":                                 {fullPostgresConfig, allDSNExp},
		"without_parameters_ssl_key":          {withoutParametersSSLKey, withoutParametersSSLKeyDSNExp},
		"without_parameters_ssl_cert":         {withoutParametersSSLCert, withoutParametersSSLCertDSNExp},
		"without_parameters_ssl_root":         {withoutParametersSSLRoot, withoutParametersSSLRootDSNExp},
		"without_parameters_ssl_mod":          {withoutParametersSSLMod, withoutParametersSSLModDSNExp},
		"without_parameters_application_name": {withoutParametersApplicationName, withoutParametersApplicationNameDSNExp},
		"without_parameters":                  {withoutParameters, withoutParametersDSNExp},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)

			dsn, err := tc.cfg.DSN()
			r.NoError(err)
			r.Equal(tc.exp, dsn)
		})
	}
}
