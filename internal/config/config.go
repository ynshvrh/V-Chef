package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	GeminiAPIKey string
	OpenAIAPIKey string
	Environment  string
}

func Load() *Config {
	port := getEnv("PORT", "8085")
	geminiKey := getEnv("GEMINI_API_KEY", "")
	openAIKey := getEnv("OPENAI_API_KEY", "")
	env := getEnv("ENV", "development")

	return &Config{
		Port:         port,
		GeminiAPIKey: geminiKey,
		OpenAIAPIKey: openAIKey,
		Environment:  env,
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
