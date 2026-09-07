package chef

import (
	"context"
	"testing"

	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestChatFallback(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
	}
	svc := NewService(cfg)

	// Ukrainian fallback
	reqUk := models.ChatRequest{
		Message:   "Привіт, що приготувати?",
		Language:  "uk",
		Inventory: []string{"яйця", "молоко"},
	}
	respUk, err := svc.Chat(context.Background(), reqUk)
	if err != nil {
		t.Fatalf("Chat fallback error: %v", err)
	}
	if respUk == nil || respUk.Reply == "" {
		t.Fatalf("Expected non-empty reply in ChatResponse")
	}

	// English fallback
	reqEn := models.ChatRequest{
		Message:  "Hello chef!",
		Language: "en",
	}
	respEn, err := svc.Chat(context.Background(), reqEn)
	if err != nil {
		t.Fatalf("Chat fallback error: %v", err)
	}
	if respEn == nil || respEn.Reply == "" {
		t.Fatalf("Expected non-empty reply in English ChatResponse")
	}
}

func TestChatEmptyMessage(t *testing.T) {
	cfg := &config.Config{}
	svc := NewService(cfg)

	req := models.ChatRequest{
		Message: "   ",
	}
	_, err := svc.Chat(context.Background(), req)
	if err == nil {
		t.Fatalf("Expected error for empty chat message")
	}
}

func TestParseChatJson(t *testing.T) {
	cfg := &config.Config{}
	svc := NewService(cfg)

	sampleJson := `{
		"reply": "Чудова ідея! Пропоную омлет.",
		"recipe": {
			"title": "Омлет з сиром",
			"description": "Швидкий та поживний сніданок",
			"prep_time_mins": 5,
			"cook_time_mins": 10,
			"servings": 2,
			"calories": 320,
			"protein_grams": 20.0,
			"fat_grams": 18.0,
			"carbs_grams": 4.0,
			"ingredients": [
				{"name": "2 яйця", "quantity": 0, "unit": "", "in_fridge": true}
			],
			"steps": ["Збийте яйця", "Підсмажте на пательні"]
		},
		"shopping_suggestions": [
			{"name": "100г твердого сиру", "quantity": 0, "unit": "", "in_fridge": false}
		]
	}`

	resp, err := svc.parseChatJson(sampleJson)
	if err != nil {
		t.Fatalf("parseChatJson failed: %v", err)
	}

	if resp.Reply != "Чудова ідея! Пропоную омлет." {
		t.Errorf("Unexpected reply: %s", resp.Reply)
	}
	if resp.Recipe == nil || resp.Recipe.Title != "Омлет з сиром" {
		t.Fatalf("Expected valid recipe parsed")
	}
	if len(resp.Recipe.Ingredients) != 1 {
		t.Fatalf("Expected 1 normalized ingredient")
	}
	if resp.Recipe.Ingredients[0].Name != "Яйця" || resp.Recipe.Ingredients[0].Quantity != 2 {
		t.Errorf("Ingredient normalization failed, got %+v", resp.Recipe.Ingredients[0])
	}

	if len(resp.ShoppingSuggestions) != 1 {
		t.Fatalf("Expected 1 shopping suggestion")
	}
	if resp.ShoppingSuggestions[0].Name != "Твердого сиру" || resp.ShoppingSuggestions[0].Quantity != 100 {
		t.Errorf("Shopping suggestion normalization failed, got %+v", resp.ShoppingSuggestions[0])
	}
}
