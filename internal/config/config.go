package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	HTTPAddr         string        `yaml:"-"`
	ShutdownTimeout  time.Duration `yaml:"shutdown_timeout"`
	LogLevel         slog.Level    `yaml:"-"`
	MigrationTimeout time.Duration `yaml:"migration_timeout"`
	Database         Database      `yaml:"database"`
}

type Database struct {
	URL                string        `yaml:"-"`
	ConnectTimeout     time.Duration `yaml:"connect_timeout"`
	MaxOpenConnections int           `yaml:"max_open_connections"`
	MaxIdleConnections int           `yaml:"max_idle_connections"`
	ConnMaxLifetime    time.Duration `yaml:"conn_max_lifetime"`
}

func Load() (Config, error) {
	path := envOrDefault("CONFIG_PATH", "config.yaml")
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("config must contain exactly one YAML document")
	}
	cfg.HTTPAddr = envOrDefault("HTTP_ADDR", ":8080")
	cfg.Database.URL = os.Getenv("DATABASE_URL")
	cfg.LogLevel = slog.LevelInfo

	if value := os.Getenv("LOG_LEVEL"); value != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(value)); err != nil {
			return Config{}, fmt.Errorf("invalid LOG_LEVEL: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) validate() error {
	if cfg.HTTPAddr == "" {
		return errors.New("HTTP_ADDR must not be empty")
	}
	if cfg.ShutdownTimeout <= 0 || cfg.MigrationTimeout <= 0 {
		return errors.New("shutdown_timeout and migration_timeout must be positive")
	}
	if cfg.Database.URL == "" {
		return errors.New("DATABASE_URL must be set")
	}
	if cfg.Database.ConnectTimeout <= 0 || cfg.Database.ConnMaxLifetime <= 0 {
		return errors.New("database connect_timeout and conn_max_lifetime must be positive")
	}
	if cfg.Database.MaxOpenConnections <= 0 || cfg.Database.MaxIdleConnections < 0 || cfg.Database.MaxIdleConnections > cfg.Database.MaxOpenConnections {
		return errors.New("database requires max_open_connections > 0 and 0 <= max_idle_connections <= max_open_connections")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
