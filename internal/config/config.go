package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port             string
	OpenRouterAPIKey string
	OpenRouterModel  string
	GeminiAPIKey     string
	Environment      string
}

func Load() *Config {
	port := getEnv("PORT", "8085")
	openRouterKey := getEnv("OPENROUTER_API_KEY", "")
	openRouterModel := getEnv("OPENROUTER_MODEL", "google/gemini-2.5-flash")
	geminiKey := getEnv("GEMINI_API_KEY", "")
	env := getEnv("ENV", "development")

	return &Config{
		Port:             port,
		OpenRouterAPIKey: openRouterKey,
		OpenRouterModel:  openRouterModel,
		GeminiAPIKey:     geminiKey,
		Environment:      env,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if valueStr, exists := os.LookupEnv(key); exists {
		if val, err := strconv.Atoi(valueStr); err == nil {
			return val
		}
	}
	return fallback
}
