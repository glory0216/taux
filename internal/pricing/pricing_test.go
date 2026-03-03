package pricing

import (
	"math"
	"testing"
)

func TestLookupPrice_ExactMatch(t *testing.T) {
	price, found := LookupPrice("claude-opus-4", nil)
	if !found {
		t.Fatal("expected to find claude-opus-4")
	}
	if price.Input != 15.0 {
		t.Errorf("Input: got %f, want 15.0", price.Input)
	}
	if price.Output != 75.0 {
		t.Errorf("Output: got %f, want 75.0", price.Output)
	}
}

func TestLookupPrice_DateStripped(t *testing.T) {
	price, found := LookupPrice("claude-sonnet-4-5-20250929", nil)
	if !found {
		t.Fatal("expected to find claude-sonnet-4-5-20250929 via date stripping")
	}
	// Should match "claude-sonnet-4" after stripping → prefix match
	if price.Input != 3.0 {
		t.Errorf("Input: got %f, want 3.0", price.Input)
	}
}

func TestLookupPrice_PrefixMatch(t *testing.T) {
	price, found := LookupPrice("claude-opus-4-6", nil)
	if !found {
		t.Fatal("expected to find claude-opus-4-6 via prefix match")
	}
	if price.Input != 15.0 {
		t.Errorf("Input: got %f, want 15.0", price.Input)
	}
	if price.Output != 75.0 {
		t.Errorf("Output: got %f, want 75.0", price.Output)
	}
}

func TestLookupPrice_Override(t *testing.T) {
	override := map[string]TokenPrice{
		"claude-opus-4": {Input: 20.0, Output: 100.0, CacheRead: 2.0, CacheWrite: 25.0},
	}
	price, found := LookupPrice("claude-opus-4", override)
	if !found {
		t.Fatal("expected to find claude-opus-4 from override")
	}
	if price.Input != 20.0 {
		t.Errorf("Input: got %f, want 20.0 (override)", price.Input)
	}
	if price.Output != 100.0 {
		t.Errorf("Output: got %f, want 100.0 (override)", price.Output)
	}
}

func TestLookupPrice_NotFound(t *testing.T) {
	price, found := LookupPrice("unknown-model", nil)
	if found {
		t.Error("expected not found for unknown-model")
	}
	if price != (TokenPrice{}) {
		t.Errorf("expected zero TokenPrice, got %+v", price)
	}
}

func TestCalculateModelCost(t *testing.T) {
	// claude-opus-4: Input=15.0, Output=75.0
	// (100000*15.0 + 50000*75.0) / 1_000_000 = (1_500_000 + 3_750_000) / 1_000_000 = 5.25
	cost := CalculateModelCost("claude-opus-4", 100000, 50000, 0, 0, nil)
	if math.Abs(cost-5.25) > 1e-9 {
		t.Errorf("cost: got %f, want 5.25", cost)
	}
}

func TestCalculateModelCost_WithCache(t *testing.T) {
	// claude-opus-4: Input=15.0, Output=75.0, CacheRead=1.50, CacheWrite=18.75
	// (1000*15.0 + 500*75.0 + 2000*1.50 + 300*18.75) / 1_000_000
	// = (15000 + 37500 + 3000 + 5625) / 1_000_000 = 61125 / 1_000_000 = 0.061125
	cost := CalculateModelCost("claude-opus-4", 1000, 500, 2000, 300, nil)
	if math.Abs(cost-0.061125) > 1e-9 {
		t.Errorf("cost: got %f, want 0.061125", cost)
	}
}

func TestCalculateModelCost_UnknownModel(t *testing.T) {
	cost := CalculateModelCost("unknown-model", 100000, 50000, 0, 0, nil)
	if cost != 0.0 {
		t.Errorf("cost: got %f, want 0.0", cost)
	}
}

func TestEffectiveCostPerToken(t *testing.T) {
	// claude-opus-4 with 1000 input + 500 output = 1500 total tokens
	// cost = (1000*15.0 + 500*75.0) / 1_000_000 = (15000 + 37500) / 1_000_000 = 0.0525
	// blended = 0.0525 / 1500 = 0.000035
	rate := EffectiveCostPerToken("claude-opus-4", 1000, 500, 0, 0, nil)
	expected := 0.0525 / 1500.0
	if math.Abs(rate-expected) > 1e-12 {
		t.Errorf("rate: got %e, want %e", rate, expected)
	}
}

func TestEffectiveCostPerToken_ZeroTokens(t *testing.T) {
	rate := EffectiveCostPerToken("claude-opus-4", 0, 0, 0, 0, nil)
	if rate != 0.0 {
		t.Errorf("rate: got %f, want 0.0", rate)
	}
}

func TestStripDateSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-sonnet-4-5-20250929", "claude-sonnet-4-5"},
		{"claude-opus-4", "claude-opus-4"},
		{"gpt-4o-20240101", "gpt-4o"},
	}
	for _, tc := range tests {
		got := stripDateSuffix(tc.input)
		if got != tc.want {
			t.Errorf("stripDateSuffix(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
