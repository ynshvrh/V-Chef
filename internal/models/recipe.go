package models

import "time"

// GenerateRecipeRequest represents the request payload from V-Fridge
type GenerateRecipeRequest struct {
	Ingredients     []string `json:"ingredients"`                  // e.g. ["milk", "eggs", "flour"]
	MealType        string   `json:"meal_type,omitempty"`          // e.g. "breakfast", "lunch", "dinner", "snack"
	DietaryCategory string   `json:"dietary_category,omitempty"`   // e.g. "keto", "vegan", "vegetarian", "any"
	MaxPrepTimeMins int      `json:"max_prep_time_mins,omitempty"` // e.g. 30
	TargetCalories  int      `json:"target_calories,omitempty"`    // e.g. 500
}

// RecipeIngredient represents a single ingredient item in a generated recipe
type RecipeIngredient struct {
	Name     string  `json:"name"`               // e.g. "Milk"
	Quantity float64 `json:"quantity,omitempty"` // e.g. 200
	Unit     string  `json:"unit,omitempty"`     // e.g. "ml"
	InFridge bool    `json:"in_fridge"`          // true if caller already has it
}

// RecipeResponse represents a fully structured recipe output
type RecipeResponse struct {
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	PrepTimeMins int                `json:"prep_time_mins"`
	CookTimeMins int                `json:"cook_time_mins"`
	Servings     int                `json:"servings"`
	Calories     int                `json:"calories"`
	ProteinGrams float64            `json:"protein_grams"`
	FatGrams     float64            `json:"fat_grams"`
	CarbsGrams   float64            `json:"carbs_grams"`
	Ingredients  []RecipeIngredient `json:"ingredients"`
	Steps        []string           `json:"steps"`
	GeneratedAt  time.Time          `json:"generated_at"`
}

// HealthResponse represents service health status
type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Timestamp time.Time `json:"timestamp"`
}
