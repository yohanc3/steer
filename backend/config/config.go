package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Global config
var Cfg *Config

type Config struct {
	Auth0Domain      string
	Auth0Audience    string
	DatabaseURL      string
	TelegramBotToken string
	TelegramEnabled  bool
	ServerURL        string
	TellerCertPem    string
	TellerKeyPem     string
	AllowedOrigins   []string
}

// Loads environment variables and initializes them in the global Config object.
//
// Implementation details: There's probably a better way to implement this, where
// we only define the env variable names and they're automatically verified and
// mapped to our global config object.
//
// Note: This loads up a global struct, which can technically be modified
// elsewhere. Low priority, and not yet fixed if this comment still exists.
func LoadConfig() error {
	domain := os.Getenv("AUTH0_DOMAIN")
	if domain == "" {
		return fmt.Errorf("AUTH0_DOMAIN environment variable required")
	}

	audience := os.Getenv("AUTH0_AUDIENCE")
	if audience == "" {
		return fmt.Errorf("AUTH0_AUDIENCE environment variable required")
	}

	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		return fmt.Errorf("DATABASE_URL environment variable required")
	}

	telegramEnabled := true
	if rawTelegramEnabled := os.Getenv("TELEGRAM_ENABLED"); rawTelegramEnabled != "" {
		var err error
		telegramEnabled, err = strconv.ParseBool(rawTelegramEnabled)
		if err != nil {
			return fmt.Errorf("TELEGRAM_ENABLED must be true or false: %w", err)
		}
	}

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	serverURL := os.Getenv("SERVER_URL")
	if telegramEnabled {
		if telegramBotToken == "" {
			return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable required when Telegram is enabled")
		}
		if serverURL == "" {
			return fmt.Errorf("SERVER_URL environment variable required when Telegram is enabled")
		}
	}

	allowedOrigins := splitCSV(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:5173"}
	}

	Cfg = &Config{
		Auth0Domain:      domain,
		Auth0Audience:    audience,
		DatabaseURL:      dbUrl,
		TelegramBotToken: telegramBotToken,
		TelegramEnabled:  telegramEnabled,
		ServerURL:        serverURL,
		TellerCertPem:    os.Getenv("TELLER_CERT_PEM"),
		TellerKeyPem:     os.Getenv("TELLER_KEY_PEM"),
		AllowedOrigins:   allowedOrigins,
	}

	return nil
}

func splitCSV(value string) []string {
	var values []string
	for item := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
