package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port             string
	OpenRouterAPIKey string
	OpenRouterModels []string
	GeminiAPIKey     string
	Environment      string
}

func Load() *Config {
	port := getEnv("PORT", "8085")
	openRouterKey := getEnvAny("OPENROUTER_API_KEY", "OpenRouter__ApiKey")
	geminiKey := getEnvAny("GEMINI_API_KEY", "Gemini__ApiKey")
	env := getEnv("ENV", "development")

	// Parse fallback models list
	models := parseOpenRouterModels()

	return &Config{
		Port:             port,
		OpenRouterAPIKey: openRouterKey,
		OpenRouterModels: models,
		GeminiAPIKey:     geminiKey,
		Environment:      env,
	}
}

func parseOpenRouterModels() []string {
	var models []string

	// Check indexed env vars: OpenRouter__Models__0, OpenRouter__Models__1, etc.
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("OpenRouter__Models__%d", i)
		if val, exists := os.LookupEnv(key); exists && strings.TrimSpace(val) != "" {
			models = append(models, strings.TrimSpace(val))
		}
	}

	// Check comma-separated OPENROUTER_MODELS
	if len(models) == 0 {
		if rawModels := getEnvAny("OPENROUTER_MODELS", "OpenRouter__Models"); rawModels != "" {
			for _, m := range strings.Split(rawModels, ",") {
				if trimmed := strings.TrimSpace(m); trimmed != "" {
					models = append(models, trimmed)
				}
			}
		}
	}

	// Check single model fallback OPENROUTER_MODEL / OpenRouter__Model
	if len(models) == 0 {
		if singleModel := getEnvAny("OPENROUTER_MODEL", "OpenRouter__Model"); singleModel != "" {
			models = append(models, singleModel)
		}
	}

	// Default fallback list (Free models first, paid model as last fallback)
	if len(models) == 0 {
		models = []string{
			"meta-llama/llama-3.3-70b-instruct:free",
			"qwen/qwen3-next-80b-a3b-instruct:free",
			"google/gemma-4-31b-it:free",
			"google/gemini-2.5-flash",
		}
	}

	return models
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvAny(keys ...string) string {
	for _, k := range keys {
		if val, exists := os.LookupEnv(k); exists && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}
