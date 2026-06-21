package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	defer cancel()

	go forceShutdown(ctx)

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
							return runServiceStart(ctx, cfgPath, logLvl)
						},
					},
				},
			},
			{
				Name:  "plugins",
				Usage: "Manage plugins",
				Commands: []*cli.Command{
					{
						Name:      "migrate",
						Usage:     "Run migrations for plugins",
						ArgsUsage: "<path>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "addr",
								Usage: "gRPC server address",
								Value: "localhost:8080",
							},
							&cli.StringFlag{
								Name:  "filter",
								Usage: "glob filter pattern for plugins (e.g. 'connectrpc/*')",
								Value: "",
							},
							&cli.BoolFlag{
								Name:  "non-interactive",
								Usage: "disable interactive UI and dynamic progress bars",
								Value: false,
							},
							&cli.StringFlag{
								Name:  "plugins-prefix",
								Usage: "prefix directory for plugins on the server",
								Value: "/plugins",
							},
						},
						Action: func(ctx context.Context, cmd *cli.Command) error {
							args := cmd.Args().Slice()
							if len(args) < 1 {
								return fmt.Errorf("missing path argument; usage: easyp-svc plugins migrate <path>")
							}
							path := args[0]
							addr := cmd.String("addr")
							filter := cmd.String("filter")
							nonInteractive := cmd.Bool("non-interactive")
							pluginsPrefix := cmd.String("plugins-prefix")

							return runPluginsMigrate(ctx, path, addr, filter, pluginsPrefix, nonInteractive)
						},
					},
				},
			},
		},
	}

	if err := app.Run(ctx, os.Args); err != nil {
		slog.Error("Failed to run app", "error", err)
		os.Exit(1)
	}
}
