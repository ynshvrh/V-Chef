package chef

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ynshvrh/V-Chef/internal/models"
)

func (s *Service) EstimateNutrition(ctx context.Context, req models.NutritionEstimateRequest) (*models.NutritionEstimateResponse, error) {
	if req.DishName == "" && req.Notes != "" {
		req.DishName = req.Notes
	}
	if req.DishName == "" {
		return nil, fmt.Errorf("dish_name is required")
	}

	// 1. Try OpenRouter if configured
	if s.cfg.OpenRouterAPIKey != "" {
		resp, err := s.nutritionWithOpenRouter(ctx, req)
		if err == nil && resp != nil && resp.Calories > 0 {
			resp.Confidence = "ai"
			return resp, nil
		}
	}

	// 2. Try Gemini if configured
	if s.cfg.GeminiAPIKey != "" {
		resp, err := s.nutritionWithGemini(ctx, req)
		if err == nil && resp != nil && resp.Calories > 0 {
			resp.Confidence = "ai"
			return resp, nil
		}
	}

	// 3. Fallback to offline heuristics
	return s.nutritionFallback(req), nil
}

func buildNutritionSystemPrompt(req models.NutritionEstimateRequest) string {
	lang := strings.ToLower(strings.TrimSpace(req.Language))
	langInstruction := "Answer strictly in Ukrainian language (українською мовою)."
	if lang == "en" {
		langInstruction = "Answer strictly in English language."
	}

	qtyInfo := ""
	if req.Quantity > 0 {
		unit := req.Unit
		if unit == "" {
			unit = "г"
		}
		qtyInfo = fmt.Sprintf("Specified portion size: %.2f %s.", req.Quantity, unit)
	}

	return fmt.Sprintf(`You are V-Chef's expert nutritional AI analyst.
%s
Estimate realistic macronutrients and calories for the given food, meal, or portion.

Input Details:
- Food / Dish Name: %s
%s
- Additional Notes: %s

CRITICAL INSTRUCTIONS:
1. Estimate total calories (kcal), protein (g), fat (g), and carbs (g) for the entire consumed portion described.
2. If portion size is not explicitly specified, assume a typical single-person adult serving (e.g., 250-300g for hot meals, 300ml for soups, 1 standard item).
3. If multiple items are listed in the dish name (e.g. '2 котлети і пюре'), sum their nutritional values.
4. Provide a brief, helpful explanation (1-2 sentences) in the notes field.
5. Output STRICTLY a valid JSON object matching this schema:
{
  "food_name": "Clean name of dish/food",
  "quantity": 250,
  "unit": "г",
  "calories": 420,
  "protein": 22.5,
  "fat": 18.0,
  "carbs": 35.0,
  "estimated_weight_g": 250.0,
  "notes": "Орієнтовний розрахунок на порцію 250г..."
}`, langInstruction, req.DishName, qtyInfo, req.Notes)
}

func (s *Service) nutritionWithOpenRouter(ctx context.Context, req models.NutritionEstimateRequest) (*models.NutritionEstimateResponse, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"
	systemPrompt := buildNutritionSystemPrompt(req)

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": fmt.Sprintf("Estimate nutrition for: %s", req.DishName)},
	}

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
		"temperature": 0.3,
	}
	if len(modelsList) == 1 {
		payload["model"] = modelsList[0]
		delete(payload, "models")
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.OpenRouterAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", "https://v-fridge.app")
	httpReq.Header.Set("X-Title", "V-Fridge Nutrition Estimator")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseNutritionChatResponse(raw, req)
}

func (s *Service) nutritionWithGemini(ctx context.Context, req models.NutritionEstimateRequest) (*models.NutritionEstimateResponse, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", s.cfg.GeminiAPIKey)
	systemPrompt := buildNutritionSystemPrompt(req)

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": systemPrompt + "\n\nPlease output only raw JSON."},
				},
			},
		},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"temperature":      0.3,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type geminiResponse struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	var gResp geminiResponse
	if err := json.Unmarshal(raw, &gResp); err != nil {
		return nil, err
	}

	if len(gResp.Candidates) == 0 || len(gResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty gemini response")
	}

	text := gResp.Candidates[0].Content.Parts[0].Text
	text = cleanJSONMarkdown(text)

	var result models.NutritionEstimateResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, err
	}

	finalizeResponse(&result, req)
	return &result, nil
}

