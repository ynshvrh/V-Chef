package chef

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ynshvrh/V-Chef/internal/models"
)

func (s *Service) GenerateMealPlan(ctx context.Context, req models.MealPlanRequest) (*models.MealPlanResponse, error) {
	// Normalize defaults
	if req.Language == "" {
		req.Language = "uk"
	}
	if req.Day == "" {
		req.Day = "Monday"
	}

	// 1. Try OpenRouter
	if s.cfg.OpenRouterAPIKey != "" {
		resp, err := s.mealPlanWithOpenRouter(ctx, req)
		if err == nil && resp != nil && len(resp.Meals) > 0 {
			return resp, nil
		}
	}

	// 2. Try Gemini
	if s.cfg.GeminiAPIKey != "" {
		resp, err := s.mealPlanWithGemini(ctx, req)
		if err == nil && resp != nil && len(resp.Meals) > 0 {
			return resp, nil
		}
	}

	// 3. Robust offline fallback generator
	return s.mealPlanFallback(req), nil
}

func buildMealPlanSystemPrompt(req models.MealPlanRequest) string {
	lang := strings.ToLower(strings.TrimSpace(req.Language))
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

	existingStr := "None"
	if len(req.ExistingMeals) > 0 {
		existingStr = strings.Join(req.ExistingMeals, ", ")
	}

	targetCalStr := "approximately 1800-2200 kcal per day"
	if req.TargetCalories > 0 {
		targetCalStr = fmt.Sprintf("target approximately %d kcal per day", req.TargetCalories)
	}

	mealScope := fmt.Sprintf("Generate 3 balanced meals (breakfast, lunch, dinner) for %s.", req.Day)
	if req.MealType != "" {
		mealScope = fmt.Sprintf("Generate exactly one %s meal for %s.", req.MealType, req.Day)
	}

	return fmt.Sprintf(`You are V-Chef's smart meal planner.
%s
%s (%s).

Context:
- Available Inventory in Fridge: %s
- Cuisine Preference: %s
- Dietary Profile: %s
- Existing Meals to Avoid Repeating: %s

CRITICAL INSTRUCTIONS:
1. Prioritize using products already in the fridge wherever logical.
2. Every ingredient MUST specify a realistic single-portion quantity and standard unit (e.g. '150g куряче філе', '2 яйця', '50g вівсянки', '1 шт банан').
3. NEVER use vague phrases like 'за смаком', 'сіль за смаком', 'спеції до смаку', 'скільки завгодно'. Spices and salt should have concrete minimal amounts (e.g. '2г солі', '1г чорного перцю') or be omitted.
4. If an essential ingredient is missing from the fridge, add it to 'gap_items' with proper category ('produce', 'dairy', 'meat', 'bakery', 'pantry', 'canned', 'frozen', 'beverages', 'other').
5. Respond with strictly valid JSON matching this schema:
{
  "meals": [
    {
      "name": "Meal Name",
      "day": "%s",
      "meal_type": "breakfast",
      "ingredients": ["100g вівсянка", "1 шт яблуко"],
      "note": "Корисний початок дня"
    }
  ],
  "gap_items": [
    {
      "name": "вівсянка",
      "quantity": "200",
      "unit": "г",
      "category": "pantry"
    }
  ]
}`,
		langInstruction,
		mealScope,
		targetCalStr,
		inventoryStr,
		req.CuisinePreference,
		req.DietaryProfile,
		existingStr,
		req.Day,
	)
}

func (s *Service) mealPlanWithOpenRouter(ctx context.Context, req models.MealPlanRequest) (*models.MealPlanResponse, error) {
	url := "https://openrouter.ai/api/v1/chat/completions"
	systemPrompt := buildMealPlanSystemPrompt(req)

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": fmt.Sprintf("Generate planned meals for %s based on our inventory.", req.Day)},
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
		"temperature": 0.5,
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
	httpReq.Header.Set("X-Title", "V-Fridge Meal Planner")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter returned status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseMealPlanChatResponse(raw, req.Day, req.MealType)
}

