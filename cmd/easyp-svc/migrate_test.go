package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePluginsPrefix(t *testing.T) {
	t.Parallel()

	t.Run("explicit prefix wins over cfg", func(t *testing.T) {
		t.Parallel()

		cfgPath := writeTempConfig(t, "./from-cfg")
		got, err := resolvePluginsPrefix(cfgPath, "./explicit", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "explicit" && got != filepath.Clean("./explicit") {
			t.Fatalf("expected cleaned explicit prefix, got %q", got)
		}
	})

	t.Run("cfg plugins_dir when prefix not explicit", func(t *testing.T) {
		t.Parallel()

		cfgPath := writeTempConfig(t, "./plugins")
		got, err := resolvePluginsPrefix(cfgPath, defaultPluginsPrefix, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Clean("./plugins")
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("default when no cfg and prefix not explicit", func(t *testing.T) {
		t.Parallel()

		got, err := resolvePluginsPrefix("", defaultPluginsPrefix, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != defaultPluginsPrefix {
			t.Fatalf("expected %q, got %q", defaultPluginsPrefix, got)
		}
	})

	t.Run("empty plugins_dir in cfg", func(t *testing.T) {
		t.Parallel()

		cfgPath := writeTempConfig(t, "")
		_, err := resolvePluginsPrefix(cfgPath, defaultPluginsPrefix, false)
		if err == nil {
			t.Fatal("expected error for empty plugins_dir")
		}
	})

	t.Run("missing cfg file", func(t *testing.T) {
		t.Parallel()

		_, err := resolvePluginsPrefix(filepath.Join(t.TempDir(), "missing.yml"), defaultPluginsPrefix, false)
		if err == nil {
			t.Fatal("expected error for missing cfg")
		}
	})
}

func writeTempConfig(t *testing.T, pluginsDir string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := "registry:\n  plugins_dir: " + pluginsDir + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return path
}
