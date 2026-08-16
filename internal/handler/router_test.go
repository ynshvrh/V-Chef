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

func TestRouterAuthMiddleware(t *testing.T) {
	const secretToken = "test-secret-123"
	cfg := &config.Config{Port: "8085", InternalToken: secretToken}
	chefSvc := chef.NewService(cfg)
	recipeHandler := NewRecipeHandler(chefSvc)
	router := NewRouter(recipeHandler, secretToken)

	// 1. Health check must be accessible without token
	t.Run("Health check without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected health check status 200, got %d", rec.Code)
		}
	})

	payload := models.GenerateRecipeRequest{
		Ingredients: []string{"egg", "bread"},
		MealType:    "breakfast",
	}
	body, _ := json.Marshal(payload)

	// 2. Generate recipe without token must fail with 401
	t.Run("Generate recipe without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/generate", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rec.Code)
		}
	})

	// 3. Generate recipe with wrong token must fail with 401
	t.Run("Generate recipe with wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/generate", bytes.NewReader(body))
		req.Header.Set("X-Internal-Token", "wrong-secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", rec.Code)
		}
	})

	// 4. Generate recipe with valid token must succeed (200 OK)
	t.Run("Generate recipe with valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/generate", bytes.NewReader(body))
		req.Header.Set("X-Internal-Token", secretToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 OK with valid token, got %d", rec.Code)
		}
	})
}
