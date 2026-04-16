package connectors_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/easyp-tech/service/internal/database/connectors"
)

func TestCockroachDB_Unmarshal(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path    string
		decoder func([]byte, any) error
	}{
		"json": {"testdata/cockroach_db.json", json.Unmarshal},
		"yaml": {"testdata/cockroach_db.yaml", yaml.Unmarshal},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)

			b, err := os.ReadFile(tc.path)
			r.NoError(err)
			value := connectors.CockroachDB{}
			err = tc.decoder(b, &value)
			r.NoError(err)
			r.Equal(fullCockroachConfig, value)
		})
	}
}

func TestCockroachDB_DSN(t *testing.T) {
	t.Parallel()

	type T = connectors.CockroachDB
	change := func(t T, fn func(*T)) T {
		var parameters *connectors.CockroachDBParameters
		if t.Parameters != nil {
			p := *t.Parameters
			parameters = &p
		}

		if parameters != nil && parameters.Options != nil {
			op := *t.Parameters.Options
			parameters.Options = &op
		}
		t.Parameters = parameters

		fn(&t)

		return t
	}

	var (
		allDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&options=--cluster%3Dcluster_id+-c+name%3Dvalue&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersOptionsVariable       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options.Variable = connectors.CockroachDBVariable{} })
		withoutParametersOptionsVariableDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&options=--cluster%3Dcluster_id&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersOptionsCluster       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options.Cluster = "" })
		withoutParametersOptionsClusterDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&options=-c+name%3Dvalue&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersOptions       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil })
		withoutParametersOptionsDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLKey       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil; t.Parameters.SSLKey = "" })
		withoutParametersSSLKeyDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLCert       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil; t.Parameters.SSLCert = "" })
		withoutParametersSSLCertDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersSSLRoot       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil; t.Parameters.SSLRootCert = "" })
		withoutParametersSSLRootDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable"

		withoutParametersSSLMod       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil; t.Parameters.Mode = 0 })
		withoutParametersSSLModDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name&sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParametersApplicationName       = change(fullCockroachConfig, func(t *T) { t.Parameters.Options = nil; t.Parameters.ApplicationName = "" })
		withoutParametersApplicationNameDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb?sslcert=path%2Fto%2Fssl%2Fcert&sslkey=path%2Fto%2Fssl%2Fkey&sslmode=disable&sslrootcert=path%2Fto%2Fssl%2Froot"

		withoutParameters       = change(fullCockroachConfig, func(t *T) { t.Parameters = nil })
		withoutParametersDSNExp = "postgres://user:password@127.0.0.1:26257/defaultdb"
	)

	testCases := map[string]struct {
		cfg T
		exp string
	}{
		"all":                                 {fullCockroachConfig, allDSNExp},
		"without_parameters_options_variable": {withoutParametersOptionsVariable, withoutParametersOptionsVariableDSNExp},
		"without_parameters_options_cluster":  {withoutParametersOptionsCluster, withoutParametersOptionsClusterDSNExp},
		"without_parameters_options":          {withoutParametersOptions, withoutParametersOptionsDSNExp},
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
