package chef

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

type Generator interface {
	GenerateRecipe(ctx context.Context, req models.GenerateRecipeRequest) (*models.RecipeResponse, error)
}

type Service struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}
}

func (s *Service) GenerateRecipe(ctx context.Context, req models.GenerateRecipeRequest) (*models.RecipeResponse, error) {
	if len(req.Ingredients) == 0 {
		return nil, fmt.Errorf("ingredients list cannot be empty")
	}

	// 1. OpenRouter API integration with dual-layer fallback
	if s.cfg.OpenRouterAPIKey != "" {
		recipe, err := s.generateWithOpenRouter(ctx, req)
		if err == nil {
			return recipe, nil
		}
		// If OpenRouter failed for any reason, try single model retries before giving up
		if fallbackRecipe, fallbackErr := s.generateWithOpenRouterSequential(ctx, req); fallbackErr == nil {
			return fallbackRecipe, nil
		}
	}

	// 2. Direct Gemini API integration (Secondary)
	if s.cfg.GeminiAPIKey != "" {
		return s.generateWithGemini(ctx, req)
	}

	// 3. Fallback to heuristic recipe generator when no AI API key is configured
	return s.generateFallback(req), nil
}

func (s *Service) generateWithOpenRouter(ctx context.Context, req models.GenerateRecipeRequest) (*models.RecipeResponse, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"

	systemPrompt := `You are V-Chef, an expert culinary AI assistant. Return ONLY a valid JSON object matching this schema without markdown code blocks, backticks, or extra text:
{
  "title": "string",
  "description": "string",
  "prep_time_mins": 10,
  "cook_time_mins": 20,
  "servings": 2,
  "calories": 450,
  "protein_grams": 25.5,
  "fat_grams": 15.0,
  "carbs_grams": 40.0,
  "ingredients": [
    {"name": "Ingredient Name", "quantity": 100, "unit": "g", "in_fridge": true}
  ],
  "steps": [
    "Step 1...", "Step 2..."
  ]
}`

	userPrompt := fmt.Sprintf(`Available ingredients in fridge: %s
Meal type: %s
Dietary preference: %s
Max prep time: %d mins
Target calories: %d`,
		strings.Join(req.Ingredients, ", "),
		req.MealType,
		req.DietaryCategory,
		req.MaxPrepTimeMins,
		req.TargetCalories,
	)

	// Ensure max 3 models in the array as required by OpenRouter API schema
	modelsList := s.cfg.OpenRouterModels
	if len(modelsList) > 3 {
		modelsList = modelsList[:3]
	}

	payload := map[string]any{
		"models": modelsList,
		"response_format": map[string]string{
			"type": "json_object",
		},
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.7,
	}

	if len(modelsList) == 1 {
		payload["model"] = modelsList[0]
		delete(payload, "models")
	}

	return s.sendOpenRouterPayload(ctx, url, payload)
}

func (s *Service) generateWithOpenRouterSequential(ctx context.Context, req models.GenerateRecipeRequest) (*models.RecipeResponse, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"

	systemPrompt := `You are V-Chef, an expert culinary AI assistant. Return ONLY a valid JSON object matching this schema without markdown code blocks, backticks, or extra text:
{
  "title": "string",
  "description": "string",
  "prep_time_mins": 10,
  "cook_time_mins": 20,
  "servings": 2,
  "calories": 450,
  "protein_grams": 25.5,
  "fat_grams": 15.0,
  "carbs_grams": 40.0,
  "ingredients": [
    {"name": "Ingredient Name", "quantity": 100, "unit": "g", "in_fridge": true}
  ],
  "steps": [
    "Step 1...", "Step 2..."
  ]
}`

	userPrompt := fmt.Sprintf(`Available ingredients in fridge: %s
Meal type: %s
Dietary preference: %s
Max prep time: %d mins
Target calories: %d`,
		strings.Join(req.Ingredients, ", "),
		req.MealType,
		req.DietaryCategory,
		req.MaxPrepTimeMins,
		req.TargetCalories,
	)

	var lastErr error
	for _, modelName := range s.cfg.OpenRouterModels {
		payload := map[string]any{
			"model": modelName,
			"response_format": map[string]string{
				"type": "json_object",
			},
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
			"temperature": 0.7,
		}

		recipe, err := s.sendOpenRouterPayload(ctx, url, payload)
		if err == nil {
			return recipe, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("all sequential model retries failed: %w", lastErr)
}

func (s *Service) sendOpenRouterPayload(ctx context.Context, url string, payload map[string]any) (*models.RecipeResponse, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenRouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.OpenRouterAPIKey))
	httpReq.Header.Set("HTTP-Referer", "https://v-fridge.app")
	httpReq.Header.Set("X-Title", "V-Fridge")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openRouterResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 || openRouterResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty response from OpenRouter API")
	}

	jsonText := strings.TrimSpace(openRouterResp.Choices[0].Message.Content)
	jsonText = strings.TrimPrefix(jsonText, "```json")
	jsonText = strings.TrimPrefix(jsonText, "```")
	jsonText = strings.TrimSuffix(jsonText, "```")
	jsonText = strings.TrimSpace(jsonText)

	var recipe models.RecipeResponse
	if err := json.Unmarshal([]byte(jsonText), &recipe); err != nil {
		return nil, fmt.Errorf("failed to parse OpenRouter JSON recipe from model (%s): %w", openRouterResp.Model, err)
	}

	recipe.GeneratedAt = time.Now().UTC()
	return &recipe, nil
}

