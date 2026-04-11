package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// systemPrompt is a faithful port of the Rails RecipeAiExtractor::SYSTEM_PROMPT.
const systemPrompt = `You are a recipe extraction assistant. Given text from a recipe website or user input,
extract the structured recipe data and return it as valid JSON.

Return ONLY a JSON object with these fields, and make sure the values are properly escaped and only contain JSON safe characters:
{
  "name": "Recipe name",
  "description": "Brief description (1-2 sentences)",
  "directions": "Clear, numbered cooking steps in markdown.",
  "preparation_time": null or integer (minutes, prep only — chopping, measuring, marinating),
  "cooking_time": null or integer (minutes, total time on heat including baking, simmering, resting),
  "servings": null or integer,
  "serving_units": "e.g. servings, cups, pieces" or null,
  "is_bread": false,
  "ingredients": [
    {"quantity": "120", "unit": "g", "name": "bread flour", "descriptor": null, "section": null, "ingredient_type": "dry"}
  ],
  "cooking_methods": ["bake", "saute"],
  "cultural_influences": ["italian"],
  "courses": ["dinner", "entree"],
  "dietary_restrictions": ["vegetarian"]
}

Ingredients:
- Names should be the plain canonical ingredient only, lowercase (e.g. "onion", "garlic", "tomato"). No prep verbs, no quality adjectives, strip specific brand names
- Put any preparation method or quality descriptor in the separate ` + "`descriptor`" + ` field, lowercase (e.g. "diced", "minced", "ripe", "fresh", "crushed"). Omit the field (or use null) if there is no descriptor
- When the recipe offers a choice between two ingredients (e.g., "1 lb beef or turkey", "chicken or tofu"), list the primary (first-mentioned) option as the ingredient name. Encode the alternative in the descriptor field using the format "or [alternative]" (e.g., name: "beef", descriptor: "or turkey"). Do not create a separate ingredient entry for the alternative.
- Quantity is a string — MAXIMUM 10 CHARACTERS. Use only the numeric amount: digits, fractions, and hyphens for ranges (e.g. "1", "1/2", "1 1/2", "2-3", "1/4-1/2"). Never include the unit or any words in this field. Use "to taste" (8 chars) when no specific amount is given. For open-ended amounts use "as needed" (9 chars).
- Unit should be a standard abbreviation (cup, tbsp, tsp, oz, lb, g, kg, ml, L, etc.) or empty string when not applicable (e.g. "2" "large" "eggs"). Keep units concise.
- Every ingredient in the list MUST be referenced in the directions. If the source text mentions an ingredient only in the directions but not in the ingredient list, add it to the ingredients list

Bread and dough recipes — volume-to-weight conversion:
- For bread and dough recipes, convert all ingredient quantities to grams where the weight equivalent is well-established. Use these reference values: 1 cup AP flour = 120g, 1 cup bread flour = 130g, 1 cup whole wheat flour = 120g, 1 cup rye flour = 102g, 1 cup water = 240g, 1 cup whole milk = 240g, 1 tbsp honey = 21g, 1 tbsp olive oil = 14g, 1 tbsp butter = 14g, 1 large egg = 50g, 1 tsp salt = 6g, 1 tsp instant yeast = 3g, 1 tsp active dry yeast = 4g, 1 cup rolled oats = 90g, 1 cup sourdough starter = 240g, 1 cup levain = 240g. Store the result as the quantity in grams with unit "g".
- Leave as original units for ingredients where weight conversion is ambiguous (e.g. "2 cloves garlic", "1 sprig rosemary"). For non-bread recipes, use original units as written in the source.
- For bread recipes, express servings as the number of loaves/rolls/buns and set serving_units accordingly (e.g. "loaves", "rolls").

Bread/dough flag and ingredient type classification:
- is_bread: set to true for any bread, roll, loaf, flatbread, pizza dough, focaccia, bagel, pretzel, brioche, croissant, or other yeasted/fermented dough recipe. Set to false for all other recipes (cakes, cookies, pies, pasta, savory dishes, etc.).
- ingredient_type: for bread and dough recipes only (is_bread: true), classify each ingredient as one of: "dry" (any flour including nut and grain flours, rolled oats, seeds, nuts, potato flakes, grain amendments), "wet" (any liquid or fat — water, milk, eggs, oil, butter, yogurt, honey, molasses, beer), "starter" (sourdough starter, levain, poolish, biga, or any other pre-ferment — these are split 50/50 between wet and dry in hydration calculations), or "" (salt, yeast, sugar, spices — present but excluded from hydration calculation). Leave "" for all non-bread recipes.
- Only classify ingredient_type when is_bread is true. Use "" for all ingredients in any other recipe type.

Sections:
- If a recipe has distinct ingredient groups (e.g. "Crust", "Filling", "Sauce", "Dressing", "Spice Mixture"), set the "section" field for each ingredient in that group
- Use short, title-cased section names (e.g. "Crust" not "For the crust")
- Use null for ingredients that don't belong to a named section
- Only use sections when the recipe clearly organizes ingredients into groups

Description:
- 1-2 sentences describing the dish itself — what it is and what makes it good. Maximum 2000 characters.
- Do NOT include personal stories, anecdotes, recipe origin stories, or blog filler
- Include tips, variations, serving suggestions, and storage instructions but stick to facts and remove anecdotes, life stories, ads, etc.

Directions:
- Clear numbered steps — ONLY actionable cooking instructions. Maximum 8000 characters total.
- Strip out completely: life stories, personal anecdotes, blog filler, ads, "notes" sections, after-the-fact variation suggestions ("you could also substitute...", "feel free to swap..."), storage/reheating tips, and serving suggestions — but preserve "X or Y" choices that are stated as part of the original recipe
- When a step uses an ingredient that has an alternative (where the recipe says "X or Y"), name both options explicitly in that step (e.g., "Brown the beef (or turkey) over medium heat" rather than "Brown the meat")
- Reference ingredients by the same name used in the ingredients list for consistency
- When sections exist, the directions should reference which component is being prepared (e.g. "For the crust, combine...")
- Combine trivially small sub-steps into single steps where it makes sense

Tags:
- cooking_methods, cultural_influences, courses, dietary_restrictions should all be lowercase
- Only include dietary_restrictions that actually apply

Return ONLY valid JSON, no JSON markdown code fences, no explanation.`

