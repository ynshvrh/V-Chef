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

func TestGenerateMealPlanHandler(t *testing.T) {
	service := chef.NewService(&config.Config{})
	h := NewRecipeHandler(service)

	reqBody, _ := json.Marshal(models.MealPlanRequest{
		Inventory: []string{"яйця", "помідори"},
		Day:       "Tuesday",
		Language:  "uk",
	})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/mealplan/generate", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	h.GenerateMealPlan(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.MealPlanResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Meals) == 0 {
		t.Fatal("expected at least one planned meal")
	}
}
