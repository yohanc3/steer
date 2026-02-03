package config

import (
	"fmt"
	"os"
)

var Cfg *Config

type Config struct {
    Auth0Domain   string
    Auth0Audience string
	DatabaseURL string
	TelegramBotToken string
	ServerURL string
}

func LoadConfig() (error) {
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

	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramBotToken == "" {
		 return fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable required")
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		return fmt.Errorf("SERVER_URL environment variable required")
	}

 	Cfg = &Config{
        Auth0Domain:   domain,
        Auth0Audience: audience,
		DatabaseURL: dbUrl,
		TelegramBotToken: telegramBotToken,
		ServerURL: serverURL,
    }

	return nil
}