func cleanJSONMarkdown(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func parseNutritionChatResponse(raw []byte, req models.NutritionEstimateRequest) (*models.NutritionEstimateResponse, error) {
	type openRouterResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(raw, &orResp); err != nil {
		return nil, err
	}

	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("empty openrouter response")
	}

	text := orResp.Choices[0].Message.Content
	text = cleanJSONMarkdown(text)

	var result models.NutritionEstimateResponse
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, err
	}

	finalizeResponse(&result, req)
	return &result, nil
}

func finalizeResponse(resp *models.NutritionEstimateResponse, req models.NutritionEstimateRequest) {
	if resp.FoodName == "" {
		resp.FoodName = req.DishName
	}
	if resp.Quantity <= 0 {
		if req.Quantity > 0 {
			resp.Quantity = req.Quantity
		} else if resp.EstimatedWeightG > 0 {
			resp.Quantity = resp.EstimatedWeightG
		} else {
			resp.Quantity = 100
		}
	}
	if resp.Unit == "" {
		if req.Unit != "" {
			resp.Unit = req.Unit
		} else {
			resp.Unit = "г"
		}
	}
	if resp.EstimatedWeightG <= 0 {
		resp.EstimatedWeightG = resp.Quantity
	}
}

// Offline Heuristic Nutrition Estimator
func (s *Service) nutritionFallback(req models.NutritionEstimateRequest) *models.NutritionEstimateResponse {
	text := strings.ToLower(req.DishName)

	extractedQty, extractedUnit := extractQuantityAndUnit(text)
	qty := req.Quantity
	unit := req.Unit

	if qty <= 0 && extractedQty > 0 {
		qty = extractedQty
		if unit == "" {
			unit = extractedUnit
		}
	}

	if unit == "" {
		unit = "г"
	}

	// Density per 100g
	cal100, p100, f100, c100 := getMacroDensities(text)

	// Determine weight in grams
	weightG := calculateWeightInGrams(text, qty, unit)

	scale := weightG / 100.0
	calories := int(float64(cal100)*scale + 0.5)
	protein := roundTo1Decimal(p100 * scale)
	fat := roundTo1Decimal(f100 * scale)
	carbs := roundTo1Decimal(c100 * scale)

	if qty <= 0 {
		qty = weightG
		unit = "г"
	}

	foodName := req.DishName
	if foodName == "" {
		foodName = "Страва"
	}

	notes := "Оцінено локально за базовими харчовими стандартами (офлайн-режим)."
	if strings.ToLower(req.Language) == "en" {
		notes = "Estimated locally using standard nutritional baselines (offline mode)."
	}

	return &models.NutritionEstimateResponse{
		FoodName:         foodName,
		Quantity:         qty,
		Unit:             unit,
		Calories:         calories,
		Protein:          protein,
		Fat:              fat,
		Carbs:            carbs,
		EstimatedWeightG: weightG,
		Confidence:       "heuristic",
		Notes:            notes,
	}
}

