package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ynshvrh/V-Chef/internal/chef"
	"github.com/ynshvrh/V-Chef/internal/models"
)

type RecipeHandler struct {
	chefService chef.Generator
}

func NewRecipeHandler(chefService chef.Generator) *RecipeHandler {
	return &RecipeHandler{
		chefService: chefService,
	}
}

func (h *RecipeHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := models.HealthResponse{
		Status:    "ok",
		Service:   "V-Chef AI Engine",
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *RecipeHandler) GenerateRecipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req models.GenerateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body format"})
		return
	}

	recipe, err := h.chefService.GenerateRecipe(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(recipe)
}
