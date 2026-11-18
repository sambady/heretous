package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sambady/heretous/internal/app"
	"github.com/sambady/heretous/internal/config"
	"github.com/sambady/heretous/internal/database"
	"github.com/sambady/heretous/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()
	logger.Info("database connected")

	provider, err := migrations.New(db)
	if err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	logger.Info("applying startup migrations", "timeout", cfg.MigrationTimeout)
	migrationCtx, cancel := context.WithTimeout(ctx, cfg.MigrationTimeout)
	applied, err := provider.Up(migrationCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("apply startup migrations: %w", err)
	}
	logger.Info("startup migrations applied", "count", len(applied))

	return app.New(cfg, logger, db).Run(ctx)
}