func (s *Service) mealPlanWithGemini(ctx context.Context, req models.MealPlanRequest) (*models.MealPlanResponse, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", s.cfg.GeminiAPIKey)
	systemPrompt := buildMealPlanSystemPrompt(req)

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
			"temperature":      0.5,
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
		return nil, fmt.Errorf("gemini returned status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var geminiRes struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(raw, &geminiRes); err != nil || len(geminiRes.Candidates) == 0 {
		return nil, fmt.Errorf("invalid gemini response format")
	}

	text := geminiRes.Candidates[0].Content.Parts[0].Text
	return parseMealPlanJson([]byte(text), req.Day, req.MealType)
}

func parseMealPlanChatResponse(raw []byte, day, mealType string) (*models.MealPlanResponse, error) {
	var chatResult struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(raw, &chatResult); err != nil || len(chatResult.Choices) == 0 {
		return nil, fmt.Errorf("invalid chat completion format")
	}

	return parseMealPlanJson([]byte(chatResult.Choices[0].Message.Content), day, mealType)
}

func parseMealPlanJson(content []byte, day, mealType string) (*models.MealPlanResponse, error) {
	var resp models.MealPlanResponse
	cleanContent := strings.TrimSpace(string(content))
	cleanContent = strings.TrimPrefix(cleanContent, "```json")
	cleanContent = strings.TrimPrefix(cleanContent, "```")
	cleanContent = strings.TrimSuffix(cleanContent, "```")
	cleanContent = strings.TrimSpace(cleanContent)

	if err := json.Unmarshal([]byte(cleanContent), &resp); err != nil {
		return nil, fmt.Errorf("failed to decode mealplan json: %w", err)
	}

	// Enforce day and mealType
	for i := range resp.Meals {
		if resp.Meals[i].Day == "" {
			resp.Meals[i].Day = day
		}
		if mealType != "" {
			resp.Meals[i].MealType = mealType
		}
		// Clean ingredients of vague "за смаком"
		cleanIngs := make([]string, 0, len(resp.Meals[i].Ingredients))
		for _, ing := range resp.Meals[i].Ingredients {
			if strings.Contains(strings.ToLower(ing), "за смаком") || strings.Contains(strings.ToLower(ing), "до смаку") {
				continue
			}
			cleanIngs = append(cleanIngs, ing)
		}
		resp.Meals[i].Ingredients = cleanIngs
	}

	return &resp, nil
}

