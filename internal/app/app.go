package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/sambady/heretous/internal/config"
	"github.com/sambady/heretous/internal/httpapi"
)

type App struct {
	server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

func New(cfg config.Config, logger *slog.Logger, db *sql.DB) *App {
	return &App{
		server: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           httpapi.NewHandler(db, cfg, logger),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		},
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}
}

// Run serves HTTP until ctx is canceled, then waits for active requests to finish.
func (a *App) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.server.Addr, err)
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- a.server.Serve(listener)
	}()
	a.logger.Info("http server started", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		// Serve errors can leave active connections open.
		_ = a.server.Close()
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
		a.logger.Info("http server shutting down")
	}

	// The signal context is already canceled; shutdown needs an independent deadline.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		_ = a.server.Close()
		<-serveErr
		return fmt.Errorf("shutdown http server: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	a.logger.Info("http server stopped")
	return nil
}
