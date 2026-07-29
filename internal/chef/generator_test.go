package chef

import (
	"context"
	"testing"

	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestGenerateRecipeFallback(t *testing.T) {
	cfg := &config.Config{
		Port:         "8085",
		GeminiAPIKey: "", // No key = fallback mode
		Environment:  "test",
	}

	service := NewService(cfg)

	req := models.GenerateRecipeRequest{
		Ingredients:      []string{"milk", "eggs"},
		MealType:         "breakfast",
		DietaryCategory:  "vegetarian",
		MaxPrepTimeMins:  15,
		TargetCalories:   400,
	}

	recipe, err := service.GenerateRecipe(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if recipe == nil {
		t.Fatalf("expected recipe response, got nil")
	}

	if len(recipe.Ingredients) != 2 {
		t.Errorf("expected 2 ingredients, got %d", len(recipe.Ingredients))
	}

	if recipe.Calories <= 0 {
		t.Errorf("expected positive calories, got %d", recipe.Calories)
	}
}

func TestGenerateRecipeEmptyIngredients(t *testing.T) {
	cfg := &config.Config{
		Port:        "8085",
		Environment: "test",
	}

	service := NewService(cfg)

	req := models.GenerateRecipeRequest{
		Ingredients: []string{},
	}

	_, err := service.GenerateRecipe(context.Background(), req)
	if err == nil {
		t.Errorf("expected error for empty ingredients, got nil")
	}
}
