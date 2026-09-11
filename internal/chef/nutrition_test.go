package chef

import (
	"context"
	"testing"

	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestEstimateNutrition_FallbackBorscht(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.NutritionEstimateRequest{
		DishName: "300мл український борщ",
		Language: "uk",
	}

	resp, err := service.EstimateNutrition(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.Calories <= 0 {
		t.Errorf("expected positive calories, got %d", resp.Calories)
	}
	if resp.Confidence != "heuristic" {
		t.Errorf("expected confidence heuristic, got %s", resp.Confidence)
	}
	if resp.Protein <= 0 || resp.Fat <= 0 || resp.Carbs <= 0 {
		t.Errorf("expected positive macros, got P:%.1f, F:%.1f, C:%.1f", resp.Protein, resp.Fat, resp.Carbs)
	}
	if resp.Quantity != 300 {
		t.Errorf("expected extracted quantity 300, got %f", resp.Quantity)
	}
}

func TestEstimateNutrition_FallbackChicken(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.NutritionEstimateRequest{
		DishName: "Куряче філе",
		Quantity: 200,
		Unit:     "г",
		Language: "uk",
	}

	resp, err := service.EstimateNutrition(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Calories < 300 || resp.Calories > 400 {
		t.Errorf("expected ~330 kcal for 200g chicken breast, got %d", resp.Calories)
	}
	if resp.Protein < 50 {
		t.Errorf("expected high protein (>50g) for 200g chicken breast, got %.1f", resp.Protein)
	}
}

func TestEstimateNutrition_FallbackEggs(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.NutritionEstimateRequest{
		DishName: "Смажені яйця",
		Quantity: 2,
		Unit:     "шт",
	}

	resp, err := service.EstimateNutrition(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Calories < 150 || resp.Calories > 250 {
		t.Errorf("expected ~180-200 kcal for 2 eggs, got %d", resp.Calories)
	}
}

func TestEstimateNutrition_EmptyDishName(t *testing.T) {
	service := NewService(&config.Config{})

	req := models.NutritionEstimateRequest{}

	_, err := service.EstimateNutrition(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty dish name")
	}
}
