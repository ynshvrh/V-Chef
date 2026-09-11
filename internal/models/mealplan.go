package models

// MealPlanMeal represents a single planned meal in the menu
type MealPlanMeal struct {
	Name                  string             `json:"name"`
	Day                   string             `json:"day"`
	MealType              string             `json:"meal_type"` // "breakfast", "lunch", "dinner"
	Ingredients           []string           `json:"ingredients"`
	Note                  string             `json:"note,omitempty"`
	Description           string             `json:"description,omitempty"`
	Steps                 []string           `json:"steps,omitempty"`
	Calories              int                `json:"calories,omitempty"`
	Protein               float64            `json:"protein,omitempty"`
	Fat                   float64            `json:"fat,omitempty"`
	Carbs                 float64            `json:"carbs,omitempty"`
	StructuredIngredients []RecipeIngredient `json:"structured_ingredients,omitempty"`
}

// MealPlanGapItem represents a missing product required for the meal plan
type MealPlanGapItem struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Category string `json:"category"`
}

// MealPlanRequest represents the request to generate a meal plan or regenerate day/meal
type MealPlanRequest struct {
	Inventory         []string `json:"inventory"`
	CuisinePreference string   `json:"cuisine_preference"`
	Language          string   `json:"language"`
	DietaryProfile    string   `json:"dietary_profile"`
	Day               string   `json:"day,omitempty"`
	MealType          string   `json:"meal_type,omitempty"`
	ExistingMeals     []string `json:"existing_meals,omitempty"`
	TargetCalories    int      `json:"target_calories,omitempty"`
}

// MealPlanResponse represents the planned meals and missing items
type MealPlanResponse struct {
	Meals    []MealPlanMeal    `json:"meals"`
	GapItems []MealPlanGapItem `json:"gap_items"`
}
