package chef

import (
	"testing"

	"github.com/ynshvrh/V-Chef/internal/models"
)

func TestNormalizeIngredient(t *testing.T) {
	tests := []struct {
		name     string
		input    models.RecipeIngredient
		wantName string
		wantQty  float64
		wantUnit string
	}{
		{
			name:     "1 морква without explicit qty/unit",
			input:    models.RecipeIngredient{Name: "1 морква"},
			wantName: "Морква",
			wantQty:  1,
			wantUnit: "шт",
		},
		{
			name:     "2 шт великої моркви",
			input:    models.RecipeIngredient{Name: "2 шт великої моркви"},
			wantName: "Моркви",
			wantQty:  2,
			wantUnit: "шт",
		},
		{
			name:     "200г борошна",
			input:    models.RecipeIngredient{Name: "200г борошна"},
			wantName: "Борошна",
			wantQty:  200,
			wantUnit: "г",
		},
		{
			name:     "1.5 л свіжого молока",
			input:    models.RecipeIngredient{Name: "1.5 л свіжого молока"},
			wantName: "Молока",
			wantQty:  1.5,
			wantUnit: "л",
		},
		{
			name:     "1/2 лимона",
			input:    models.RecipeIngredient{Name: "1/2 лимона"},
			wantName: "Лимона",
			wantQty:  0.5,
			wantUnit: "шт",
		},
		{
			name:     "1-2 зубчики часнику",
			input:    models.RecipeIngredient{Name: "1-2 зубчики часнику"},
			wantName: "Часнику",
			wantQty:  2,
			wantUnit: "зубчик",
		},
		{
			name:     "дрібка солі",
			input:    models.RecipeIngredient{Name: "дрібка солі"},
			wantName: "Солі",
			wantQty:  1,
			wantUnit: "дрібка",
		},
		{
			name:     "Already clean ingredient with quantity and unit",
			input:    models.RecipeIngredient{Name: "Куряче філе", Quantity: 500, Unit: "г"},
			wantName: "Куряче філе",
			wantQty:  500,
			wantUnit: "г",
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
		})
	}
}