func (s *Service) generateWithGemini(ctx context.Context, req models.GenerateRecipeRequest) (*models.RecipeResponse, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", s.cfg.GeminiAPIKey)

	prompt := fmt.Sprintf(`System: You are V-Chef, an expert culinary assistant. Return ONLY a valid JSON object matching this schema, without any markdown formatting or commentary:
{
  "title": "string",
  "description": "string",
  "prep_time_mins": 10,
  "cook_time_mins": 20,
  "servings": 2,
  "calories": 450,
  "protein_grams": 25.5,
  "fat_grams": 15.0,
  "carbs_grams": 40.0,
  "ingredients": [
    {"name": "Ingredient Name", "quantity": 100, "unit": "g", "in_fridge": true}
  ],
  "steps": [
    "Step 1...", "Step 2..."
  ]
}

User request:
Available ingredients in fridge: %s
Meal type: %s
Dietary preference: %s
Max prep time: %d mins
Target calories: %d`,
		strings.Join(req.Ingredients, ", "),
		req.MealType,
		req.DietaryCategory,
		req.MaxPrepTimeMins,
		req.TargetCalories,
	)

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0.7,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Gemini request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode Gemini response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini API")
	}

	jsonText := geminiResp.Candidates[0].Content.Parts[0].Text

	var recipe models.RecipeResponse
	if err := json.Unmarshal([]byte(jsonText), &recipe); err != nil {
		return nil, fmt.Errorf("failed to parse AI JSON recipe: %w", err)
	}

	recipe.GeneratedAt = time.Now().UTC()
	return &recipe, nil
}

func (s *Service) generateFallback(req models.GenerateRecipeRequest) *models.RecipeResponse {
	title := fmt.Sprintf("Фірмовий %s з %s", strings.Title(req.MealType), strings.Title(req.Ingredients[0]))
	if req.MealType == "" {
		title = fmt.Sprintf("Швидка страва з %s", strings.Title(req.Ingredients[0]))
	}

	recipeIngredients := make([]models.RecipeIngredient, 0, len(req.Ingredients))
	for i, ing := range req.Ingredients {
		recipeIngredients = append(recipeIngredients, models.RecipeIngredient{
			Name:     ing,
			Quantity: float64((i + 1) * 100),
			Unit:     "г",
			InFridge: true,
		})
	}

	return &models.RecipeResponse{
		Title:        title,
		Description:  fmt.Sprintf("Смачна та поживна страва, приготована з продуктів у вашому холодильнику (%s).", strings.Join(req.Ingredients, ", ")),
		PrepTimeMins: 10,
		CookTimeMins: 15,
		Servings:     2,
		Calories:     420,
		ProteinGrams: 22.5,
		FatGrams:     14.0,
		CarbsGrams:   45.0,
		Ingredients:  recipeIngredients,
		Steps: []string{
			"Підготуйте та промийте всі продукти з холодильника.",
			"Наріжте інгредієнти зручними шматочками.",
			"Обсмажте або протушкуйте протягом 15 хвилин до готовності.",
			"Подавайте гарячим та насолоджуйтеся смаком!",
		},
		GeneratedAt: time.Now().UTC(),
	}
}
