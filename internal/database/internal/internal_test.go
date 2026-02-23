//nolint:testpackage // Testing unexported function.
package internal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypeMethodName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		given     string
		want      string
		wantPanic bool
	}{
		{"", "", true},
		{"bad", "", true},
		{"main.main", "", true},
		{"main.f", "", true},
		{"main.f.func1", "f.func1", false},
		{"main.f.func2", "f.func2", false},
		{"main.f.func2.1", "f.func2", false},
		{"main.f.func2.1.1", "f.func2", false},
		{"main.f.func3", "f.func3", false},
		{"main.T.m", "T.m", false},
		{"main.T.m.func1", "T.m", false},
		{"main.T.m.func2", "T.m", false},
		{"main.T.m.func2.1", "T.m", false},
		{"github.com/powerman/whoami/subpkg.F", "", true},
		{"github.com/powerman/whoami/subpkg.F.func1", "F.func1", false},
		{"github.com/powerman/whoami/subpkg.F.func2", "F.func2", false},
		{"github.com/powerman/whoami/subpkg.F.func2.1", "F.func2", false},
		{"github.com/powerman/whoami/subpkg.(*T).M", "T.M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func1", "T.M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func2", "T.M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func2.1", "T.M", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.given, func(tt *testing.T) {
			r := require.New(t)

			if tc.wantPanic {
				r.Panics(func() { typeMethodName(tc.given) })
			} else {
				r.Equal(stripTypeRef(typeMethodName(tc.given)), tc.want)
			}
		})
	}
}

func TestMethodName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		given     string
		want      string
		wantPanic bool
	}{
		{"", "", true},
		{"bad", "", true},
		{"main.main", "", true},
		{"main.f", "", true},
		{"main.f.func1", "func1", false},
		{"main.f.func2", "func2", false},
		{"main.f.func2.1", "func2", false},
		{"main.f.func2.1.1", "func2", false},
		{"main.f.func3", "func3", false},
		{"main.T.m", "m", false},
		{"main.T.m.func1", "m", false},
		{"main.T.m.func2", "m", false},
		{"main.T.m.func2.1", "m", false},
		{"github.com/powerman/whoami/subpkg.F", "", true},
		{"github.com/powerman/whoami/subpkg.F.func1", "func1", false},
		{"github.com/powerman/whoami/subpkg.F.func2", "func2", false},
		{"github.com/powerman/whoami/subpkg.F.func2.1", "func2", false},
		{"github.com/powerman/whoami/subpkg.(*T).M", "M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func1", "M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func2", "M", false},
		{"github.com/powerman/whoami/subpkg.(*T).M.func2.1", "M", false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.given, func(tt *testing.T) {
			r := require.New(t)

			if tc.wantPanic {
				r.Panics(func() { methodName(tc.given) })
			} else {
				r.Equal(methodName(tc.given), tc.want)
			}
		})
	}
}

func stripTypeRef(name string) string {
	if name[0] == '(' {
		pos := strings.IndexByte(name, ')')
		name = name[2:pos] + name[pos+1:]
	}
	return name
}
