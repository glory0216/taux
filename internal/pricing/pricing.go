package pricing

import (
	"regexp"
	"strings"
)

// TokenPrice holds per-million-token prices for a model.
type TokenPrice struct {
	Input      float64 // $/MTok for input tokens
	Output     float64 // $/MTok for output tokens
	CacheRead  float64 // $/MTok for cache read tokens
	CacheWrite float64 // $/MTok for cache creation tokens
}

// Built-in pricing table. Keys are model name prefixes.
var defaultPriceMap = map[string]TokenPrice{
	// Anthropic Claude 4.x family
	"claude-opus-4":   {Input: 15.0, Output: 75.0, CacheRead: 1.50, CacheWrite: 18.75},
	"claude-sonnet-4": {Input: 3.0, Output: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
	"claude-haiku-4":  {Input: 0.80, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
	// Anthropic Claude 3.x family
	"claude-3-5-sonnet": {Input: 3.0, Output: 15.0, CacheRead: 0.30, CacheWrite: 3.75},
	"claude-3-5-haiku":  {Input: 0.80, Output: 4.0, CacheRead: 0.08, CacheWrite: 1.0},
	"claude-3-opus":     {Input: 15.0, Output: 75.0, CacheRead: 1.50, CacheWrite: 18.75},
	// OpenAI
	"gpt-4o":      {Input: 2.50, Output: 10.0},
	"gpt-4o-mini": {Input: 0.15, Output: 0.60},
	"gpt-4-turbo": {Input: 10.0, Output: 30.0},
	"o1-mini":     {Input: 1.10, Output: 4.40},
	"o3-mini":     {Input: 1.10, Output: 4.40},
	"o1":          {Input: 15.0, Output: 60.0},
	"o3":          {Input: 2.0, Output: 8.0},
	"o4-mini":     {Input: 1.10, Output: 4.40},
	// Google Gemini
	"gemini-2.5-pro":   {Input: 1.25, Output: 10.0, CacheRead: 0.3125, CacheWrite: 4.50},
	"gemini-2.5-flash": {Input: 0.15, Output: 0.60, CacheRead: 0.0375, CacheWrite: 1.00},
	"gemini-2.0-flash": {Input: 0.10, Output: 0.40, CacheRead: 0.025, CacheWrite: 0.025},
	"gemini-1.5-pro":   {Input: 1.25, Output: 5.0, CacheRead: 0.3125, CacheWrite: 4.50},
	"gemini-1.5-flash": {Input: 0.075, Output: 0.30, CacheRead: 0.01875, CacheWrite: 1.00},
}

// dateSuffixRe matches trailing -YYYYMMDD date suffixes in model names.
var dateSuffixRe = regexp.MustCompile(`-\d{8}$`)

// LookupPrice finds the pricing for a model name.
// Strategy: exact match → strip date suffix → longest prefix match.
// overrideMap takes priority over defaults.
func LookupPrice(modelName string, overrideMap map[string]TokenPrice) (TokenPrice, bool) {
	// 1. Exact match (override first, then default)
	if overrideMap != nil {
		if p, ok := overrideMap[modelName]; ok {
			return p, true
		}
	}
	if p, ok := defaultPriceMap[modelName]; ok {
		return p, true
	}

	// 2. Strip date suffix and retry
	stripped := stripDateSuffix(modelName)
	if stripped != modelName {
		if overrideMap != nil {
			if p, ok := overrideMap[stripped]; ok {
				return p, true
			}
		}
		if p, ok := defaultPriceMap[stripped]; ok {
			return p, true
		}
	}

	// 3. Longest prefix match across both tables
	var bestKey string
	var bestPrice TokenPrice

	for key, price := range defaultPriceMap {
		if strings.HasPrefix(modelName, key) && len(key) > len(bestKey) {
			bestKey = key
			bestPrice = price
		}
	}
	if overrideMap != nil {
		for key, price := range overrideMap {
			if strings.HasPrefix(modelName, key) && len(key) > len(bestKey) {
				bestKey = key
				bestPrice = price
			}
		}
	}
	if bestKey != "" {
		return bestPrice, true
	}

	return TokenPrice{}, false
}

// CalculateModelCost computes the USD cost for a set of token counts.
// Returns 0.0 if the model is not found in pricing tables.
func CalculateModelCost(
	modelName string,
	inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int,
	overrideMap map[string]TokenPrice,
) float64 {
	price, found := LookupPrice(modelName, overrideMap)
	if !found {
		return 0.0
	}
	return (float64(inputTokens)*price.Input +
		float64(outputTokens)*price.Output +
		float64(cacheReadTokens)*price.CacheRead +
		float64(cacheWriteTokens)*price.CacheWrite) / 1_000_000.0
}

// EffectiveCostPerToken computes a blended cost per token for a model.
// Used for period cost approximation from DailyModelTokens (which only
// stores total token counts, not broken down by type).
func EffectiveCostPerToken(
	modelName string,
	inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int,
	overrideMap map[string]TokenPrice,
) float64 {
	totalTokens := inputTokens + outputTokens + cacheReadTokens + cacheWriteTokens
	if totalTokens == 0 {
		return 0.0
	}
	cost := CalculateModelCost(modelName, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens, overrideMap)
	return cost / float64(totalTokens)
}

// stripDateSuffix removes a trailing -YYYYMMDD suffix from a model name.
// e.g. "claude-sonnet-4-5-20250929" → "claude-sonnet-4-5"
func stripDateSuffix(name string) string {
	return dateSuffixRe.ReplaceAllString(name, "")
}
