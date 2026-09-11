package models

type NutritionEstimateRequest struct {
	DishName string  `json:"dish_name"`
	Quantity float64 `json:"quantity,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Notes    string  `json:"notes,omitempty"`
	Language string  `json:"language,omitempty"`
}

type NutritionEstimateResponse struct {
	FoodName         string  `json:"food_name"`
	Quantity         float64 `json:"quantity"`
	Unit             string  `json:"unit"`
	Calories         int     `json:"calories"`
	Protein          float64 `json:"protein"`
	Fat              float64 `json:"fat"`
	Carbs            float64 `json:"carbs"`
	EstimatedWeightG float64 `json:"estimated_weight_g"`
	Confidence       string  `json:"confidence"` // "ai" | "heuristic"
	Notes            string  `json:"notes,omitempty"`
}
