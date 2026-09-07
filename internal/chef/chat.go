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

	"github.com/ynshvrh/V-Chef/internal/models"
)

func (s *Service) Chat(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	if strings.TrimSpace(req.Message) == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}

	// 1. OpenRouter API integration with fallback
	if s.cfg.OpenRouterAPIKey != "" {
		resp, err := s.chatWithOpenRouter(ctx, req)
		if err == nil {
			return resp, nil
		}
	}

	// 2. Direct Gemini API integration (Secondary)
	if s.cfg.GeminiAPIKey != "" {
		resp, err := s.chatWithGemini(ctx, req)
		if err == nil {
			return resp, nil
		}
	}

	// 3. Heuristic offline chat fallback
	return s.chatFallback(req), nil
}

func buildChatSystemPrompt(req models.ChatRequest) string {
	lang := strings.ToLower(strings.TrimSpace(req.Language))
	if lang == "" {
		lang = "uk"
	}

	langInstruction := "Answer strictly in Ukrainian language (українською мовою)."
	if lang == "en" {
		langInstruction = "Answer strictly in English language."
	}

	var inventoryStr string
	if len(req.Inventory) > 0 {
		inventoryStr = strings.Join(req.Inventory, ", ")
	} else {
		inventoryStr = "Fridge is currently empty"
	}

	return fmt.Sprintf(`You are V-Chef, an executive AI culinary assistant and smart kitchen advisor.
%s

GASTRONOMIC & OPERATIONAL RULES:
1. Provide helpful, conversational, culinary advice, answer cooking questions, and propose realistic meals.
2. If suggesting a recipe, populate the "recipe" object with full macros and ingredients.
3. STRICT INGREDIENT SCHEMA:
   - "name": Pure product name only (e.g. "Морква", "Куряче філе", "Carrot", "Flour"). Never prefix with numbers or units!
   - "quantity": Numeric quantity (e.g. 1, 200, 0.5).
   - "unit": Standard unit ("шт", "г", "кг", "мл", "л", "ст. л.", "ч. л.", "дрібка", "зубчик" or "pcs", "g", "kg", "ml", "l", "tbsp", "tsp", "pinch", "clove").
   - "in_fridge": boolean. Set to true if already in user's fridge inventory, false if missing.
4. If ingredients are missing to cook the suggested recipe, list them in "shopping_suggestions".
5. If the user only asks a question (or gives a greeting) and does NOT ask for a recipe, set "recipe" to null and "shopping_suggestions" to empty array.

USER CONTEXT:
- Available fridge inventory: %s
- Preferred cuisine: %s
- Dietary profile/restrictions: %s

OUTPUT FORMAT:
Return ONLY a valid JSON object matching this exact schema:
{
  "reply": "Friendly conversational answer, tips, or explanation",
  "recipe": {
    "title": "Dish Name",
    "description": "Appetizing description",
    "prep_time_mins": 10,
    "cook_time_mins": 20,
    "servings": 2,
    "calories": 450,
    "protein_grams": 25.0,
    "fat_grams": 15.0,
    "carbs_grams": 40.0,
    "ingredients": [
      {"name": "Морква", "quantity": 1, "unit": "шт", "in_fridge": true}
    ],
    "steps": ["Step 1...", "Step 2..."]
  },
  "shopping_suggestions": [
    {"name": "Сметана", "quantity": 1, "unit": "уп", "in_fridge": false}
  ]
}`,
		langInstruction,
		inventoryStr,
		req.CuisinePreference,
		req.DietaryProfile,
	)
}

func (s *Service) chatWithOpenRouter(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"

	systemPrompt := buildChatSystemPrompt(req)

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}

	for _, h := range req.History {
		role := h.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		messages = append(messages, map[string]string{
			"role":    role,
			"content": h.Content,
		})
	}

	messages = append(messages, map[string]string{
		"role":    "user",
		"content": req.Message,
	})

	modelsList := s.cfg.OpenRouterModels
	if len(modelsList) > 3 {
		modelsList = modelsList[:3]
	}

	payload := map[string]any{
		"models": modelsList,
		"response_format": map[string]string{
			"type": "json_object",
		},
		"messages":    messages,
		"temperature": 0.7,
	}

	if len(modelsList) == 1 {
		payload["model"] = modelsList[0]
		delete(payload, "models")
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenRouter chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.OpenRouterAPIKey))
	httpReq.Header.Set("HTTP-Referer", "https://v-fridge.app")
	httpReq.Header.Set("X-Title", "V-Fridge")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenRouter chat API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openRouterResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openRouterResp); err != nil {
		return nil, fmt.Errorf("failed to decode OpenRouter chat response: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices array from OpenRouter chat")
	}

	return s.parseChatJson(openRouterResp.Choices[0].Message.Content)
}

func (s *Service) chatWithGemini(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", s.cfg.GeminiAPIKey)

	systemPrompt := buildChatSystemPrompt(req)
	var promptBuilder strings.Builder
	promptBuilder.WriteString("System: " + systemPrompt + "\n\n")

	for _, h := range req.History {
		promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", h.Role, h.Content))
	}
	promptBuilder.WriteString("User: " + req.Message)

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": promptBuilder.String()},
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
		return nil, fmt.Errorf("failed to marshal Gemini chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat request to Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gemini chat API error (status %d): %s", resp.StatusCode, string(respBody))
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
		return nil, fmt.Errorf("failed to decode Gemini chat response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty candidates array from Gemini chat")
	}

	return s.parseChatJson(geminiResp.Candidates[0].Content.Parts[0].Text)
}

func (s *Service) parseChatJson(rawJson string) (*models.ChatResponse, error) {
	jsonText := strings.TrimSpace(rawJson)
	if strings.HasPrefix(jsonText, "```json") {
		jsonText = strings.TrimPrefix(jsonText, "```json")
		jsonText = strings.TrimSuffix(jsonText, "```")
		jsonText = strings.TrimSpace(jsonText)
	} else if strings.HasPrefix(jsonText, "```") {
		jsonText = strings.TrimPrefix(jsonText, "```")
		jsonText = strings.TrimSuffix(jsonText, "```")
		jsonText = strings.TrimSpace(jsonText)
	}

	var chatResp models.ChatResponse
	if err := json.Unmarshal([]byte(jsonText), &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse chat response JSON: %w", err)
	}

	if chatResp.Recipe != nil {
		chatResp.Recipe.GeneratedAt = time.Now().UTC()
		chatResp.Recipe = NormalizeRecipe(chatResp.Recipe)
	}

	for i := range chatResp.ShoppingSuggestions {
		chatResp.ShoppingSuggestions[i] = NormalizeIngredient(chatResp.ShoppingSuggestions[i])
	}

	return &chatResp, nil
}

func (s *Service) chatFallback(req models.ChatRequest) *models.ChatResponse {
	isEn := strings.EqualFold(strings.TrimSpace(req.Language), "en")

	reply := fmt.Sprintf("Привіт! Я V-Chef. Я почув ваше повідомлення: «%s». Чим ще я можу допомогти вам на кухні?", req.Message)
	if isEn {
		reply = fmt.Sprintf("Hello! I am V-Chef. I received your message: \"%s\". How can I assist with your cooking today?", req.Message)
	}

	return &models.ChatResponse{
		Reply:               reply,
		Recipe:              nil,
		ShoppingSuggestions: nil,
	}
}
