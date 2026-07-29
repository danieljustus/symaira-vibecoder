package pricing

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed pricing.json
var embeddedPricingJSON []byte

type Rates struct {
	InputCostPerM     float64 `json:"input_cost_per_m"`
	OutputCostPerM    float64 `json:"output_cost_per_m"`
	CacheReadCostPerM float64 `json:"cache_read_cost_per_m"`
}

type Dataset struct {
	UpdatedAt string           `json:"updated_at"`
	Models    map[string]Rates `json:"models"`
	Fallbacks map[string]Rates `json:"fallbacks"`
}

var (
	datasetOnce sync.Once
	dataset     Dataset
)

func loadDataset() {
	datasetOnce.Do(func() {
		_ = json.Unmarshal(embeddedPricingJSON, &dataset)
	})
}

// GetRates performs a two-tier lookup (exact provider/model key, then fallback model family).
// Returns (rates, ok). ok is false if no pricing rate is known.
func GetRates(providerModel string) (Rates, bool) {
	loadDataset()
	if providerModel == "" {
		return Rates{}, false
	}
	key := strings.ToLower(strings.TrimSpace(providerModel))

	// 1. Exact match in models
	if r, ok := dataset.Models[key]; ok {
		return r, true
	}

	// 2. Match without provider prefix if provided as provider/model
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		if r, ok := dataset.Models[parts[1]]; ok {
			return r, true
		}
	}

	// 3. Fallback matching by key containment
	for fallbackKey, r := range dataset.Fallbacks {
		if strings.Contains(key, strings.ToLower(fallbackKey)) {
			return r, true
		}
	}

	return Rates{}, false
}

// CalculateCost computes total USD cost for token usage.
// Returns (costUSD, ok). ok is false if model rates are unknown.
func CalculateCost(providerModel string, inputTokens, outputTokens, cacheReadTokens int) (float64, bool) {
	rates, ok := GetRates(providerModel)
	if !ok {
		return 0, false
	}
	cost := (float64(inputTokens)*rates.InputCostPerM +
		float64(outputTokens)*rates.OutputCostPerM +
		float64(cacheReadTokens)*rates.CacheReadCostPerM) / 1_000_000.0
	return cost, true
}
