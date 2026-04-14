<!-- generated: 2026-04-14, template: development.md -->
# Testing

## 1. Test Package Naming

Tests use the same package as the code under test (internal test package):

```go
package core  // in core/crud_test.go
package api   // in api/api_test.go
```

## 2. Test File Structure

Typical test from the project (`internal/core/crud_test.go`):

```go
package core

import (
    "context"
    "testing"
)

func TestCore_CreatePlugin(t *testing.T) {
    // Setup mock dependencies
    // Create Core instance
    // Call method
    // Assert result
}
```

## 3. Key Patterns

### Table-Driven Tests

Used throughout for parameterized testing:

```go
tests := []struct {
    name    string
    input   SomeInput
    want    SomeOutput
    wantErr error
}{
    {name: "success case", ...},
    {name: "not found", ...},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test body
    })
}
```

### Mocks Defined in Test Files

Mocks are hand-written in test files, not generated:

```go
// Mock implements core.Registry for testing
type mockRegistry struct {
    getFunc    func(...) (Plugin, error)
    listFunc   func(...) ([]PluginInfo, error)
    // ... other methods
}
```

### Test Helpers

Common setup patterns extracted into helper functions within test files.

## 4. Mock Generation

- **No mock generation tool** — all mocks are hand-written
- Mocks live alongside tests in the same `_test.go` file
- Mocks implement core interfaces (`Registry`, `Plugin`, `Metrics`, `FeatureGate`)

## 5. Integration Tests

The registry adapter has integration tests that test SQL migrations and data preservation:

- `internal/adapters/registry/registry_migration_test.go` — migration correctness
- `internal/adapters/registry/registry_preservation_test.go` — data preserved across migrations

These require a running PostgreSQL instance.

## 6. Commands

```bash
# Unit tests
go test ./...

# With race detector
go test -race ./...

# Single package
go test ./internal/core/...

# Verbose
go test -v ./internal/api/...

# Specific test
go test -run TestCore_CreatePlugin ./internal/core/...

# Coverage
go test -cover ./...

# Coverage with report
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# MCP integration test
task test-mcp
```