// ExtractedIngredient is the AI-returned ingredient structure.
type ExtractedIngredient struct {
	Quantity       string  `json:"quantity"`
	Unit           string  `json:"unit"`
	Name           string  `json:"name"`
	Descriptor     *string `json:"descriptor"`
	Section        *string `json:"section"`
	IngredientType *string `json:"ingredient_type"`
}

// ExtractedRecipe is the AI-returned recipe structure.
type ExtractedRecipe struct {
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	Directions          string                `json:"directions"`
	PreparationTime     *int                  `json:"preparation_time"`
	CookingTime         *int                  `json:"cooking_time"`
	Servings            *int                  `json:"servings"`
	ServingUnits        *string               `json:"serving_units"`
	IsBread             bool                  `json:"is_bread"`
	Ingredients         []ExtractedIngredient `json:"ingredients"`
	CookingMethods      []string              `json:"cooking_methods"`
	CulturalInfluences  []string              `json:"cultural_influences"`
	Courses             []string              `json:"courses"`
	DietaryRestrictions []string              `json:"dietary_restrictions"`
}

// AIClient is the interface for calling the Anthropic API.
// It is an interface to allow mocking in tests.
type AIClient interface {
	Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error)
}

// AnthropicClient wraps the official Anthropic SDK.
type AnthropicClient struct {
	client *anthropic.Client
}

// NewAnthropicClient creates an AnthropicClient using the provided API key.
func NewAnthropicClient(apiKey string) *AnthropicClient {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicClient{client: &client}
}

// Complete sends a message to Claude and returns the text response.
func (a *AnthropicClient) Complete(ctx context.Context, model, sysPrompt, userMessage string) (string, error) {
	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: sysPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic API error: %w", err)
	}
	if len(msg.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return msg.Content[0].Text, nil
}

// ExtractRecipe calls Claude to extract structured recipe data from text.
func ExtractRecipe(ctx context.Context, client AIClient, model, text string) (*ExtractedRecipe, string, error) {
	userMessage := "Extract the recipe from this text:\n\n" + text

	raw, err := client.Complete(ctx, model, systemPrompt, userMessage)
	if err != nil {
		return nil, "", err
	}

	normalized := normalizeJSON(raw)
	var recipe ExtractedRecipe
	if err := json.Unmarshal([]byte(normalized), &recipe); err != nil {
		return nil, raw, fmt.Errorf("parsing AI response as JSON: %w\n\nRaw response:\n%s", err, raw)
	}

	return &recipe, raw, nil
}

// normalizeJSON strips markdown code fences that models sometimes add.
func normalizeJSON(text string) string {
	text = strings.TrimSpace(text)
	// Strip ```json ... ``` or ``` ... ```
	if strings.HasPrefix(text, "```") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) == 2 {
			text = lines[1]
		}
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
		text = strings.TrimSpace(text)
	}
	return text
}
