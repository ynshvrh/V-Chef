package models

// ChatHistoryItem represents a single turn in the conversation history
type ChatHistoryItem struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // message text
}

// ChatRequest represents the interactive culinary chat request
type ChatRequest struct {
	History           []ChatHistoryItem `json:"history"`
	Message           string            `json:"message"`
	Inventory         []string          `json:"inventory"`
	Language          string            `json:"language"`
	CuisinePreference string            `json:"cuisine_preference"`
	DietaryProfile    string            `json:"dietary_profile"`
}

// ChatResponse represents the structured culinary response
type ChatResponse struct {
	Reply               string             `json:"reply"`
	Recipe              *RecipeResponse    `json:"recipe,omitempty"`
	ShoppingSuggestions []RecipeIngredient `json:"shopping_suggestions,omitempty"`
}