func getMacroDensities(text string) (cal100 int, p100 float64, f100 float64, c100 float64) {
	switch {
	case strings.Contains(text, "котлет"):
		return 230, 16.0, 15.0, 8.0
	case strings.Contains(text, "куряч") || strings.Contains(text, "курк") || strings.Contains(text, "філе"):
		return 165, 31.0, 3.6, 0.0
	case strings.Contains(text, "свинин") || strings.Contains(text, "ялович") || strings.Contains(text, "стейк") || strings.Contains(text, "м'яс"):
		return 240, 24.0, 16.0, 0.0
	case strings.Contains(text, "риб") || strings.Contains(text, "лосос") || strings.Contains(text, "хек") || strings.Contains(text, "тунець"):
		return 150, 21.0, 7.0, 0.0
	case strings.Contains(text, "яйц") || strings.Contains(text, "яєчн") || strings.Contains(text, "омлет"):
		return 155, 13.0, 11.0, 1.0
	case strings.Contains(text, "борщ"):
		return 55, 3.5, 2.5, 5.5
	case strings.Contains(text, "суп") || strings.Contains(text, "бульйон"):
		return 45, 3.0, 2.0, 4.0
	case strings.Contains(text, "пюре") || strings.Contains(text, "картопл"):
		return 85, 2.0, 2.5, 15.0
	case strings.Contains(text, "вівсян") || strings.Contains(text, "овсянк"):
		return 110, 3.5, 2.5, 18.0
	case strings.Contains(text, "рис") || strings.Contains(text, "гречк") || strings.Contains(text, "каш") || strings.Contains(text, "булгур"):
		return 125, 3.5, 1.0, 26.0
	case strings.Contains(text, "макарон") || strings.Contains(text, "паст") || strings.Contains(text, "спагет"):
		return 140, 5.0, 1.0, 28.0
	case strings.Contains(text, "салат"):
		return 65, 1.5, 4.5, 4.5
	case strings.Contains(text, "творог") || (strings.Contains(text, "сир") && strings.Contains(text, "кисломолоч")):
		return 110, 16.0, 4.0, 3.0
	case strings.Contains(text, "сир") && !strings.Contains(text, "сирник"):
		return 340, 24.0, 26.0, 1.5
	case strings.Contains(text, "хліб") || strings.Contains(text, "батон") || strings.Contains(text, "булк"):
		return 250, 8.0, 2.5, 48.0
	case strings.Contains(text, "яблук") || strings.Contains(text, "банан") || strings.Contains(text, "фрукт"):
		return 65, 1.0, 0.3, 16.0
	case strings.Contains(text, "піц"):
		return 260, 11.0, 10.0, 32.0
	case strings.Contains(text, "бургер") || strings.Contains(text, "шаурм"):
		return 240, 12.0, 11.0, 24.0
	default:
		// Balanced composite meal density
		return 145, 7.0, 6.0, 17.0
	}
}

func calculateWeightInGrams(text string, qty float64, unit string) float64 {
	u := strings.ToLower(strings.TrimSpace(unit))

	if qty <= 0 {
		switch {
		case strings.Contains(text, "борщ") || strings.Contains(text, "суп"):
			return 300
		case strings.Contains(text, "яйц"):
			return 120 // ~2 eggs
		case strings.Contains(text, "яблук") || strings.Contains(text, "банан"):
			return 150
		default:
			return 250 // standard meal portion
		}
	}

	switch u {
	case "кг", "kg":
		return qty * 1000
	case "г", "g":
		return qty
	case "мл", "ml":
		return qty
	case "л", "l":
		return qty * 1000
	case "шт", "pcs":
		switch {
		case strings.Contains(text, "яйц"):
			return qty * 60
		case strings.Contains(text, "котлет"):
			return qty * 80
		case strings.Contains(text, "яблук") || strings.Contains(text, "банан"):
			return qty * 150
		case strings.Contains(text, "хліб") || strings.Contains(text, "шматочок") || strings.Contains(text, "скибк"):
			return qty * 35
		default:
			return qty * 100
		}
	case "порц", "порція", "порцій", "portion", "portions":
		return qty * 250
	default:
		return qty
	}
}

func extractQuantityAndUnit(text string) (float64, string) {
	re := regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*(г|g|кг|kg|мл|ml|л|l|шт|порц)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 3 {
		numStr := strings.Replace(matches[1], ",", ".", 1)
		val, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			return val, matches[2]
		}
	}
	return 0, ""
}

func roundTo1Decimal(val float64) float64 {
	return float64(int(val*10+0.5)) / 10.0
}
