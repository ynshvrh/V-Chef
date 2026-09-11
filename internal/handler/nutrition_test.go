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

func TestEstimateNutritionHandler(t *testing.T) {
	service := chef.NewService(&config.Config{})
	h := NewRecipeHandler(service)

	reqBody, _ := json.Marshal(models.NutritionEstimateRequest{
		DishName: "Гречана каша з маслом",
		Quantity: 200,
		Unit:     "г",
		Language: "uk",
	})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/nutrition/estimate", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	h.EstimateNutrition(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.NutritionEstimateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Calories <= 0 {
		t.Fatalf("expected positive calories, got %d", resp.Calories)
	}
	if resp.Quantity != 200 {
		t.Fatalf("expected quantity 200, got %f", resp.Quantity)
	}
}

func TestEstimateNutritionHandler_RouterIntegration(t *testing.T) {
	service := chef.NewService(&config.Config{})
	h := NewRecipeHandler(service)
	router := NewRouter(h, "test-token")

	reqBody, _ := json.Marshal(models.NutritionEstimateRequest{
		DishName: "Салат цезар",
		Quantity: 250,
		Unit:     "г",
	})

	// Without auth header -> 401
	rNoAuth := httptest.NewRequest(http.MethodPost, "/api/v1/nutrition/estimate", bytes.NewReader(reqBody))
	wNoAuth := httptest.NewRecorder()
	router.ServeHTTP(wNoAuth, rNoAuth)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without token, got %d", wNoAuth.Code)
	}

	// With auth header -> 200
	rAuth := httptest.NewRequest(http.MethodPost, "/api/v1/nutrition/estimate", bytes.NewReader(reqBody))
	rAuth.Header.Set("X-Internal-Token", "test-token")
	wAuth := httptest.NewRecorder()
	router.ServeHTTP(wAuth, rAuth)
	if wAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with token, got %d: %s", wAuth.Code, wAuth.Body.String())
	}
}
