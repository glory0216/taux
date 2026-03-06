package model

import "strings"

// maxContextMap maps model name prefixes to their maximum context window size.
var maxContextMap = map[string]int{
	// Anthropic Claude 4.x
	"claude-opus-4":   200000,
	"claude-sonnet-4": 200000,
	"claude-haiku-4":  200000,
	// Anthropic Claude 3.x
	"claude-3-5-sonnet": 200000,
	"claude-3-5-haiku":  200000,
	"claude-3-opus":     200000,
	// OpenAI
	"gpt-4o":      128000,
	"gpt-4o-mini": 128000,
	"gpt-4-turbo": 128000,
	"o1-mini":     200000,
	"o3-mini":     200000,
	"o1":          200000,
	"o3":          200000,
	"o4-mini":     200000,
	// Google Gemini
	"gemini-2.5-pro":   1000000,
	"gemini-2.5-flash": 1000000,
	"gemini-2.0-flash": 1000000,
}

// MaxContext returns the maximum context window size for a model.
// Uses longest prefix match. Returns 0 if unknown.
func MaxContext(modelName string) int {
	// Exact match first
	if v, ok := maxContextMap[modelName]; ok {
		return v
	}
	// Longest prefix match
	var bestKey string
	var bestVal int
	for key, val := range maxContextMap {
		if strings.HasPrefix(modelName, key) && len(key) > len(bestKey) {
			bestKey = key
			bestVal = val
		}
	}
	return bestVal
}
