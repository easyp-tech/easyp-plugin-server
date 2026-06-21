package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "easyp-svc",
		Usage: "EasyP Service CLI",
		Commands: []*cli.Command{
			{
				Name:  "service",
				Usage: "Manage the easyp service",
				Commands: []*cli.Command{
					{
						Name:  "start",
						Usage: "Start the gRPC/MCP service",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "cfg",
								Usage: "path to config file",
								Value: "",
							},
							&cli.StringFlag{
								Name:  "log_level",
								Usage: "log level (debug, info, warn, error)",
								Value: "debug",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							cfgPath := cmd.String("cfg")
							logLvl := cmd.String("log_level")

							fmt.Printf("Starting easyp-svc with config: %q, log level: %q\n", cfgPath, logLvl)
							slog.Info("Service started (stub)")

							// TODO: Port start() logic from legacy main.go here
							return nil
						},
					},
				},
			},
			{
				Name:  "plugins",
				Usage: "Manage plugins",
				Commands: []*cli.Command{
					{
						Name:  "migrate",
						Usage: "Run migrations for plugins",
						Action: func(ctx context.Context, cmd *cli.Command) error {
							fmt.Println("Running plugin migrations (stub)...")
							slog.Info("Plugin migrations executed")

							// TODO: Implement migration logic here
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("Failed to run app", "error", err)
		os.Exit(1)
	}
}
