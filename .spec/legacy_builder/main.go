package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

type PluginConfig struct {
	Binary      string            `yaml:"binary"`
	Description string            `yaml:"description,omitempty"`
	SourceURL   string            `yaml:"source_url,omitempty"`
	BuildArgs   map[string]string `yaml:"build_args,omitempty"`
	Versions    []interface{}     `yaml:"versions"`
}

type BuildJob struct {
	Group, Name, Version, Binary string
	BuildArgs                    map[string]string
	PluginDir, OutputDir         string
}

func (j BuildJob) Key() string { return fmt.Sprintf("%s/%s:%s", j.Group, j.Name, j.Version) }

// needsBuild returns true if the plugin version has not been built yet
func needsBuild(job BuildJob) bool {
	for _, path := range []string{
		filepath.Join(job.OutputDir, "plugin"),
		filepath.Join(job.OutputDir, job.Binary),
	} {
		if _, err := os.Stat(path); err == nil {
			return false
		}
	}
	if stat, err := os.Stat(filepath.Join(job.OutputDir, "app")); err == nil && stat.IsDir() {
		return false
	}
	return true
}

// Tracker keeps state of active builds for display
type Tracker struct {
	mu         sync.Mutex
	active     map[string]time.Time
	completed  int
	succeeded  int
	failed     int
	total      int
	startTime  time.Time
	lastReport time.Time
}

func NewTracker(total int) *Tracker {
	now := time.Now()
	return &Tracker{
		active:     make(map[string]time.Time),
		total:      total,
		startTime:  now,
		lastReport: now,
	}
}

func (t *Tracker) Start(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.active[key] = time.Now()
}

func (t *Tracker) Finish(key string, success bool, details string, dur time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.active, key)
	t.completed++
	if success {
		t.succeeded++
	} else {
		t.failed++
	}
	t.lastReport = time.Now()

	icon := "✅"
	if !success {
		icon = "❌"
	}

	fmt.Printf("[%4d/%4d] %s %s (%s, %s) | active: %d\n",
		t.completed, t.total, icon, key, details, dur, len(t.active))
}

// PrintStalled prints active builds if nothing completed recently
func (t *Tracker) PrintStalled() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.active) == 0 {
		return
	}
	if time.Since(t.lastReport) < 30*time.Second {
		return
	}

	elapsed := time.Since(t.startTime).Round(time.Second)

	type entry struct {
		name string
		dur  time.Duration
	}
	var entries []entry
	for name, started := range t.active {
		entries = append(entries, entry{name, time.Since(started).Round(time.Second)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].dur > entries[j].dur })

	fmt.Printf("  ⏳ [%s] %d/%d done | active: %d\n", elapsed, t.completed, t.total, len(entries))
	for _, e := range entries {
		fmt.Printf("    %-50s (%s)\n", e.name, e.dur)
	}
	t.lastReport = time.Now()
}

func main() {
	registryDir := "registry"
	outputDir := "plugins"

	var configs []string
	filepath.Walk(registryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "plugin.yaml" {
			configs = append(configs, path)
		}
		return nil
	})

	sort.Strings(configs)

	var allJobs []BuildJob
	for _, configPath := range configs {
		data, _ := os.ReadFile(configPath)
		var cfg PluginConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("Failed to parse %s: %v", configPath, err)
		}

		pluginDir := filepath.Dir(configPath)
		name := filepath.Base(pluginDir)
		group := filepath.Base(filepath.Dir(pluginDir))

		for _, v := range cfg.Versions {
			var ver string
			switch val := v.(type) {
			case string:
				ver = val
			case map[string]interface{}:
				ver = val["version"].(string)
			}
			if group == "apple" && name == "swift" && ver == "v1.25.2" {
				allJobs = append(allJobs, BuildJob{
					Group: group, Name: name, Version: ver,
					Binary: cfg.Binary, BuildArgs: cfg.BuildArgs,
					PluginDir: pluginDir,
					OutputDir: filepath.Join(outputDir, group, name, ver),
				})
			}
		}
	}

	// Pre-scan: separate cached from uncached
	var toBuild []BuildJob
	cached := 0
	for _, job := range allJobs {
		if needsBuild(job) {
			toBuild = append(toBuild, job)
		} else {
			cached++
		}
	}

	fmt.Printf("Found %d total: %d cached ⏭, %d to build (parallel=%d)\n\n", len(allJobs), cached, len(toBuild), 10)

	if len(toBuild) == 0 {
		fmt.Println("Nothing to build, all cached!")
		return
	}

	tracker := NewTracker(len(toBuild))

	// Safety ticker: prints active builds if nothing completed for 30s
	ticker := time.NewTicker(10 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				tracker.PrintStalled()
			case <-done:
				return
			}
		}
	}()

	var g errgroup.Group
	g.SetLimit(3)

	for _, j := range toBuild {
		job := j
		g.Go(func() error {
			key := job.Key()
			start := time.Now()

			os.MkdirAll(job.OutputDir, 0755)
			tracker.Start(key)

			var outBuf bytes.Buffer
			args := []string{
				"build", "--progress=plain",
				"--build-arg", fmt.Sprintf("VERSION=%s", job.Version),
				"--build-arg", fmt.Sprintf("BINARY_NAME=%s", job.Binary),
			}
			for k, v := range job.BuildArgs {
				args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
			}
			args = append(args, "--output", job.OutputDir+"/", "-f",
				filepath.Join(job.PluginDir, "Dockerfile"), job.PluginDir)

			cmd := exec.Command("docker", args...)
			cmd.Stdout = &outBuf
			cmd.Stderr = &outBuf

			err := cmd.Run()

			logPath := filepath.Join(job.OutputDir, "build.log")
			os.WriteFile(logPath, outBuf.Bytes(), 0644)

			dur := time.Since(start).Round(time.Second)

			pluginFile := filepath.Join(job.OutputDir, "plugin")
			binaryFile := filepath.Join(job.OutputDir, job.Binary)
			appDir := filepath.Join(job.OutputDir, "app")

			if err != nil {
				os.WriteFile(logPath, []byte(fmt.Sprintf("\n%s\nExec Error: %v\n", outBuf.Bytes(), err)), 0644)
				tracker.Finish(key, false, fmt.Sprintf("see %s", logPath), dur)
				return nil
			}

			sizeStr := ""
			if stat, err := os.Stat(pluginFile); err == nil {
				os.Chmod(pluginFile, 0755)
				sizeStr = formatSize(stat.Size())
			} else if stat, err := os.Stat(binaryFile); err == nil {
				os.Rename(binaryFile, pluginFile)
				os.Chmod(pluginFile, 0755)
				sizeStr = formatSize(stat.Size())
			} else if stat, err := os.Stat(appDir); err == nil && stat.IsDir() {
				sizeStr = "app dir"
			} else {
				tracker.Finish(key, false, "no output binary", dur)
				return nil
			}

			tracker.Finish(key, true, sizeStr, dur)
			return nil
		})
	}

	g.Wait()
	ticker.Stop()
	close(done)

	elapsed := time.Since(tracker.startTime).Round(time.Second)
	fmt.Printf("\n══════════════════════════════════════════\n")
	fmt.Printf("  DONE in %s\n", elapsed)
	fmt.Printf("  Built:   %d ✅\n", tracker.succeeded)
	fmt.Printf("  Failed:  %d ❌\n", tracker.failed)
	fmt.Printf("  Cached:  %d ⏭\n", cached)
	fmt.Printf("══════════════════════════════════════════\n")
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}
