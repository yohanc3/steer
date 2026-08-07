package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config contains runtime environment variables.
type Config struct {
	DatabaseURL                 string   `env:"DATABASE_URL,required"`
	PublicBaseURL               string   `env:"PUBLIC_BASE_URL,required"`
	TelegramBotToken            string   `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramWebhookSecret       string   `env:"TELEGRAM_WEBHOOK_SECRET,required"`
	TellerApplicationID         string   `env:"TELLER_APPLICATION_ID,required"`
	TellerEnvironment           string   `env:"TELLER_ENVIRONMENT,required"`
	TellerCertPEM               string   `env:"TELLER_CERT_PEM,required"`
	TellerKeyPEM                string   `env:"TELLER_KEY_PEM,required"`
	TellerTokenSigningPublicKey string   `env:"TELLER_TOKEN_SIGNING_PUBLIC_KEY,required"`
	TellerWebhookSecrets        []string `env:"TELLER_WEBHOOK_SECRETS,required"`
	TokenEncryptionKey          string   `env:"TOKEN_ENCRYPTION_KEY,required"`
	DeepSeekAPIKey              string   `env:"DEEPSEEK_API_KEY,required"`
	DeepSeekBaseURL             string   `env:"DEEPSEEK_BASE_URL" envDefault:"https://api.deepseek.com"`
	DeepSeekModel               string   `env:"DEEPSEEK_MODEL" envDefault:"deepseek-v4-pro"`
}

// LoadConfig parses Config from process environment variables.
func LoadConfig() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse environment: %w", err)
	}
	return &cfg, nil
}
