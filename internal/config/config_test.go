package config

import (
	"os"
	"testing"
)

// Helper to set env vars and clean up
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set %s: %v", key, err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv(key)
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	_ = os.Unsetenv(key)
}

func TestLoad_MissingAppID(t *testing.T) {
	// Clear env
	unsetEnv(t, "TG_APP_ID")
	unsetEnv(t, "TG_APP_HASH")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_ID is missing")
	}
	if err.Error() != "TG_APP_ID environment variable is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidAppID(t *testing.T) {
	setEnv(t, "TG_APP_ID", "not_a_number")
	setEnv(t, "TG_APP_HASH", "somehash")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_ID is not a number")
	}
	if err.Error() != "TG_APP_ID must be an integer" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_MissingAppHash(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	unsetEnv(t, "TG_APP_HASH")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when TG_APP_HASH is missing")
	}
	if err.Error() != "TG_APP_HASH environment variable is required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_Success(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "TG_PHONE", "+1234567890")
	setEnv(t, "LOG_LEVEL", "debug")
	setEnv(t, "RATE_LIMIT_MS", "500")
	setEnv(t, "TG_CONNECT_TIMEOUT_SECONDS", "45")
	setEnv(t, "HISTORY_DELAY_MIN_MS", "2500")
	setEnv(t, "HISTORY_DELAY_MAX_MS", "4500")
	setEnv(t, "FLOOD_WAIT_MAX_SECONDS", "600")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramAppID != 12345 {
		t.Errorf("expected AppID 12345, got %d", cfg.TelegramAppID)
	}
	if cfg.TelegramAppHash != "testhash" {
		t.Errorf("expected AppHash 'testhash', got %s", cfg.TelegramAppHash)
	}
	if cfg.Phone != "+1234567890" {
		t.Errorf("expected Phone '+1234567890', got %s", cfg.Phone)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %s", cfg.LogLevel)
	}
	if cfg.RateLimitMs != 500 {
		t.Errorf("expected RateLimitMs 500, got %d", cfg.RateLimitMs)
	}
	if cfg.TelegramConnectTimeoutSeconds != 45 {
		t.Errorf("expected TelegramConnectTimeoutSeconds 45, got %d", cfg.TelegramConnectTimeoutSeconds)
	}
	if cfg.HistoryDelayMinMs != 2500 {
		t.Errorf("expected HistoryDelayMinMs 2500, got %d", cfg.HistoryDelayMinMs)
	}
	if cfg.HistoryDelayMaxMs != 4500 {
		t.Errorf("expected HistoryDelayMaxMs 4500, got %d", cfg.HistoryDelayMaxMs)
	}
	if cfg.FloodWaitMaxSeconds != 600 {
		t.Errorf("expected FloodWaitMaxSeconds 600, got %d", cfg.FloodWaitMaxSeconds)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	unsetEnv(t, "TG_PHONE")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "RATE_LIMIT_MS")
	unsetEnv(t, "TG_CONNECT_TIMEOUT_SECONDS")
	unsetEnv(t, "HISTORY_DELAY_MIN_MS")
	unsetEnv(t, "HISTORY_DELAY_MAX_MS")
	unsetEnv(t, "FLOOD_WAIT_MAX_SECONDS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Phone != "" {
		t.Errorf("expected empty Phone, got %s", cfg.Phone)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default LogLevel 'info', got %s", cfg.LogLevel)
	}
	if cfg.RateLimitMs != 350 {
		t.Errorf("expected default RateLimitMs 350, got %d", cfg.RateLimitMs)
	}
	if cfg.TelegramConnectTimeoutSeconds != 60 {
		t.Errorf("expected default TelegramConnectTimeoutSeconds 60, got %d", cfg.TelegramConnectTimeoutSeconds)
	}
	if cfg.HistoryDelayMinMs != 2000 {
		t.Errorf("expected default HistoryDelayMinMs 2000, got %d", cfg.HistoryDelayMinMs)
	}
	if cfg.HistoryDelayMaxMs != 4000 {
		t.Errorf("expected default HistoryDelayMaxMs 4000, got %d", cfg.HistoryDelayMaxMs)
	}
	if cfg.FloodWaitMaxSeconds != 900 {
		t.Errorf("expected default FloodWaitMaxSeconds 900, got %d", cfg.FloodWaitMaxSeconds)
	}
}

func TestLoad_InvalidRateLimitFallsBackToDefault(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "RATE_LIMIT_MS", "not_a_number")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RateLimitMs != 350 {
		t.Errorf("expected default RateLimitMs 350 on invalid input, got %d", cfg.RateLimitMs)
	}
}

func TestLoad_InvalidSafetyLimitsFallBackToDefaults(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "HISTORY_DELAY_MIN_MS", "not_a_number")
	setEnv(t, "HISTORY_DELAY_MAX_MS", "-1")
	setEnv(t, "FLOOD_WAIT_MAX_SECONDS", "0")
	setEnv(t, "TG_CONNECT_TIMEOUT_SECONDS", "-1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HistoryDelayMinMs != 2000 {
		t.Errorf("expected default HistoryDelayMinMs 2000, got %d", cfg.HistoryDelayMinMs)
	}
	if cfg.HistoryDelayMaxMs != 4000 {
		t.Errorf("expected default HistoryDelayMaxMs 4000, got %d", cfg.HistoryDelayMaxMs)
	}
	if cfg.FloodWaitMaxSeconds != 900 {
		t.Errorf("expected default FloodWaitMaxSeconds 900, got %d", cfg.FloodWaitMaxSeconds)
	}
	if cfg.TelegramConnectTimeoutSeconds != 60 {
		t.Errorf("expected default TelegramConnectTimeoutSeconds 60, got %d", cfg.TelegramConnectTimeoutSeconds)
	}
}

func TestLoad_SwapsReversedHistoryDelayRange(t *testing.T) {
	setEnv(t, "TG_APP_ID", "12345")
	setEnv(t, "TG_APP_HASH", "testhash")
	setEnv(t, "HISTORY_DELAY_MIN_MS", "5000")
	setEnv(t, "HISTORY_DELAY_MAX_MS", "2000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HistoryDelayMinMs != 2000 {
		t.Errorf("expected swapped HistoryDelayMinMs 2000, got %d", cfg.HistoryDelayMinMs)
	}
	if cfg.HistoryDelayMaxMs != 5000 {
		t.Errorf("expected swapped HistoryDelayMaxMs 5000, got %d", cfg.HistoryDelayMaxMs)
	}
}
