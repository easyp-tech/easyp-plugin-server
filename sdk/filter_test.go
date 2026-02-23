package sdk

import (
	"testing"

	generator "github.com/easyp-tech/service/api/generator/v1"
)

// testPlugins returns a reusable slice of PluginInfo for filter tests.
func testPlugins() []*generator.PluginInfo {
	return []*generator.PluginInfo{
		{Id: "1", Group: "protocolbuffers", Name: "go", Version: "v1.0.0", Tags: []string{"go", "official", "protobuf"}},
		{Id: "2", Group: "protocolbuffers", Name: "python", Version: "v1.0.0", Tags: []string{"python", "official", "protobuf"}},
		{Id: "3", Group: "grpc", Name: "go", Version: "v2.0.0", Tags: []string{"go", "grpc", "official"}},
		{Id: "4", Group: "grpc", Name: "python", Version: "v2.0.0", Tags: []string{"python", "grpc", "official"}},
		{Id: "5", Group: "community", Name: "rust", Version: "v1.0.0", Tags: []string{"rust", "community"}},
	}
}

func TestFilter_EmptyFilter_ReturnsAll(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{})

	if len(got) != len(plugins) {
		t.Fatalf("expected %d plugins, got %d", len(plugins), len(got))
	}
	for i := range plugins {
		if got[i] != plugins[i] {
			t.Errorf("plugin[%d]: expected %v, got %v", i, plugins[i], got[i])
		}
	}
}

func TestFilter_ByGroupOnly(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Group: "grpc"})

	if len(got) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got))
	}
	for _, p := range got {
		if p.GetGroup() != "grpc" {
			t.Errorf("expected group grpc, got %s", p.GetGroup())
		}
	}
}

func TestFilter_ByNameOnly(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Name: "go"})

	if len(got) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got))
	}
	for _, p := range got {
		if p.GetName() != "go" {
			t.Errorf("expected name go, got %s", p.GetName())
		}
	}
}

func TestFilter_ByVersionOnly(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Version: "v1.0.0"})

	if len(got) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(got))
	}
	for _, p := range got {
		if p.GetVersion() != "v1.0.0" {
			t.Errorf("expected version v1.0.0, got %s", p.GetVersion())
		}
	}
}

func TestFilter_ByGroupAndName(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Group: "protocolbuffers", Name: "python"})

	if len(got) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(got))
	}
	if got[0].GetId() != "2" {
		t.Errorf("expected plugin id 2, got %s", got[0].GetId())
	}
}

func TestFilter_NoMatch_ReturnsEmpty(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Group: "nonexistent"})

	if len(got) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(got))
	}
}

func TestFilter_NilSlice_ReturnsEmpty(t *testing.T) {
	got := applyFilter(nil, PluginFilter{Name: "go"})

	if len(got) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(got))
	}
}

func TestFilter_ByTagSingle(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Tags: []string{"go"}})

	if len(got) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got))
	}
	if got[0].GetId() != "1" || got[1].GetId() != "3" {
		t.Errorf("expected plugins 1 and 3, got %s and %s", got[0].GetId(), got[1].GetId())
	}
}

func TestFilter_ByTagMultiple(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Tags: []string{"go", "official"}})

	if len(got) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got))
	}
	if got[0].GetId() != "1" || got[1].GetId() != "3" {
		t.Errorf("expected plugins 1 and 3, got %s and %s", got[0].GetId(), got[1].GetId())
	}
}

func TestFilter_ByTagNoMatch(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Tags: []string{"java"}})

	if len(got) != 0 {
		t.Fatalf("expected 0 plugins, got %d", len(got))
	}
}

func TestFilter_ByTagEmptyString(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Tags: []string{""}})

	if len(got) != len(plugins) {
		t.Fatalf("expected %d plugins (empty string ignored), got %d", len(plugins), len(got))
	}
}

func TestFilter_ByGroupAndTag(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Group: "grpc", Tags: []string{"go"}})

	if len(got) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(got))
	}
	if got[0].GetId() != "3" {
		t.Errorf("expected plugin 3, got %s", got[0].GetId())
	}
}

func TestFilter_EmptyTagsField(t *testing.T) {
	plugins := testPlugins()
	got := applyFilter(plugins, PluginFilter{Tags: nil})

	if len(got) != len(plugins) {
		t.Fatalf("expected %d plugins, got %d", len(plugins), len(got))
	}
}

func TestPluginFilter_IsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		filter PluginFilter
		want   bool
	}{
		{"zero value", PluginFilter{}, true},
		{"group set", PluginFilter{Group: "grpc"}, false},
		{"name set", PluginFilter{Name: "go"}, false},
		{"version set", PluginFilter{Version: "v1"}, false},
		{"tags set", PluginFilter{Tags: []string{"go"}}, false},
		{"tags empty slice", PluginFilter{Tags: []string{}}, true},
		{"tags nil", PluginFilter{Tags: nil}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.isEmpty(); got != tt.want {
				t.Errorf("isEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
