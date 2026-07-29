package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ynshvrh/V-Chef/internal/chef"
	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestHealthCheckHandler(t *testing.T) {
	cfg := &config.Config{Port: "8085"}
	chefSvc := chef.NewService(cfg)
	h := NewRecipeHandler(chefSvc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp models.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.Status)
	}
}

func TestGenerateRecipeHandler(t *testing.T) {
	cfg := &config.Config{Port: "8085"}
	chefSvc := chef.NewService(cfg)
	h := NewRecipeHandler(chefSvc)

	payload := models.GenerateRecipeRequest{
		Ingredients: []string{"apple", "cinnamon"},
		MealType:    "snack",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.GenerateRecipe(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var recipe models.RecipeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &recipe); err != nil {
		t.Fatalf("failed to unmarshal recipe response: %v", err)
	}

	if recipe.Title == "" {
		t.Errorf("expected non-empty recipe title")
	}
}
