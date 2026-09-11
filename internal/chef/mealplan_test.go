package chef

import (
	"context"
	"testing"

	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestMealPlanFallback_FullDay(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.MealPlanRequest{
		Inventory: []string{"яйця", "куряче філе", "помідори"},
		Day:       "Wednesday",
		Language:  "uk",
	}

	resp, err := service.GenerateMealPlan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if len(resp.Meals) != 3 {
		t.Fatalf("expected 3 meals for full day, got %d", len(resp.Meals))
	}

	for _, m := range resp.Meals {
		if m.Day != "Wednesday" {
			t.Errorf("expected day Wednesday, got %s", m.Day)
		}
		if len(m.Ingredients) == 0 {
			t.Errorf("meal %s has no ingredients", m.Name)
		}
		for _, ing := range m.Ingredients {
			if ing == "" {
				t.Errorf("empty ingredient in meal %s", m.Name)
			}
		}
	}
}

func TestMealPlanFallback_SingleMeal(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.MealPlanRequest{
		Inventory: []string{"рис", "курка"},
		Day:       "Friday",
		MealType:  "dinner",
		Language:  "uk",
	}

	resp, err := service.GenerateMealPlan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Meals) != 1 {
		t.Fatalf("expected exactly 1 meal, got %d", len(resp.Meals))
	}

	if resp.Meals[0].MealType != "dinner" {
		t.Errorf("expected dinner, got %s", resp.Meals[0].MealType)
	}
}

func TestParseMealPlanJson_CleansVagueIngredients(t *testing.T) {
	rawJson := []byte(`{
		"meals": [
			{
				"name": "Тестова страва",
				"day": "Monday",
				"meal_type": "lunch",
				"ingredients": ["150г рису", "сіль за смаком", "спеції до смаку", "200г курки"]
			}
		],
		"gap_items": []
	}`)

	resp, err := parseMealPlanJson(rawJson, "Monday", "lunch")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(resp.Meals[0].Ingredients) != 2 {
		t.Fatalf("expected 2 ingredients after removing vague ones, got %d: %v", len(resp.Meals[0].Ingredients), resp.Meals[0].Ingredients)
	}
}
