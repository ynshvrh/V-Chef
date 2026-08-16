package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port             string
	GrpcPort         string
	OpenRouterAPIKey string
	OpenRouterModels []string
	GeminiAPIKey     string
	Environment      string
	InternalToken    string
}

func Load() *Config {
	port := getEnv("PORT", "8085")
	grpcPort := getEnv("GRPC_PORT", "50051")
	openRouterKey := getEnvAny("OPENROUTER_API_KEY", "OpenRouter__ApiKey")
	geminiKey := getEnvAny("GEMINI_API_KEY", "Gemini__ApiKey")
	env := getEnv("ENV", "development")
	internalToken := getEnvAny("INTERNAL_TOKEN", "InternalToken", "VChef__InternalToken")

	// Parse fallback models list
	models := parseOpenRouterModels()

	return &Config{
		Port:             port,
		GrpcPort:         grpcPort,
		OpenRouterAPIKey: openRouterKey,
		OpenRouterModels: models,
		GeminiAPIKey:     geminiKey,
		Environment:      env,
		InternalToken:    internalToken,
	}
}

func parseOpenRouterModels() []string {
	var freeModels []string

	// 1. Check indexed env vars: OpenRouter__Models__0, OpenRouter__Models__1, etc.
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("OpenRouter__Models__%d", i)
		if val, exists := os.LookupEnv(key); exists && strings.TrimSpace(val) != "" {
			freeModels = append(freeModels, strings.TrimSpace(val))
		}
	}

	// 2. Check comma-separated OPENROUTER_MODELS
	if len(freeModels) == 0 {
		if rawModels := getEnvAny("OPENROUTER_MODELS", "OpenRouter__Models"); rawModels != "" {
			for _, m := range strings.Split(rawModels, ",") {
				if trimmed := strings.TrimSpace(m); trimmed != "" {
					freeModels = append(freeModels, trimmed)
				}
			}
		}
	}

	// 3. Check single main/fallback model OPENROUTER_MODEL / OpenRouter__Model
	mainModel := getEnvAny("OPENROUTER_MODEL", "OpenRouter__Model")
	if mainModel == "" {
		mainModel = "google/gemini-2.5-flash"
	}

	var finalModels []string
	if len(freeModels) == 0 {
		finalModels = []string{
			"google/gemma-4-31b-it:free",
			"nvidia/nemotron-3-super-120b-a12b:free",
			mainModel,
		}
	} else {
		// OpenRouter API strictly requires max 3 items in the 'models' array.
		// Keep up to 2 free models and place the main paid fallback model as the 3rd item.
		for _, m := range freeModels {
			if !strings.EqualFold(m, mainModel) {
				finalModels = append(finalModels, m)
			}
			if len(finalModels) == 2 {
				break
			}
		}
		finalModels = append(finalModels, mainModel)
	}

	return finalModels
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
