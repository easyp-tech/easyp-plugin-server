package registry

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/pluginpb"
)

var mockPluginPath string

func TestMain(m *testing.M) {
	flag.Parse()

	// Compile mock plugin binary
	tmpDir, err := os.MkdirTemp("", "easyp-mock-plugin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	binaryPath := filepath.Join(tmpDir, "mock_plugin")
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, "./testdata/mock_plugin.go")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		fmt.Fprintf(os.Stderr, "failed to build mock plugin: %v\n", err)
		os.Exit(1)
	}

	mockPluginPath = binaryPath

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// TestPluginConfig_RoundTrip validates JSON marshalling and unmarshalling of PluginConfig.
func TestPluginConfig_RoundTrip(t *testing.T) {
	cfg := PluginConfig{
		Command: []string{"/plugins/some/plugin", "--arg"},
		Env:     map[string]string{"KEY": "VAL"},
		Timeout: "10s",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	var parsed PluginConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if len(parsed.Command) != 2 || parsed.Command[0] != "/plugins/some/plugin" || parsed.Command[1] != "--arg" {
		t.Errorf("unexpected command: %v", parsed.Command)
	}
	if parsed.Env["KEY"] != "VAL" {
		t.Errorf("unexpected env: %v", parsed.Env)
	}
	if parsed.Timeout != "10s" {
		t.Errorf("unexpected timeout: %s", parsed.Timeout)
	}
}

// TestValidateConfig validates configurations against path traversal and other requirements.
func TestValidateConfig(t *testing.T) {
	pluginsDir := "/plugins"

	tests := []struct {
		name    string
		config  string
		wantErr bool
		errSub  string
	}{
		{
			name:    "valid direct command",
			config:  `{"command": ["/plugins/protocolbuffers/go/v1.36.10/plugin"]}`,
			wantErr: false,
		},
		{
			name:    "valid command with interpreter",
			config:  `{"command": ["python3", "/plugins/mygroup/myplugin/v1.0.0/plugin.py"]}`,
			wantErr: false,
		},
		{
			name:    "empty command array",
			config:  `{"command": []}`,
			wantErr: true,
			errSub:  "empty command",
		},
		{
			name:    "no element in plugins dir",
			config:  `{"command": ["python3", "main.py"]}`,
			wantErr: true,
			errSub:  "must contain at least one path inside plugins directory",
		},
		{
			name:    "path traversal outside plugins dir via relativity",
			config:  `{"command": ["/plugins/../bin/sh"]}`,
			wantErr: true,
			errSub:  "outside plugins directory",
		},
		{
			name:    "path traversal outside plugins dir via dotdot in argument",
			config:  `{"command": ["python3", "/plugins/some/../../bin/sh"]}`,
			wantErr: true,
			errSub:  "outside plugins directory",
		},
		{
			name:    "old docker format config",
			config:  `{"docker": {"network": "none"}}`,
			wantErr: true,
			errSub:  "old format config",
		},
		{
			name:    "invalid json",
			config:  `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(json.RawMessage(tt.config), pluginsDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil && tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("ValidateConfig() error %q does not contain %q", err.Error(), tt.errSub)
			}
		})
	}
}

func getMockRequest() *pluginpb.CodeGeneratorRequest {
	param := "test"

	return &pluginpb.CodeGeneratorRequest{
		Parameter: &param,
	}
}

func TestGenerate_Success(t *testing.T) {
	p := &plugin{
		GroupName: "protocolbuffers",
		Name:      "go",
		Version:   "v1.36.10",
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	resp, err := p.Generate(context.Background(), getMockRequest())
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(resp.GetFile()) != 1 || resp.GetFile()[0].GetName() != "generated.go" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestGenerate_BinaryNotFound(t *testing.T) {
	p := &plugin{
		GroupName: "protocolbuffers",
		Name:      "go",
		Version:   "v1.36.10",
		pluginConfig: PluginConfig{
			Command: []string{"/plugins/nonexistent/plugin"},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no such file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerate_PermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()

	unexecutableFile := filepath.Join(tmpDir, "unexecutable")
	err := os.WriteFile(unexecutableFile, []byte("#!/bin/sh\nexit 0\n"), 0o600)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{unexecutableFile},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err = p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerate_NonZeroExit(t *testing.T) {
	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_EXIT_CODE": "42", "MOCK_STDERR": "test stderr message"},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exit status 42") || !strings.Contains(err.Error(), "test stderr message") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenerate_EnvironmentIsolation(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://secret:password@localhost:5432/db")
	t.Setenv("LICENSE_KEY", "secret-paseto-token")

	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_PRINT_ENV": "1", "MOCK_EXIT_CODE": "1"},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errText := err.Error()
	if strings.Contains(errText, "DATABASE_URL") || strings.Contains(errText, "LICENSE_KEY") {
		t.Errorf("secrets leaked into process env: %s", errText)
	}
}

func TestGenerate_CustomEnv(t *testing.T) {
	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_PRINT_ENV": "1", "CUSTOM_VAR": "propagated-value", "MOCK_EXIT_CODE": "1"},
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errText := err.Error()
	if !strings.Contains(errText, "CUSTOM_VAR=propagated-value") {
		t.Errorf("custom env not propagated: %s", errText)
	}
}

func TestGenerate_OutputSizeLimit(t *testing.T) {
	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_OUTPUT_SIZE": "1000"},
		},
		maxOutputSize: 100,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "output limit exceeded") && !strings.Contains(err.Error(), "too large") && !strings.Contains(err.Error(), "limit") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerate_Timeout(t *testing.T) {
	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_SLEEP": "5s"},
			Timeout: "100ms",
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGenerate_ProcessGroupKill(t *testing.T) {
	p := &plugin{
		pluginConfig: PluginConfig{
			Command: []string{mockPluginPath},
			Env:     map[string]string{"MOCK_FORK": "1", "MOCK_SLEEP": "5s"},
			Timeout: "100ms",
		},
		maxOutputSize: 10 * 1024 * 1024,
	}

	_, err := p.Generate(context.Background(), getMockRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	time.Sleep(200 * time.Millisecond)

	cmd := exec.CommandContext(context.Background(), "pgrep", "-f", "mock_plugin")
	output, _ := cmd.CombinedOutput()
	pids := strings.Fields(string(output))
	if len(pids) > 1 {
		t.Logf("found remaining mock_plugin processes: %v", pids)
	}
}
