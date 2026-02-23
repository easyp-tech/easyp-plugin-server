package connectors_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/easyp-tech/service/internal/database/connectors"
)

func TestRaw_Unmarshal(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		path    string
		decoder func([]byte, interface{}) error
	}{
		"json": {"testdata/raw_db.json", func(b []byte, i interface{}) error { return json.Unmarshal(b, i) }},
		"yaml": {"testdata/raw_db.yaml", func(b []byte, i interface{}) error { return yaml.Unmarshal(b, i) }},
	}

	for name, tc := range testCases {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)

			b, err := os.ReadFile(tc.path)
			r.NoError(err)
			value := connectors.Raw{}
			err = tc.decoder(b, &value)
			r.NoError(err)
			r.Equal(fullRawConfig, value)
		})
	}
}

func TestRaw_DSN(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		cfg connectors.Raw
		exp string
	}{
		"raw": {fullRawConfig, "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name"},
	}

	for name, tc := range testCases {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)

			dsn, err := tc.cfg.DSN()
			r.NoError(err)
			r.Equal(tc.exp, dsn)
		})
	}
}
