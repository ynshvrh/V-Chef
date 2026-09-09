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

	category := strings.TrimSpace(ing.Category)
	if category == "" || category == "other" {
		category = InferCategory(cleanName)
	}

	return models.RecipeIngredient{
		Name:     cleanName,
		Quantity: qty,
		Unit:     unit,
		Category: category,
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

var ukrSuffixes = []string{
	"ами", "ями", "ного", "ному", "них", "ній", "ної", "ним", "ний",
	"ою", "ею", "єю", "ом", "ем", "єм", "ів", "ей",
	"на", "не", "ні", "та", "те", "ті",
	"а", "я", "и", "і", "у", "ю", "е", "є", "о",
}

func stripUkrainianEnding(w string) string {
	runes := []rune(w)
	for _, suffix := range ukrSuffixes {
		sRunes := []rune(suffix)
		if strings.HasSuffix(w, suffix) && len(runes)-len(sRunes) >= 3 {
			return string(runes[:len(runes)-len(sRunes)])
		}
	}
	return w
}

func matchAnyStem(text string, stems ...string) bool {
	for _, stem := range stems {
		if strings.Contains(text, stem) {
			return true
		}
	}
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	for _, word := range words {
		stripped := stripUkrainianEnding(word)
		for _, stem := range stems {
			if strings.HasPrefix(stripped, stem) || strings.HasPrefix(word, stem) {
				return true
			}
		}
	}
	return false
}

// InferCategory determines the ProductCategories slug from the ingredient name
func InferCategory(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return "other"
	}

	// 1. Prepared meals
	if matchAnyStem(lower, "борщ", "суп", "рагу", "плов", "запіканк", "котлет", "омлет", "голубц", "вареник", "дерун", "сирник", "млинц") {
		return "prepared-meals"
	}

	// 2. Sauces, oils, spices
	if matchAnyStem(lower, "сіл", "сол", "salt", "олі", "oil", "перец", "перц", "pepper", "соус", "sauce", "паприк", "спеці", "spice", "кориц", "оцет", "оцт", "vinegar", "майонез", "mayo", "кетчуп", "ketchup", "гірчиц", "mustard", "приправ", "лавр", "каррі", "curry", "ореган", "базилік", "куркум", "сироп", "syrup") {
		return "sauces"
	}

	// 3. Dairy
	if matchAnyStem(lower, "молок", "milk", "сир", "cheese", "масл", "butter", "сметан", "sour cream", "кефір", "kefir", "йогурт", "yogurt", "творог", "cottage cheese", "вершк", "cream", "ряжанк", "моцарел", "пармезан", "сулугуні", "бринз") {
		return "dairy"
	}

	// 4. Meat & Fish
	if matchAnyStem(lower, "м'яс", "м’яс", "meat", "кур", "chicken", "фарш", "mince", "свинин", "pork", "яловичин", "beef", "телятин", "veal", "риб", "fish", "лосос", "salmon", "тунец", "тунц", "tuna", "креветк", "shrimp", "філе", "filet", "fillet", "бекон", "bacon", "ковбас", "sausage", "сосиск", "індичк", "turkey", "качк", "duck") {
		return "meat-fish"
	}

	// 5. Vegetables & greens
	if matchAnyStem(lower, "цибул", "onion", "часник", "garlic", "моркв", "carrot", "картопл", "potato", "помідор", "томат", "tomato", "огірок", "огірк", "cucumber", "капуст", "cabbage", "зелен", "петрушк", "parsley", "кріп", "кроп", "dill", "шпинат", "spinach", "салат", "lettuce", "кабачок", "кабачк", "zucchini", "баклажан", "eggplant", "броккол", "broccoli", "гриб", "mushroom", "печериц") {
		return "vegetables"
	}

	// 6. Fruits & berries
	if matchAnyStem(lower, "яблук", "apple", "банан", "banana", "лимон", "lemon", "лайм", "lime", "апельсин", "orange", "мандарин", "полуниц", "strawberry", "малин", "raspberry", "ягід", "ягод", "berries", "груш", "pear", "виноград", "grape", "авокадо", "avocado", "персик", "peach") {
		return "fruits"
	}

	// 7. Bread & Bakery
	if matchAnyStem(lower, "хліб", "bread", "батон", "булочк", "булк", "bun", "лаваш", "піт", "pita", "багет", "baguette", "круасан") {
		return "bakery"
	}

	// 8. Pantry staples
	if matchAnyStem(lower, "борошн", "flour", "рис", "rice", "гречк", "buckwheat", "макарон", "pasta", "спагет", "spaghetti", "цукор", "цукр", "sugar", "вівсян", "oats", "oatmeal", "круп", "квасол", "beans", "горох", "peas", "сочевиц", "lentils", "дріждж", "yeast", "крохмал", "starch") {
		return "pantry"
	}

	// 9. Drinks
	if matchAnyStem(lower, "вод", "water", "сік", "сок", "juice", "чай", "tea", "кав", "coffee", "морс", "компот") {
		return "drinks"
	}

	// 10. Alcohol
	if matchAnyStem(lower, "вин", "wine", "пив", "beer", "горілк", "vodka", "коньяк", "віскі", "whiskey", "ром", "rum") {
		return "alcohol"
	}

	// 11. Snacks & sweets
	if matchAnyStem(lower, "шоколад", "chocolate", "печив", "cookie", "цукерк", "candy", "горіх", "nuts", "чипс", "chips") {
		return "snacks"
	}

	// 12. Frozen
	if matchAnyStem(lower, "заморож", "frozen") {
		return "frozen"
	}

	// 13. Canned
	if matchAnyStem(lower, "консерв", "canned", "тушонк", "шпрот") {
		return "canned-prepared"
	}

	return "other"
}