// mealPlanFallback provides a reliable, balanced offline meal plan using inventory
func (s *Service) mealPlanFallback(req models.MealPlanRequest) *models.MealPlanResponse {
	isEn := strings.EqualFold(req.Language, "en")
	day := req.Day
	if day == "" {
		day = "Monday"
	}

	invSet := make(map[string]bool)
	for _, item := range req.Inventory {
		invSet[strings.ToLower(strings.TrimSpace(item))] = true
	}

	hasItem := func(needle string) bool {
		needle = strings.ToLower(needle)
		for k := range invSet {
			if strings.Contains(k, needle) {
				return true
			}
		}
		return false
	}

	var meals []models.MealPlanMeal
	var gaps []models.MealPlanGapItem

	// 1. Breakfast
	if req.MealType == "" || strings.EqualFold(req.MealType, "breakfast") {
		bName := "Вівсянка з яблуком та медом"
		bIngs := []string{"60г вівсяних пластівців", "1 шт яблуко", "150мл води або молока", "15г меду"}
		bNote := "Легкий та енергійний початок дня"
		if isEn {
			bName = "Oatmeal with Apple and Honey"
			bIngs = []string{"60g rolled oats", "1 apple", "150ml milk or water", "15g honey"}
			bNote = "Wholesome and energizing breakfast"
		}
		if hasItem("яйц") || hasItem("egg") {
			bName = "Омлет із зеленню та сиром"
			bIngs = []string{"2 яйця", "30г сиру", "10г вершкового масла", "зелень"}
			bNote = "Ситний білковий сніданок"
			if isEn {
				bName = "Cheese & Herb Omelette"
				bIngs = []string{"2 eggs", "30g cheese", "10g butter", "fresh herbs"}
				bNote = "High protein morning meal"
			}
		}

		meals = append(meals, models.MealPlanMeal{
			Name:        bName,
			Day:         day,
			MealType:    "breakfast",
			Ingredients: bIngs,
			Note:        bNote,
			Calories:    380,
			Protein:     18,
			Fat:         14,
			Carbs:       45,
		})
	}

	// 2. Lunch
	if req.MealType == "" || strings.EqualFold(req.MealType, "lunch") {
		lName := "Курячий бульйон з локшиною та овочами"
		lIngs := []string{"150г курячого філе", "50г яєчної локшини", "1 шт морква", "1 шт цибуля"}
		lNote := "Теплий та збалансований обід"
		if isEn {
			lName = "Chicken Noodle Vegetable Soup"
			lIngs = []string{"150g chicken fillet", "50g egg noodles", "1 carrot", "1 onion"}
			lNote = "Warm balanced lunch"
		}
		if !hasItem("кур") && !hasItem("chick") && (hasItem("рис") || hasItem("rice")) {
			lName = "Рис з овочами та соєвим соусом"
			lIngs = []string{"150г рису басматі", "1 шт солодкий перець", "1 шт морква", "20мл соєвого соусу"}
			lNote = "Легкий овочевий обід"
			if isEn {
				lName = "Vegetable Fried Rice"
				lIngs = []string{"150g basmati rice", "1 bell pepper", "1 carrot", "20ml soy sauce"}
				lNote = "Quick savory lunch"
			}
		}

		meals = append(meals, models.MealPlanMeal{
			Name:        lName,
			Day:         day,
			MealType:    "lunch",
			Ingredients: lIngs,
			Note:        lNote,
			Calories:    540,
			Protein:     38,
			Fat:         12,
			Carbs:       68,
		})
	}

	// 3. Dinner
	if req.MealType == "" || strings.EqualFold(req.MealType, "dinner") {
		dName := "Запечені овочі з сиром"
		dIngs := []string{"2 шт картоплі", "1 шт кабачок", "50г твердого сиру", "10мл рослинної олії"}
		dNote := "Легка вечеря для гарного сну"
		if isEn {
			dName = "Baked Vegetables with Cheese"
			dIngs = []string{"2 potatoes", "1 zucchini", "50g hard cheese", "10ml olive oil"}
			dNote = "Light dinner for restful sleep"
		}
		if hasItem("кур") || hasItem("chick") {
			dName = "Куряче філе на грилі зі свіжим салатом"
			dIngs = []string{"180г курячого філе", "2 шт помідори", "1 шт огірок", "10мл оливкової олії"}
			dNote = "Високобілкова легка вечеря"
			if isEn {
				dName = "Grilled Chicken Breast with Fresh Salad"
				dIngs = []string{"180g chicken breast", "2 tomatoes", "1 cucumber", "10ml olive oil"}
				dNote = "High-protein light dinner"
			}
		}

		meals = append(meals, models.MealPlanMeal{
			Name:        dName,
			Day:         day,
			MealType:    "dinner",
			Ingredients: dIngs,
			Note:        dNote,
			Calories:    460,
			Protein:     32,
			Fat:         16,
			Carbs:       42,
		})
	}

	// Check missing gap items for the generated meals
	for _, meal := range meals {
		for _, ing := range meal.Ingredients {
			norm := NormalizeIngredient(models.RecipeIngredient{Name: ing})
			if !hasItem(norm.Name) {
				qtyStr := ""
				if norm.Quantity > 0 {
					qtyStr = fmt.Sprintf("%v", norm.Quantity)
				}
				gaps = append(gaps, models.MealPlanGapItem{
					Name:     norm.Name,
					Quantity: qtyStr,
					Unit:     norm.Unit,
					Category: "produce",
				})
			}
		}
	}

	// Deduplicate gaps
	seenGaps := make(map[string]bool)
	uniqueGaps := make([]models.MealPlanGapItem, 0, len(gaps))
	for _, g := range gaps {
		key := strings.ToLower(g.Name)
		if !seenGaps[key] && key != "" {
			seenGaps[key] = true
			uniqueGaps = append(uniqueGaps, g)
		}
	}

	return &models.MealPlanResponse{
		Meals:    meals,
		GapItems: uniqueGaps,
	}
}
