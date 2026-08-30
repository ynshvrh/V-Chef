package chef

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ynshvrh/V-Chef/internal/models"
)

var (
	// Matches leading fraction like "1/2", "1/4", "3/4"
	fractionRegex = regexp.MustCompile(`^\s*(\d+)\s*/\s*(\d+)\s*([a-zA-Zа-яА-ЯіїєІЇЄ\.\s]*?)\s+(.+)$`)

	// Matches leading range like "1-2", "2-3"
	rangeRegex = regexp.MustCompile(`^\s*(\d+(?:[\.,]\d+)?)\s*-\s*(\d+(?:[\.,]\d+)?)\s*([a-zA-Zа-яА-ЯіїєІЇЄ\.\s]*?)\s+(.+)$`)

	// Matches leading quantity + optional unit + remainder, e.g. "500г борошна", "2 шт моркви", "1 морква"
	leadingQtyRegex = regexp.MustCompile(`^\s*(\d+(?:[\.,]\d+)?)\s*([a-zA-Zа-яА-ЯіїєІЇЄ\.]+)?(?:\s+(.+))?$`)

	// Matches words like "дрібка солі", "щепотка перцю"
	specialPinchRegex = regexp.MustCompile(`^(?i)\s*(дрібка|щепотка|pinch)\s+(.+)$`)
)

// NormalizeIngredient cleans up messy ingredient names produced by AI models
// (e.g. "1 морква", "200г борошна", "2 шт великої цибулі") into structured Name, Quantity, and Unit.
func NormalizeIngredient(ing models.RecipeIngredient) models.RecipeIngredient {
	rawName := strings.TrimSpace(ing.Name)
	if rawName == "" {
		return ing
	}

	qty := ing.Quantity
	unit := strings.TrimSpace(ing.Unit)
	cleanName := rawName

	// 1. Check special pinch phrases like "дрібка солі"
	if pinchMatch := specialPinchRegex.FindStringSubmatch(cleanName); len(pinchMatch) > 2 {
		if qty == 0 {
			qty = 1
		}
		if unit == "" {
			unit = "дрібка"
		}
		cleanName = pinchMatch[2]
	} else if fracMatch := fractionRegex.FindStringSubmatch(cleanName); len(fracMatch) > 4 {
		// 2. Check fraction like "1/2 лимона"
		num, _ := strconv.ParseFloat(fracMatch[1], 64)
		denom, _ := strconv.ParseFloat(fracMatch[2], 64)
		if denom > 0 && qty == 0 {
			qty = num / denom
		}
		potentialUnit := strings.TrimSpace(fracMatch[3])
		if unit == "" && potentialUnit != "" {
			unit = potentialUnit
		}
		cleanName = fracMatch[4]
	} else if rangeMatch := rangeRegex.FindStringSubmatch(cleanName); len(rangeMatch) > 4 {
		// 3. Check range like "1-2 зубчики часнику" -> take upper bound
		valStr := strings.ReplaceAll(rangeMatch[2], ",", ".")
		if upper, err := strconv.ParseFloat(valStr, 64); err == nil && qty == 0 {
			qty = upper
		}
		potentialUnit := strings.TrimSpace(rangeMatch[3])
		if unit == "" && potentialUnit != "" {
			unit = potentialUnit
		}
		cleanName = rangeMatch[4]
	} else if qtyMatch := leadingQtyRegex.FindStringSubmatch(cleanName); len(qtyMatch) > 1 {
		// 4. Check leading quantity + unit
		valStr := strings.ReplaceAll(qtyMatch[1], ",", ".")
		if parsedQty, err := strconv.ParseFloat(valStr, 64); err == nil && parsedQty > 0 {
			if qty == 0 {
				qty = parsedQty
			}
			potentialUnit := strings.TrimSpace(qtyMatch[2])
			if unit == "" && potentialUnit != "" {
				unit = potentialUnit
			}

			// If group 3 has remainder (e.g. "моркви" in "1 шт моркви" or "1 морква")
			if len(qtyMatch) > 3 && strings.TrimSpace(qtyMatch[3]) != "" {
				cleanName = qtyMatch[3]
			} else if potentialUnit != "" && isLikelyUnit(potentialUnit) {
				// E.g. "200г" with no remainder
				cleanName = ""
			} else if potentialUnit != "" && !isLikelyUnit(potentialUnit) {
				// E.g. "1 морква" where group 2 caught "морква" as unit because there was no 2nd word!
				cleanName = potentialUnit
				if unit == potentialUnit {
					unit = "шт"
				}
			}
		}
	}

	// Clean up unit
	unit = NormalizeUnit(unit)
	if unit == "" {
		unit = "шт"
	}
	if qty == 0 {
		qty = 1
	}

	// Strip common descriptor prefixes from ingredient name (e.g. "великої", "середньої", "свіжого")
	cleanName = cleanDescriptorNoise(cleanName)
	cleanName = capitalizeFirst(cleanName)

	if cleanName == "" {
		cleanName = capitalizeFirst(rawName)
	}

	return models.RecipeIngredient{
		Name:     cleanName,
		Quantity: qty,
		Unit:     unit,
		InFridge: ing.InFridge,
	}
}

