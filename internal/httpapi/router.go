package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sambady/heretous/internal/config"
	"github.com/sambady/heretous/internal/session"
)

func NewHandler(db *sql.DB, cfg config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	auth := &sessionAPI{store: session.NewStore(db, cfg.Session.TTL), cfg: cfg.Session, logger: logger}
	mux.HandleFunc("POST /api/auth/guest", auth.guest)
	mux.HandleFunc("GET /api/session", auth.current)
	mux.HandleFunc("DELETE /api/session", auth.logout)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if db.PingContext(ctx) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		index := filepath.Join(cfg.WebDir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "Страница временно недоступна.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'")
		http.ServeFile(w, r, index)
	})
	assets := http.FileServer(http.Dir(cfg.WebDir))
	mux.HandleFunc("GET /assets/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		assets.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.WebDir, "favicon.svg"))
	})

	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusForbidden, "cross_origin", "Запрос с другого сайта запрещён.")
	}))
	protected := protection.Handler(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		protected.ServeHTTP(w, r)
	})
}
