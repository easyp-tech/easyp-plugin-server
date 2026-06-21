package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/easyp-tech/service/sdk"
)

type pluginInfo struct {
	group   string
	name    string
	version string
	path    string
}

func runPluginsMigrate(ctx context.Context, scanPath string, addr string, filter string, pluginsPrefix string, nonInteractive bool) error {
	// 1. Verify scanPath exists and is a directory
	info, err := os.Stat(scanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", scanPath)
		}
		return fmt.Errorf("failed to access directory %s: %w", scanPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", scanPath)
	}

	// Clean path
	scanPath = filepath.Clean(scanPath)

	// 2. Scan directory
	var plugins []pluginInfo
	err = filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "plugin" {
			group, name, version, parseErr := parsePluginPath(scanPath, path)
			if parseErr != nil {
				// Ignore invalid plugin structures (REQ-1.3)
				return nil
			}
			// Apply filter if specified
			if matchFilter(group, name, version, filter) {
				plugins = append(plugins, pluginInfo{
					group:   group,
					name:    name,
					version: version,
					path:    path,
				})
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning directory: %w", err)
	}

	total := len(plugins)
	if total == 0 {
		fmt.Println("No plugins found matching the criteria.")
		return nil
	}

	// 3. Connect to server
	client, err := sdk.NewClient(addr, sdk.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server at %s: %w", addr, err)
	}
	defer func() {
		_ = client.Close()
	}()

	// Detect if output is TTY and interactive is enabled
	isInteractive := !nonInteractive && term.IsTerminal(int(os.Stdout.Fd()))

	var registered, skipped, failed int

	// TUI helper variables
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinIdx := 0

	for i, p := range plugins {
		pName := fmt.Sprintf("%s/%s:%s", p.group, p.name, p.version)

		// Dynamic console TUI rendering (REQ-5.1)
		if isInteractive {
			// Update spinner and progress bar
			pct := int(float64(i) / float64(total) * 100)
			progressBar := renderProgressBar(pct, 20)
			// Print current status
			fmt.Printf("\r\033[K%s %s Регистрация %s... %d%% (%d/%d)", spinners[spinIdx], progressBar, pName, pct, i, total)
			spinIdx = (spinIdx + 1) % len(spinners)
		} else {
			// Non-interactive mode (REQ-5.2)
			fmt.Printf("Registering %s...\n", pName)
		}

		// Prepare config
		// format: pluginsPrefix + "/" + group + "/" + name + "/" + version + "/plugin"
		targetPath := pluginsPrefix + "/" + p.group + "/" + p.name + "/" + p.version + "/plugin"
		configMap := map[string]any{
			"command": []any{targetPath},
		}

		// gRPC call
		_, registerErr := client.CreatePlugin(ctx, p.group, p.name, p.version, configMap, nil)
		if registerErr == nil {
			registered++
			if !isInteractive {
				fmt.Printf("Successfully registered %s\n", pName)
			}
		} else {
			st, ok := status.FromError(registerErr)
			if ok && st.Code() == codes.AlreadyExists {
				skipped++
				if !isInteractive {
					fmt.Printf("Skipped (already exists): %s\n", pName)
				}
			} else {
				failed++
				if !isInteractive {
					fmt.Fprintf(os.Stderr, "Error registering %s: %v\n", pName, registerErr)
				}
			}
		}

		// Small sleep for smoother spinner animation in interactive mode
		if isInteractive {
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Final progress update
	if isInteractive {
		fmt.Printf("\r\033[K%s %s Готово! 100%% (%d/%d)\n", "✓", renderProgressBar(100, 20), total, total)
	}

	// Render final summary table (REQ-5.3)
	fmt.Println("\nРезультаты миграции:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("%-25s : %d\n", "Всего плагинов найдено", total)
	fmt.Printf("%-25s : %d\n", "Успешно зарегистрировано", registered)
	fmt.Printf("%-25s : %d\n", "Пропущено (уже создано)", skipped)
	fmt.Printf("%-25s : %d\n", "Ошибка регистрации", failed)
	fmt.Println(strings.Repeat("-", 40))

	return nil
}

func parsePluginPath(basePath string, fullPath string) (group string, name string, version string, err error) {
	rel, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", "", fmt.Errorf("path outside base directory")
	}

	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	// Expected: group/name/version/plugin
	if len(parts) != 4 || parts[3] != "plugin" {
		return "", "", "", fmt.Errorf("does not match expected structure")
	}
	return parts[0], parts[1], parts[2], nil
}

func matchFilter(group string, name string, version string, pattern string) bool {
	if pattern == "" {
		return true
	}
	fullName := group + "/" + name
	matched, err := filepath.Match(pattern, fullName)
	if err == nil && matched {
		return true
	}
	fullNameWithVersion := group + "/" + name + ":" + version
	matched, err = filepath.Match(pattern, fullNameWithVersion)
	return err == nil && matched
}

func renderProgressBar(percent int, width int) string {
	filled := int(float64(percent) / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}
