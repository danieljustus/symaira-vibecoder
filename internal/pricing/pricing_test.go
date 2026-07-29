package pricing

import (
	"testing"
)

func TestPricingLookupAndCost(t *testing.T) {
	t.Run("Exact model lookup", func(t *testing.T) {
		cost, ok := CalculateCost("anthropic/claude-3-5-sonnet-20241022", 1000, 2000, 500)
		if !ok {
			t.Fatalf("expected pricing lookup to succeed")
		}
		// 1000 * 3.0 / 1e6 = 0.003
		// 2000 * 15.0 / 1e6 = 0.030
		// 500 * 0.3 / 1e6 = 0.00015
		// Total = 0.03315
		expected := 0.03315
		if diff := cost - expected; diff > 0.00001 || diff < -0.00001 {
			t.Errorf("cost = %v, want %v", cost, expected)
		}
	})

	t.Run("Fallback family lookup", func(t *testing.T) {
		cost, ok := CalculateCost("anthropic/claude-sonnet-custom-v1", 10000, 10000, 0)
		if !ok {
			t.Fatalf("expected fallback pricing lookup to succeed")
		}
		// 10000 * 3.0 / 1e6 = 0.03
		// 10000 * 15.0 / 1e6 = 0.15
		// Total = 0.18
		expected := 0.18
		if diff := cost - expected; diff > 0.00001 || diff < -0.00001 {
			t.Errorf("cost = %v, want %v", cost, expected)
		}
	})

	t.Run("Unknown model returns false and 0", func(t *testing.T) {
		cost, ok := CalculateCost("unknown/model-xyz", 1000, 1000, 0)
		if ok {
			t.Errorf("expected unknown model pricing to fail, got cost = %v", cost)
		}
		if cost != 0 {
			t.Errorf("expected 0 cost for unknown model, got %v", cost)
		}
	})

	t.Run("Empty model returns false", func(t *testing.T) {
		_, ok := CalculateCost("", 1000, 1000, 0)
		if ok {
			t.Errorf("expected empty model string to fail")
		}
	})
}
