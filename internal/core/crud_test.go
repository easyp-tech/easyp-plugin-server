package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/protobuf/types/pluginpb"
)

// --- Mock FeatureGate ---

type mockFeatureGate struct {
	enabledFn    func(Feature) bool
	maxWorkersFn func() int
	maxPluginsFn func() int
}

func (m *mockFeatureGate) Enabled(feature Feature) bool {
	if m.enabledFn != nil {
		return m.enabledFn(feature)
	}
	return true
}

func (m *mockFeatureGate) MaxWorkers() int {
	if m.maxWorkersFn != nil {
		return m.maxWorkersFn()
	}
	return -1
}

func (m *mockFeatureGate) MaxPlugins() int {
	if m.maxPluginsFn != nil {
		return m.maxPluginsFn()
	}
	return -1
}

// TestGenerate_PreservationAfterCRUD verifies that Core.Generate() continues
// to work correctly. This locks existing behavior before interface extensions.
func TestGenerate_PreservationAfterCRUD(t *testing.T) {
	t.Parallel()

	wantResp := &pluginpb.CodeGeneratorResponse{}
	wantInfo := &PluginInfo{Group: "grpc", Name: "go", Version: "v1.5.1"}

	reg := &mockRegistry{
		getFn: func(_ context.Context, group, name, version string) (Plugin, error) {
			if group != "grpc" || name != "go" || version != "v1.5.1" {
				t.Fatalf("unexpected Get(%q, %q, %q)", group, name, version)
			}
			return &mockPlugin{
				generateFn: func(_ context.Context, _ *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
					return wantResp, nil
				},
				infoFn: func(_ context.Context) *PluginInfo { return wantInfo },
			}, nil
		},
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
	}

	c := New(mockMetrics{}, reg, nil)
	resp, err := c.Generate(context.Background(), GenerateCodeRequest{
		PluginName: "grpc/go:v1.5.1",
		Payload:    &pluginpb.CodeGeneratorRequest{},
	})
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if resp.Payload != wantResp {
		t.Fatal("Generate() returned wrong payload")
	}
}

// TestListPlugins_PreservationAfterCRUD verifies that Core.ListPlugins() continues
// to work correctly. This locks existing behavior before interface extensions.
func TestListPlugins_PreservationAfterCRUD(t *testing.T) {
	t.Parallel()

	want := []PluginInfo{
		{Group: "grpc", Name: "go", Version: "v1.5.1"},
		{Group: "protocolbuffers", Name: "go", Version: "v1.36.10"},
	}

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return want, nil },
	}

	c := New(mockMetrics{}, reg, nil)
	got, err := c.ListPlugins(context.Background(), PluginFilter{})
	if err != nil {
		t.Fatalf("ListPlugins() unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ListPlugins() returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Group != want[i].Group || got[i].Name != want[i].Name || got[i].Version != want[i].Version {
			t.Errorf("ListPlugins()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// --- CreatePlugin tests ---

func TestCreatePlugin_Success(t *testing.T) {
	t.Parallel()

	want := &PluginInfo{Group: "grpc", Name: "go", Version: "v1.5.1"}

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		createFn: func(_ context.Context, req CreatePluginRequest) (*PluginInfo, error) {
			if req.Group != "grpc" || req.Name != "go" || req.Version != "v1.5.1" {
				t.Fatalf("unexpected Create(%+v)", req)
			}
			return want, nil
		},
	}
	gate := &mockFeatureGate{maxPluginsFn: func() int { return -1 }}
	c := New(mockMetrics{}, reg, gate)

	got, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
		Config: json.RawMessage(`{}`), Tags: []string{"stable"},
	})
	if err != nil {
		t.Fatalf("CreatePlugin() unexpected error: %v", err)
	}
	if got.Group != want.Group || got.Name != want.Name || got.Version != want.Version {
		t.Errorf("CreatePlugin() = %+v, want %+v", got, want)
	}
}

func TestCreatePlugin_AlreadyExists(t *testing.T) {
	t.Parallel()

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		createFn: func(_ context.Context, _ CreatePluginRequest) (*PluginInfo, error) {
			return nil, ErrAlreadyExists
		},
	}
	gate := &mockFeatureGate{maxPluginsFn: func() int { return -1 }}
	c := New(mockMetrics{}, reg, gate)

	_, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("CreatePlugin() error = %v, want ErrAlreadyExists", err)
	}
}

