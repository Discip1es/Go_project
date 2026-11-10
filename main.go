package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"taskmgr/internal/cli"
	"taskmgr/internal/config"
	"taskmgr/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("taskmgr", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to config file")
	logLevel := fs.String("log-level", "", "override log level (debug|info|warn|error)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	logger := newLogger(cfg)
	store := storage.NewJSONStorage(cfg.DataFile)

	app := cli.NewApp(cfg, store, logger)
	return app.Run(ctx, fs.Args())
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if cfg.LogFile != "" {
		if f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			handler = slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
		}
	}
	if handler == nil {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	return slog.New(handler)
}
