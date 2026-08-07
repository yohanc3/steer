package config

import "testing"

func setRequiredEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DATABASE_URL":                    "steer.sqlite",
		"PUBLIC_BASE_URL":                 "https://steer.example/",
		"TELEGRAM_BOT_TOKEN":              "telegram-token",
		"TELEGRAM_WEBHOOK_SECRET":         "telegram-secret",
		"TELLER_APPLICATION_ID":           "app_test",
		"TELLER_ENVIRONMENT":              "sandbox",
		"TELLER_CERT_PEM":                 "/run/secrets/teller-cert.pem",
		"TELLER_KEY_PEM":                  "/run/secrets/teller-key.pem",
		"TELLER_TOKEN_SIGNING_PUBLIC_KEY": "public-key",
		"TELLER_WEBHOOK_SECRETS":          "old-secret,new-secret",
		"TOKEN_ENCRYPTION_KEY":            "encryption-key",
		"DEEPSEEK_API_KEY":                "deepseek-token",
	}
	for name, value := range values {
		t.Setenv(name, value)
	}
}

func TestLoadConfig(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("DEEPSEEK_BASE_URL", "https://deepseek.example/")
	t.Setenv("DEEPSEEK_MODEL", "custom-model")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.PublicBaseURL != "https://steer.example/" {
		t.Fatalf("PublicBaseURL = %q", cfg.PublicBaseURL)
	}
	if len(cfg.TellerWebhookSecrets) != 2 {
		t.Fatalf("TellerWebhookSecrets count = %d", len(cfg.TellerWebhookSecrets))
	}
	if cfg.DeepSeekBaseURL != "https://deepseek.example/" {
		t.Fatalf("DeepSeekBaseURL = %q", cfg.DeepSeekBaseURL)
	}
	if cfg.DeepSeekModel != "custom-model" {
		t.Fatalf("DeepSeekModel = %q", cfg.DeepSeekModel)
	}
}

func TestLoadConfigUsesDefaults(t *testing.T) {
	setRequiredEnvironment(t)
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.DeepSeekBaseURL != "https://api.deepseek.com" {
		t.Fatalf("DeepSeekBaseURL = %q", cfg.DeepSeekBaseURL)
	}
	if cfg.DeepSeekModel != "deepseek-v4-pro" {
		t.Fatalf("DeepSeekModel = %q", cfg.DeepSeekModel)
	}
}
