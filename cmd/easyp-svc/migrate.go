package main

import (
	"context"
	"errors"
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

const (
	progressBarWidth     = 20
	percentMultiplier    = 100
	spinnerSleepDuration = 50 * time.Millisecond
	separatorLength      = 40
)

var (
	// ErrDirectoryNotExist is returned when the target directory does not exist.
	ErrDirectoryNotExist = errors.New("directory does not exist")
	// ErrNotADirectory is returned when the target path is not a directory.
	ErrNotADirectory = errors.New("path is not a directory")
	// ErrPathOutsideBase is returned when the target path is outside the base directory.
	ErrPathOutsideBase = errors.New("path outside base directory")
	// ErrInvalidStructure is returned when the path structure is invalid.
	ErrInvalidStructure = errors.New("does not match expected structure")
)

type pluginInfo struct {
	group   string
	name    string
	version string
	path    string
}

func getSpinners() []string {
	return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
}

func runPluginsMigrate(ctx context.Context, scanPath string, addr string, filter string, pluginsPrefix string, nonInteractive bool) error {
	plugins, err := scanPlugins(scanPath, filter)
	if err != nil {
		return err
	}

	total := len(plugins)
	if total == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No plugins found matching the criteria.")

		return nil
	}

	client, err := sdk.NewClient(addr, sdk.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server at %s: %w", addr, err)
	}
	defer func() { _ = client.Close() }()

	isInteractive := !nonInteractive && term.IsTerminal(int(os.Stdout.Fd()))

	var registered, skipped, failed int
	spinners := getSpinners()
	spinIdx := 0

	for idx, plg := range plugins {
		pName := fmt.Sprintf("%s/%s:%s", plg.group, plg.name, plg.version)

		if isInteractive {
			pct := int(float64(idx) / float64(total) * percentMultiplier)
			progressBar := renderProgressBar(pct, progressBarWidth)
			_, _ = fmt.Fprintf(os.Stdout, "\r\033[K%s %s Регистрация %s... %d%% (%d/%d)", spinners[spinIdx], progressBar, pName, pct, idx, total)
			spinIdx = (spinIdx + 1) % len(spinners)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "Registering %s...\n", pName)
		}

		isSkipped, errReg := registerSinglePlugin(ctx, client, plg, pluginsPrefix)
		processMigrationResult(pName, errReg, isSkipped, isInteractive, &registered, &skipped, &failed)

		if isInteractive {
			time.Sleep(spinnerSleepDuration)
		}
	}

	if isInteractive {
		progressBar := renderProgressBar(percentMultiplier, progressBarWidth)
		_, _ = fmt.Fprintf(
			os.Stdout,
			"\r\033[K%s %s Готово! 100%% (%d/%d)\n",
			"✓",
			progressBar,
			total,
			total,
		)
	}

	printSummary(total, registered, skipped, failed)

	return nil
}

func printSummary(total, registered, skipped, failed int) {
	_, _ = fmt.Fprintln(os.Stdout, "\nРезультаты миграции:")
	_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("-", separatorLength))
	_, _ = fmt.Fprintf(os.Stdout, "%-25s : %d\n", "Всего плагинов найдено", total)
	_, _ = fmt.Fprintf(os.Stdout, "%-25s : %d\n", "Успешно зарегистрировано", registered)
	_, _ = fmt.Fprintf(os.Stdout, "%-25s : %d\n", "Пропущено (уже создано)", skipped)
	_, _ = fmt.Fprintf(os.Stdout, "%-25s : %d\n", "Ошибка регистрации", failed)
	_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("-", separatorLength))
}

func processMigrationResult(pName string, errReg error, isSkipped bool, isInteractive bool, registered *int, skipped *int, failed *int) {
	if errReg != nil {
		*failed++
		if !isInteractive {
			_, _ = fmt.Fprintf(os.Stderr, "Error registering %s: %v\n", pName, errReg)
		}

		return
	}

	if isSkipped {
		*skipped++
		if !isInteractive {
			_, _ = fmt.Fprintf(os.Stdout, "Skipped (already exists): %s\n", pName)
		}

		return
	}

	*registered++
	if !isInteractive {
		_, _ = fmt.Fprintf(os.Stdout, "Successfully registered %s\n", pName)
	}
}

func scanPlugins(scanPath string, filter string) ([]pluginInfo, error) {
	info, err := os.Stat(scanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrDirectoryNotExist, scanPath)
		}

		return nil, fmt.Errorf("failed to access directory %s: %w", scanPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNotADirectory, scanPath)
	}

	cleanedPath := filepath.Clean(scanPath)

	var plugins []pluginInfo
	err = filepath.WalkDir(cleanedPath, func(path string, dirEntry os.DirEntry, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		if dirEntry.IsDir() {
			return nil
		}
		if dirEntry.Name() == "plugin" {
			group, name, version, parseErr := parsePluginPath(cleanedPath, path)
			if parseErr == nil && matchFilter(group, name, version, filter) {
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
		return nil, fmt.Errorf("scanning directory: %w", err)
	}

	return plugins, nil
}

func registerSinglePlugin(ctx context.Context, client *sdk.Client, plg pluginInfo, pluginsPrefix string) (bool, error) {
	targetPath := pluginsPrefix + "/" + plg.group + "/" + plg.name + "/" + plg.version + "/plugin"
	configMap := map[string]any{
		"command": []any{targetPath},
	}

	_, registerErr := client.CreatePlugin(ctx, plg.group, plg.name, plg.version, configMap, nil)
	if registerErr != nil {
		st, ok := status.FromError(registerErr)
		if ok && st.Code() == codes.AlreadyExists {
			return true, nil
		}

		return false, fmt.Errorf("sdk.Client.CreatePlugin: %w", registerErr)
	}

	return false, nil
}

func parsePluginPath(basePath, fullPath string) (string, string, string, error) {
	rel, err := filepath.Rel(basePath, fullPath)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", "", ErrPathOutsideBase
	}

	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	if len(parts) != 4 || parts[3] != "plugin" {
		return "", "", "", ErrInvalidStructure
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
	filled = min(filled, width)
	empty := width - filled

	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}
