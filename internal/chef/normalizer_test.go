package chef

import (
	"testing"

	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestNormalizeIngredient(t *testing.T) {
	tests := []struct {
		name         string
		input        models.RecipeIngredient
		wantName     string
		wantQty      float64
		wantUnit     string
		wantCategory string
	}{
		{
			name:         "1 морква without explicit qty/unit",
			input:        models.RecipeIngredient{Name: "1 морква"},
			wantName:     "Морква",
			wantQty:      1,
			wantUnit:     "шт",
			wantCategory: "vegetables",
		},
		{
			name:         "2 шт великої моркви",
			input:        models.RecipeIngredient{Name: "2 шт великої моркви"},
			wantName:     "Моркви",
			wantQty:      2,
			wantUnit:     "шт",
			wantCategory: "vegetables",
		},
		{
			name:         "200г борошна",
			input:        models.RecipeIngredient{Name: "200г борошна"},
			wantName:     "Борошна",
			wantQty:      200,
			wantUnit:     "г",
			wantCategory: "pantry",
		},
		{
			name:         "1.5 л свіжого молока",
			input:        models.RecipeIngredient{Name: "1.5 л свіжого молока"},
			wantName:     "Молока",
			wantQty:      1.5,
			wantUnit:     "л",
			wantCategory: "dairy",
		},
		{
			name:         "1/2 лимона",
			input:        models.RecipeIngredient{Name: "1/2 лимона"},
			wantName:     "Лимона",
			wantQty:      0.5,
			wantUnit:     "шт",
			wantCategory: "fruits",
		},
		{
			name:         "1-2 зубчики часнику",
			input:        models.RecipeIngredient{Name: "1-2 зубчики часнику"},
			wantName:     "Часнику",
			wantQty:      2,
			wantUnit:     "зубчик",
			wantCategory: "vegetables",
		},
		{
			name:         "дрібка солі",
			input:        models.RecipeIngredient{Name: "дрібка солі"},
			wantName:     "Солі",
			wantQty:      1,
			wantUnit:     "дрібка",
			wantCategory: "sauces",
		},
		{
			name:         "Already clean ingredient with quantity and unit",
			input:        models.RecipeIngredient{Name: "Куряче філе", Quantity: 500, Unit: "г"},
			wantName:     "Куряче філе",
			wantQty:      500,
			wantUnit:     "г",
			wantCategory: "meat-fish",
		},
		{
			name:         "500 мл води",
			input:        models.RecipeIngredient{Name: "500 мл води"},
			wantName:     "Води",
			wantQty:      500,
			wantUnit:     "мл",
			wantCategory: "drinks",
		},
		{
			name:         "1 кг картоплі",
			input:        models.RecipeIngredient{Name: "1 кг картоплі"},
			wantName:     "Картоплі",
			wantQty:      1,
			wantUnit:     "кг",
			wantCategory: "vegetables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeIngredient(tt.input)
			if got.Name != tt.wantName {
				t.Errorf("NormalizeIngredient() Name = %v, want %v", got.Name, tt.wantName)
			}
			if got.Quantity != tt.wantQty {
				t.Errorf("NormalizeIngredient() Quantity = %v, want %v", got.Quantity, tt.wantQty)
			}
			if got.Unit != tt.wantUnit {
				t.Errorf("NormalizeIngredient() Unit = %v, want %v", got.Unit, tt.wantUnit)
			}
			if tt.wantCategory != "" && got.Category != tt.wantCategory {
				t.Errorf("NormalizeIngredient() Category = %v, want %v", got.Category, tt.wantCategory)
			}
		})
	}
}