// NormalizeRecipe processes all ingredients and nutritional values in a recipe
func NormalizeRecipe(recipe *models.RecipeResponse) *models.RecipeResponse {
	if recipe == nil {
		return nil
	}

	sanitizedIngredients := make([]models.RecipeIngredient, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		sanitizedIngredients[i] = NormalizeIngredient(ing)
	}
	recipe.Ingredients = sanitizedIngredients

	if recipe.Servings <= 0 {
		recipe.Servings = 2
	}
	if recipe.PrepTimeMins < 0 {
		recipe.PrepTimeMins = 10
	}
	if recipe.CookTimeMins < 0 {
		recipe.CookTimeMins = 15
	}

	recipe.Title = capitalizeFirst(strings.TrimSpace(recipe.Title))
	recipe.Description = strings.TrimSpace(recipe.Description)

	return recipe
}

func isLikelyUnit(s string) bool {
	lower := strings.ToLower(strings.Trim(s, "."))
	switch lower {
	case "г", "g", "гр", "грам", "грамм", "кг", "kg", "мл", "ml", "л", "l", "шт", "pcs", "pc", "ст", "стл", "чл", "ст.л", "ч.л", "tbsp", "tsp", "зубчик", "зубчики":
		return true
	default:
		return false
	}
}

func NormalizeUnit(unit string) string {
	if unit == "" {
		return ""
	}
	u := strings.ToLower(strings.TrimSpace(unit))
	u = strings.TrimSuffix(u, ".")

	switch u {
	case "г", "g", "гр", "грам", "грамів", "грамма", "грамм", "grams", "gram":
		return "г"
	case "кг", "kg", "кілограм", "кілограмів", "килограмм", "килограммів", "kilograms", "kilogram":
		return "кг"
	case "мл", "ml", "мілілітр", "мілілітрів", "миллилитр", "миллилитров", "milliliters", "milliliter":
		return "мл"
	case "л", "l", "літр", "літрів", "литр", "литров", "liters", "liter":
		return "л"
	case "шт", "pcs", "штук", "штуки", "штука", "pc", "piece", "pieces":
		return "шт"
	case "ст. л", "ст.л", "ст л", "столова ложка", "столові ложки", "столових ложок", "tbsp", "tablespoon", "tablespoons":
		return "ст.л."
	case "ч. л", "ч.л", "ч л", "чайна ложка", "чайні ложки", "чайних ложок", "tsp", "teaspoon", "teaspoons":
		return "ч.л."
	case "дрібка", "щепотка", "pinch":
		return "дрібка"
	case "зубчик", "зубчики", "зубчиків", "clove", "cloves":
		return "зубчик"
	case "порція", "порції", "порцій", "serving", "servings":
		return "порцій"
	default:
		return unit
	}
}

func cleanDescriptorNoise(name string) string {
	n := strings.TrimSpace(name)
	// Remove common leading punctuation
	n = strings.Trim(n, "-–—:,.")

	noiseWords := []string{
		"великої", "великий", "велика", "великі", "великих",
		"середньої", "середній", "середня", "середні", "середніх",
		"маленької", "маленький", "маленька", "маленькі", "маленьких",
		"свіжого", "свіжий", "свіжа", "свіжі", "свіжих",
		"стиглого", "стиглий", "стигла", "стиглі", "стиглих",
	}

	lower := strings.ToLower(n)
	for _, word := range noiseWords {
		if strings.HasPrefix(lower, word+" ") {
			n = strings.TrimSpace(n[len(word)+1:])
			lower = strings.ToLower(n)
		}
	}

	return strings.Trim(n, "-–—:,.")
}

func capitalizeFirst(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
