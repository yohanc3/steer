package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config contains runtime environment variables.
type Config struct {
	DatabaseURL                 string        `env:"DATABASE_URL,required"`
	DatabaseBackupDirectory     string        `env:"DATABASE_BACKUP_DIRECTORY" envDefault:"/backups"`
	PublicBaseURL               string        `env:"PUBLIC_BASE_URL,required"`
	TelegramBotToken            string        `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramDeliveryInterval    time.Duration `env:"TELEGRAM_DELIVERY_INTERVAL" envDefault:"30s"`
	TelegramDeliveryConcurrency int           `env:"TELEGRAM_DELIVERY_CONCURRENCY" envDefault:"5"`
	TellerApplicationID         string        `env:"TELLER_APPLICATION_ID,required"`
	TellerEnvironment           string        `env:"TELLER_ENVIRONMENT,required"`
	TellerCertPEM               string        `env:"TELLER_CERT_PEM,required"`
	TellerKeyPEM                string        `env:"TELLER_KEY_PEM,required"`
	TellerTokenSigningPublicKey string        `env:"TELLER_TOKEN_SIGNING_PUBLIC_KEY,required"`
	TokenEncryptionKey          string        `env:"TOKEN_ENCRYPTION_KEY,required"`
	TellerPollInterval          time.Duration `env:"TELLER_POLL_INTERVAL" envDefault:"30m"`
	TellerQueueRefreshInterval  time.Duration `env:"TELLER_QUEUE_REFRESH_INTERVAL" envDefault:"5m"`
	TellerSyncLookbackDays      int           `env:"TELLER_SYNC_LOOKBACK_DAYS" envDefault:"10"`
	DatabaseBackupInterval      time.Duration `env:"DATABASE_BACKUP_INTERVAL" envDefault:"24h"`
}

// LoadConfig parses Config from process environment variables.
func LoadConfig() (*Config, error) {
	// Parse environment variables once into the application's typed configuration.
	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse environment: %w", err)
	}
	if cfg.TellerPollInterval <= 0 {
		return nil, fmt.Errorf("TELLER_POLL_INTERVAL must be greater than zero")
	}
	if cfg.TelegramDeliveryInterval <= 0 {
		return nil, fmt.Errorf("TELEGRAM_DELIVERY_INTERVAL must be greater than zero")
	}
	if cfg.TelegramDeliveryConcurrency <= 0 {
		return nil, fmt.Errorf("TELEGRAM_DELIVERY_CONCURRENCY must be greater than zero")
	}
	if cfg.TellerQueueRefreshInterval <= 0 {
		return nil, fmt.Errorf("TELLER_QUEUE_REFRESH_INTERVAL must be greater than zero")
	}
	if cfg.TellerSyncLookbackDays <= 0 {
		return nil, fmt.Errorf("TELLER_SYNC_LOOKBACK_DAYS must be greater than zero")
	}
	return &cfg, nil
}
