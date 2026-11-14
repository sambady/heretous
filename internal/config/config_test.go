package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.ShutdownTimeout != 10*time.Second || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "250ms")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 250*time.Millisecond || cfg.LogLevel != slog.LevelDebug {
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
			t.Setenv("HTTP_ADDR", "")
			t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
			t.Setenv("LOG_LEVEL", "")
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected a configuration error")
			}
		})
	}
}
