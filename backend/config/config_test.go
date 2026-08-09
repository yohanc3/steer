package config

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "test.sqlite")
	t.Setenv("PUBLIC_BASE_URL", "https://example.test")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "secret")
	t.Setenv("TELLER_APPLICATION_ID", "app")
	t.Setenv("TELLER_ENVIRONMENT", "sandbox")
	t.Setenv("TELLER_CERT_PEM", "cert")
	t.Setenv("TELLER_KEY_PEM", "key")
	t.Setenv("TELLER_TOKEN_SIGNING_PUBLIC_KEY", "public")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "01234567890123456789012345678901")
	t.Setenv("TELLER_POLL_INTERVAL", "15m")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TellerPollInterval != 15*time.Minute {
		t.Fatalf("interval = %s", cfg.TellerPollInterval)
	}
}
