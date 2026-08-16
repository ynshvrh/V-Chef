package handler

import (
	"log"
	"net/http"
)

func NewRouter(recipeHandler *RecipeHandler, internalToken ...string) http.Handler {
	token := ""
	if len(internalToken) > 0 {
		token = internalToken[0]
	}

	mux := http.NewServeMux()

	// /health remains public (unauthenticated) for liveness & warmup pings
	mux.HandleFunc("GET /health", recipeHandler.HealthCheck)

	var generateHandler http.Handler = http.HandlerFunc(recipeHandler.GenerateRecipe)
	if token != "" {
		generateHandler = authMiddleware(token, generateHandler)
	} else {
		log.Println("⚠️ V-Chef: INTERNAL_TOKEN is empty — auth header check disabled (dev mode)")
	}

	mux.Handle("POST /api/v1/recipes/generate", generateHandler)

	// Add CORS middleware wrapper
	return corsMiddleware(mux)
}

func authMiddleware(expectedToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Internal-Token")
		if token == "" || token != expectedToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code":"UNAUTHORIZED","error":"Invalid or missing internal service token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Token")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
