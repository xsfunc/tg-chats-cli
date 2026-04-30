// Package config provides configuration management for the application.
package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	TelegramAppID                 int
	TelegramAppHash               string
	Phone                         string
	LogLevel                      string
	RateLimitMs                   int
	TelegramConnectTimeoutSeconds int
	HistoryDelayMinMs             int
	HistoryDelayMaxMs             int
	FloodWaitMaxSeconds           int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	appIDStr := os.Getenv("TG_APP_ID")
	if appIDStr == "" {
		return nil, errors.New("TG_APP_ID environment variable is required")
	}
	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		return nil, errors.New("TG_APP_ID must be an integer")
	}

	appHash := os.Getenv("TG_APP_HASH")
	if appHash == "" {
		return nil, errors.New("TG_APP_HASH environment variable is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	rateLimitStr := os.Getenv("RATE_LIMIT_MS")
	rateLimit := 350 // Default safe limit
	if rateLimitStr != "" {
		if r, err := strconv.Atoi(rateLimitStr); err == nil {
			rateLimit = r
		}
	}

	connectTimeout := intEnv("TG_CONNECT_TIMEOUT_SECONDS", 60)
	if connectTimeout < 0 {
		connectTimeout = 60
	}

	historyDelayMin := intEnv("HISTORY_DELAY_MIN_MS", 2000)
	historyDelayMax := intEnv("HISTORY_DELAY_MAX_MS", 4000)
	if historyDelayMin <= 0 {
		historyDelayMin = 2000
	}
	if historyDelayMax <= 0 {
		historyDelayMax = 4000
	}
	if historyDelayMin > historyDelayMax {
		historyDelayMin, historyDelayMax = historyDelayMax, historyDelayMin
	}

	floodWaitMax := intEnv("FLOOD_WAIT_MAX_SECONDS", 900)
	if floodWaitMax <= 0 {
		floodWaitMax = 900
	}

	return &Config{
		TelegramAppID:                 appID,
		TelegramAppHash:               appHash,
		Phone:                         os.Getenv("TG_PHONE"),
		LogLevel:                      logLevel,
		RateLimitMs:                   rateLimit,
		TelegramConnectTimeoutSeconds: connectTimeout,
		HistoryDelayMinMs:             historyDelayMin,
		HistoryDelayMaxMs:             historyDelayMax,
		FloodWaitMaxSeconds:           floodWaitMax,
	}, nil
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