func TestCreatePlugin_MaxPluginsExceeded(t *testing.T) {
	t.Parallel()

	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) {
			return make([]PluginInfo, 5), nil
		},
	}
	gate := &mockFeatureGate{maxPluginsFn: func() int { return 5 }}
	c := New(mockMetrics{}, reg, gate)

	_, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if !errors.Is(err, ErrMaxPluginsExceeded) {
		t.Fatalf("CreatePlugin() error = %v, want ErrMaxPluginsExceeded", err)
	}
}

func TestCreatePlugin_MaxPluginsUnlimited(t *testing.T) {
	t.Parallel()

	want := &PluginInfo{Group: "grpc", Name: "go", Version: "v1.5.1"}
	reg := &mockRegistry{
		getFn: func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) {
			return make([]PluginInfo, 100), nil
		},
		createFn: func(_ context.Context, _ CreatePluginRequest) (*PluginInfo, error) {
			return want, nil
		},
	}
	gate := &mockFeatureGate{maxPluginsFn: func() int { return -1 }}
	c := New(mockMetrics{}, reg, gate)

	got, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if err != nil {
		t.Fatalf("CreatePlugin() unexpected error: %v", err)
	}
	if got.Group != want.Group {
		t.Errorf("CreatePlugin() = %+v, want %+v", got, want)
	}
}

func TestCreatePlugin_InvalidName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		group string
		pName string
	}{
		{name: "uppercase group", group: "UPPER", pName: "go"},
		{name: "numeric start group", group: "123abc", pName: "go"},
		{name: "empty group", group: "", pName: "go"},
		{name: "space in group", group: "a b", pName: "go"},
		{name: "uppercase name", group: "grpc", pName: "UPPER"},
		{name: "empty name", group: "grpc", pName: ""},
	}

	gate := &mockFeatureGate{maxPluginsFn: func() int { return -1 }}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
	}
	c := New(mockMetrics{}, reg, gate)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
				Group: tt.group, Name: tt.pName, Version: "v1.0.0",
			})
			if !errors.Is(err, ErrInvalidPluginName) {
				t.Errorf("CreatePlugin(%q, %q) error = %v, want ErrInvalidPluginName", tt.group, tt.pName, err)
			}
		})
	}
}

func TestCreatePlugin_InvalidVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "no v prefix", version: "1.0.0", wantErr: true},
		{name: "latest2", version: "latest2", wantErr: true},
		{name: "empty", version: "", wantErr: true},
		{name: "valid semver", version: "v1.0.0", wantErr: false},
		{name: "latest", version: "latest", wantErr: false},
	}

	want := &PluginInfo{Group: "grpc", Name: "go", Version: "v1.0.0"}
	gate := &mockFeatureGate{maxPluginsFn: func() int { return -1 }}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		createFn: func(_ context.Context, _ CreatePluginRequest) (*PluginInfo, error) {
			return want, nil
		},
	}
	c := New(mockMetrics{}, reg, gate)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
				Group: "grpc", Name: "go", Version: tt.version,
			})
			if tt.wantErr && !errors.Is(err, ErrInvalidPluginName) {
				t.Errorf("CreatePlugin(version=%q) error = %v, want ErrInvalidPluginName", tt.version, err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("CreatePlugin(version=%q) unexpected error: %v", tt.version, err)
			}
		})
	}
}

// --- UpdatePlugin tests ---

func TestUpdatePlugin_Success(t *testing.T) {
	t.Parallel()

	want := &PluginInfo{Group: "grpc", Name: "go", Version: "v1.5.1", Tags: []string{"updated"}}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		updateFn: func(_ context.Context, req UpdatePluginRequest) (*PluginInfo, error) {
			if req.Group != "grpc" || req.Name != "go" || req.Version != "v1.5.1" {
				t.Fatalf("unexpected Update(%+v)", req)
			}
			return want, nil
		},
	}
	c := New(mockMetrics{}, reg, nil)

	got, err := c.UpdatePlugin(context.Background(), UpdatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
		Config: json.RawMessage(`{}`), Tags: []string{"updated"},
	})
	if err != nil {
		t.Fatalf("UpdatePlugin() unexpected error: %v", err)
	}
	if got.Group != want.Group || got.Version != want.Version {
		t.Errorf("UpdatePlugin() = %+v, want %+v", got, want)
	}
}

