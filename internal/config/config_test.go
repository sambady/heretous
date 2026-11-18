package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	useConfig(t, "{}\n")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.ShutdownTimeout != 10*time.Second || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadEnvironment(t *testing.T) {
	useConfig(t, "http_addr: ':7070'\nlog_level: WARN\nshutdown_timeout: 2s\ndatabase:\n  url: postgres://file/test\n")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "250ms")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("DATABASE_URL", "postgres://env/test")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 250*time.Millisecond || cfg.LogLevel != slog.LevelDebug || cfg.Database.URL != "postgres://env/test" {
		t.Fatalf("environment was not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{"invalid duration", "HTTP_SHUTDOWN_TIMEOUT", "later"},
		{"zero duration", "HTTP_SHUTDOWN_TIMEOUT", "0s"},
		{"negative duration", "HTTP_SHUTDOWN_TIMEOUT", "-1s"},
		{"invalid log level", "LOG_LEVEL", "verbose"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useConfig(t, "{}\n")
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected a configuration error")
			}
		})
	}
}

func TestLoadYAML(t *testing.T) {
	useConfig(t, "http_addr: ':9091'\nshutdown_timeout: 3s\nlog_level: WARN\nmigration_timeout: 2m\ndatabase:\n  url: postgres://localhost/example\n  max_open_connections: 20\n  max_idle_connections: 4\n  connect_timeout: 7s\n  conn_max_lifetime: 1h\n")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9091" || cfg.ShutdownTimeout != 3*time.Second || cfg.LogLevel != slog.LevelWarn || cfg.MigrationTimeout != 2*time.Minute {
		t.Fatalf("YAML settings not applied: %+v", cfg)
	}
	if cfg.Database.URL != "postgres://localhost/example" || cfg.Database.MaxOpenConnections != 20 || cfg.Database.MaxIdleConnections != 4 || cfg.Database.ConnectTimeout != 7*time.Second || cfg.Database.ConnMaxLifetime != time.Hour {
		t.Fatal("database YAML settings not applied")
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	for _, content := range []string{
		"unknown_setting: true\n",
		"database:\n  unknown_setting: true\n",
		"shutdown_timeout: 0s\n",
		"migration_timeout: -1s\n",
		"database:\n  connect_timeout: 0s\n",
		"database:\n  max_open_connections: 0\n",
		"database:\n  max_idle_connections: -1\n",
		"database:\n  max_idle_connections: 11\n",
		"database:\n  url: ''\n",
		"log_level: wrong\n",
		"http_addr: [\n",
		"{}\n---\n{}\n",
	} {
		t.Run(content, func(t *testing.T) {
			useConfig(t, content)
			if _, err := Load(); err == nil {
				t.Fatal("expected invalid config to fail")
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := Load(); err == nil {
		t.Fatal("expected missing config to fail")
	}
}

func useConfig(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	for _, key := range []string{"HTTP_ADDR", "HTTP_SHUTDOWN_TIMEOUT", "LOG_LEVEL", "DATABASE_URL"} {
		t.Setenv(key, "")
	}
}
