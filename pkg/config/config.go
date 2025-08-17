package config

import (
	"os"
	"strconv"
)

type Config struct {
	TelegramToken string
	AdminTG       int64
	HHLogin       string
	HHPassword    string
	Timezone      string
	Proxy         string
}

func Load() *Config {
	return &Config{
		TelegramToken: getEnv("TELEGRAM_TOKEN", ""),
		AdminTG:       getEnvInt64("ADMIN_TG", 0),
		HHLogin:       getEnv("HH_LOGIN", ""),
		HHPassword:    getEnv("HH_PASSWORD", ""),
		Timezone:      getEnv("TZ", "Europe/Moscow"),
		Proxy:         getEnv("PROXY", "None"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}