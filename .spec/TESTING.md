<!-- generated: 2026-05-24, template: development.md -->
# Testing

Project testing conventions for EasyP Service.

## 1. Test Package Naming

Tests use the **same package** (internal tests) — not `_test` suffix packages. This allows access to unexported symbols.

```go
// internal/core/crud_test.go
package core  // same package as the code being tested
```

## 2. Test File Structure

Typical test from the project (`internal/core/crud_test.go`):

```go
package core

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCreatePlugin(t *testing.T) {
    tests := []struct {
        name    string
        req     CreatePluginRequest
        setup   func(*mockRegistry, *mockFeatureGate)
        wantErr error
    }{
        {
            name: "success",
            req: CreatePluginRequest{
                Group:   "grpc",
                Name:    "go",
                Version: "v1.5.1",
            },
            setup: func(r *mockRegistry, fg *mockFeatureGate) {
                fg.enabled = true
                fg.maxPlugins = -1
                r.createResult = &PluginInfo{...}
            },
        },
        {
            name:    "feature denied",
            req:     CreatePluginRequest{Group: "grpc", Name: "go", Version: "v1.0.0"},
            setup:   func(r *mockRegistry, fg *mockFeatureGate) { fg.enabled = false },
            wantErr: ErrFeatureDenied,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // setup mocks
            reg := &mockRegistry{}
            fg := &mockFeatureGate{}
            tt.setup(reg, fg)

            c := New(nil, reg, fg, make(chan<- AuditEntry, 100), slog.Default())
            _, err := c.CreatePlugin(context.Background(), tt.req)

            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

## 3. Key Patterns

### Table-Driven Tests (slice of structs)

All tests use slice-of-structs pattern with `t.Run`:

```go
tests := []struct {
    name    string
    // inputs
    wantErr error
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test body
    })
}
```

### Parallel Tests

All table-driven tests MUST use `t.Parallel()` at two levels:

```go
func TestXxx(t *testing.T) {
    t.Parallel() // top-level: run alongside other Test* functions

    tests := []struct {
        name string
        // ...
    }{...}

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // sub-test: run cases concurrently

            // Create fresh dependencies per sub-test (no shared mutable state)
            // ...
        })
    }
}
```

**Rules:**
- Every table-driven test has `t.Parallel()` at top-level and inside each `t.Run`
- Each sub-test creates its own mocks/dependencies (no shared mutable state)
- If a test cannot be parallel, it indicates shared state — refactor the code or test
- Enforced by `paralleltest` and `tparallel` linters

### Inline Mocks

Mocks are defined as simple structs in test files — no code generation:

```go
type mockRegistry struct {
    getResult    Plugin
    getErr       error
    listResult   []PluginInfo
    createResult *PluginInfo
    // ...
}

func (m *mockRegistry) Get(ctx context.Context, group, name, version string) (Plugin, error) {
    return m.getResult, m.getErr
}
```

### Assertion Library

Uses `testify` (`assert` and `require`):

```go
require.NoError(t, err)
require.ErrorIs(t, err, ErrNotFound)
assert.Equal(t, expected, actual)
assert.Len(t, plugins, 3)
```

## 4. Mock Generation

**No generated mocks.** All mocks are hand-written structs in test files.

Pattern: mock struct with fields for return values, methods return those fields.

Location: same `_test.go` file that uses them.

## 5. Integration Tests

No separate integration test infrastructure. The project uses:
- `docker-compose` for full stack testing
- `task smoke-mcp` for MCP end-to-end smoke tests
- Standard `go test` for unit tests

## 6. Relaxed Linter Rules in Tests

Tests have relaxed linter rules (`.golangci.yml`):
- Disabled: `dupl`, `errcheck`, `funlen`, `gocyclo`, `goconst`, `gosec`, `lll`, `varnamelen`, `testpackage`, `paralleltest`, `err113`
- This allows more verbose test code without linter noise

## 7. Commands

```bash
# Unit tests
go test ./...

# With race detector
go test -race ./...

# Specific package
go test ./internal/core/...

# With verbose output
go test -v ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# MCP server tests
task test-mcp

# MCP smoke test (requires running service)
task smoke-mcp
```
