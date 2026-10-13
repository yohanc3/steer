package config

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "test.sqlite")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_DELIVERY_INTERVAL", "20s")
	t.Setenv("TELEGRAM_DELIVERY_CONCURRENCY", "7")
	t.Setenv("TYPESAFE_API_KEY", "typesafe-key")
	t.Setenv("CATEGORIZATION_INTERVAL", "12s")
	t.Setenv("CATEGORIZATION_CONCURRENCY", "4")
	t.Setenv("JEV_CONFIDENCE_THRESHOLD", "0.8")
	t.Setenv("TELLER_APPLICATION_ID", "app")
	t.Setenv("TELLER_ENVIRONMENT", "sandbox")
	t.Setenv("TELLER_CERT_PEM", "cert")
	t.Setenv("TELLER_KEY_PEM", "key")
	t.Setenv("TELLER_TOKEN_SIGNING_PUBLIC_KEY", "public")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("TELLER_POLL_INTERVAL", "15m")
	t.Setenv("TELLER_QUEUE_REFRESH_INTERVAL", "3m")
	t.Setenv("TELLER_SYNC_LOOKBACK_DAYS", "9")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TellerPollInterval != 15*time.Minute {
		t.Fatalf("interval = %s", cfg.TellerPollInterval)
	}
	if cfg.TelegramDeliveryInterval != 20*time.Second {
		t.Fatalf("delivery interval = %s", cfg.TelegramDeliveryInterval)
	}
	if cfg.TelegramDeliveryConcurrency != 7 {
		t.Fatalf("delivery concurrency = %d", cfg.TelegramDeliveryConcurrency)
	}
	if cfg.CategorizationInterval != 12*time.Second || cfg.CategorizationConcurrency != 4 || cfg.JevConfidenceThreshold != 0.8 {
		t.Fatalf("categorization configuration = %s, %d, %f", cfg.CategorizationInterval, cfg.CategorizationConcurrency, cfg.JevConfidenceThreshold)
	}
	if cfg.TellerQueueRefreshInterval != 3*time.Minute {
		t.Fatalf("queue refresh interval = %s", cfg.TellerQueueRefreshInterval)
	}
	if cfg.TellerSyncLookbackDays != 9 {
		t.Fatalf("lookback days = %d", cfg.TellerSyncLookbackDays)
	}
}

func TestLoadConfigRejectsInvalidPollingConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "test.sqlite")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TYPESAFE_API_KEY", "typesafe-key")
	t.Setenv("TELLER_APPLICATION_ID", "app")
	t.Setenv("TELLER_ENVIRONMENT", "sandbox")
	t.Setenv("TELLER_CERT_PEM", "cert")
	t.Setenv("TELLER_KEY_PEM", "key")
	t.Setenv("TELLER_TOKEN_SIGNING_PUBLIC_KEY", "public")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("TELLER_QUEUE_REFRESH_INTERVAL", "0s")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid queue refresh interval to fail")
	}
}

func TestLoadConfigRejectsInvalidDeliveryConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "test.sqlite")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TYPESAFE_API_KEY", "typesafe-key")
	t.Setenv("TELLER_APPLICATION_ID", "app")
	t.Setenv("TELLER_ENVIRONMENT", "sandbox")
	t.Setenv("TELLER_CERT_PEM", "cert")
	t.Setenv("TELLER_KEY_PEM", "key")
	t.Setenv("TELLER_TOKEN_SIGNING_PUBLIC_KEY", "public")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("TELEGRAM_DELIVERY_CONCURRENCY", "0")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid delivery concurrency to fail")
	}
}

func TestLoadConfigRejectsInvalidCategorizationConfiguration(t *testing.T) {
	t.Setenv("DATABASE_URL", "test.sqlite")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TYPESAFE_API_KEY", "typesafe-key")
	t.Setenv("TELLER_APPLICATION_ID", "app")
	t.Setenv("TELLER_ENVIRONMENT", "sandbox")
	t.Setenv("TELLER_CERT_PEM", "cert")
	t.Setenv("TELLER_KEY_PEM", "key")
	t.Setenv("TELLER_TOKEN_SIGNING_PUBLIC_KEY", "public")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("JEV_CONFIDENCE_THRESHOLD", "1.1")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected invalid Jev confidence threshold to fail")
	}
}
