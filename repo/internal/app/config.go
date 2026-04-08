package app

import (
	"os"
	"strconv"
)

type Config struct {
	Port             string
	DatabaseURL      string
	SessionSecret    string
	EncryptionKey    string
	UploadsDir       string
	ExportsDir       string
	MaxUploadBytes   int64
	SchedulerEnabled bool
	CutoffInterval   int // seconds
	AlertInterval    int // seconds
	ExportRetryInt   int // seconds
}

func LoadConfig() *Config {
	return &Config{
		Port:             envOrDefault("APP_PORT", "8080"),
		DatabaseURL:      envOrDefault("DATABASE_URL", "postgres://fleet:fleet@localhost:5432/fleetcommerce?sslmode=disable"),
		SessionSecret:    envOrDefault("SESSION_SECRET", "change-me-in-production-32bytes!"),
		EncryptionKey:    envOrDefault("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"), // 32 bytes hex for AES-256
		UploadsDir:       envOrDefault("UPLOADS_DIR", "./web/uploads"),
		ExportsDir:       envOrDefault("EXPORTS_DIR", "./web/exports"),
		MaxUploadBytes:   envOrDefaultInt64("MAX_UPLOAD_BYTES", 25*1024*1024),
		SchedulerEnabled: envOrDefaultBool("SCHEDULER_ENABLED", true),
		CutoffInterval:   envOrDefaultInt("CUTOFF_INTERVAL_SEC", 60),
		AlertInterval:    envOrDefaultInt("ALERT_INTERVAL_SEC", 900),
		ExportRetryInt:   envOrDefaultInt("EXPORT_RETRY_INTERVAL_SEC", 300),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envOrDefaultBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
