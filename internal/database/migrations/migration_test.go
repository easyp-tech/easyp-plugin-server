package migrations_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/easyp-tech/service/internal/database/migrations"
)

func TestCollectMigrations(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		want    migrations.Migrations
		wantErr error
		fn      func() (migrations.Migrations, error)
	}{
		"success_parse":   {fullMigrations, nil, func() (migrations.Migrations, error) { return migrations.Parse(path) }},
		"success_from_fs": {fullMigrations, nil, func() (migrations.Migrations, error) { return migrations.FromFS(os.DirFS(path), ".") }},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert := require.New(t)
			assert.NotNil(tc.fn)

			res, err := tc.fn()
			assert.ErrorIs(err, tc.wantErr)
			assert.Equal(tc.want, res)
		})
	}
}