func TestUpdatePlugin_NotFound(t *testing.T) {
	t.Parallel()

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		updateFn: func(_ context.Context, _ UpdatePluginRequest) (*PluginInfo, error) {
			return nil, ErrNotFound
		},
	}
	c := New(mockMetrics{}, reg, nil)

	_, err := c.UpdatePlugin(context.Background(), UpdatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdatePlugin() error = %v, want ErrNotFound", err)
	}
}

// --- DeletePlugin tests ---

func TestDeletePlugin_Success(t *testing.T) {
	t.Parallel()

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		deleteFn: func(_ context.Context, group, name, version string) error {
			if group != "grpc" || name != "go" || version != "v1.5.1" {
				t.Fatalf("unexpected Delete(%q, %q, %q)", group, name, version)
			}
			return nil
		},
	}
	c := New(mockMetrics{}, reg, nil)

	err := c.DeletePlugin(context.Background(), "grpc", "go", "v1.5.1")
	if err != nil {
		t.Fatalf("DeletePlugin() unexpected error: %v", err)
	}
}

func TestDeletePlugin_NotFound(t *testing.T) {
	t.Parallel()

	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		deleteFn: func(_ context.Context, _, _, _ string) error {
			return ErrNotFound
		},
	}
	c := New(mockMetrics{}, reg, nil)

	err := c.DeletePlugin(context.Background(), "grpc", "go", "v1.5.1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeletePlugin() error = %v, want ErrNotFound", err)
	}
}

// --- Feature denial tests (REQ-1.1 – REQ-1.5) ---

func TestCreatePlugin_FeatureDenied(t *testing.T) {
	t.Parallel()

	gate := &mockFeatureGate{
		enabledFn:    func(_ Feature) bool { return false },
		maxPluginsFn: func() int { return -1 },
	}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		createFn: func(_ context.Context, _ CreatePluginRequest) (*PluginInfo, error) {
			t.Fatal("registry.Create must not be called when feature is denied")
			return nil, nil
		},
	}
	c := New(mockMetrics{}, reg, gate)

	_, err := c.CreatePlugin(context.Background(), CreatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if !errors.Is(err, ErrFeatureDenied) {
		t.Fatalf("CreatePlugin() error = %v, want ErrFeatureDenied", err)
	}
}

func TestUpdatePlugin_FeatureDenied(t *testing.T) {
	t.Parallel()

	gate := &mockFeatureGate{
		enabledFn: func(_ Feature) bool { return false },
	}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		updateFn: func(_ context.Context, _ UpdatePluginRequest) (*PluginInfo, error) {
			t.Fatal("registry.Update must not be called when feature is denied")
			return nil, nil
		},
	}
	c := New(mockMetrics{}, reg, gate)

	_, err := c.UpdatePlugin(context.Background(), UpdatePluginRequest{
		Group: "grpc", Name: "go", Version: "v1.5.1",
	})
	if !errors.Is(err, ErrFeatureDenied) {
		t.Fatalf("UpdatePlugin() error = %v, want ErrFeatureDenied", err)
	}
}

func TestDeletePlugin_FeatureDenied(t *testing.T) {
	t.Parallel()

	gate := &mockFeatureGate{
		enabledFn: func(_ Feature) bool { return false },
	}
	reg := &mockRegistry{
		getFn:  func(_ context.Context, _, _, _ string) (Plugin, error) { return nil, nil },
		listFn: func(_ context.Context, _ PluginFilter) ([]PluginInfo, error) { return nil, nil },
		deleteFn: func(_ context.Context, _, _, _ string) error {
			t.Fatal("registry.Delete must not be called when feature is denied")
			return nil
		},
	}
	c := New(mockMetrics{}, reg, gate)

	err := c.DeletePlugin(context.Background(), "grpc", "go", "v1.5.1")
	if !errors.Is(err, ErrFeatureDenied) {
		t.Fatalf("DeletePlugin() error = %v, want ErrFeatureDenied", err)
	}
}
