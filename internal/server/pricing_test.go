package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
)

func TestPricingEstimateUsesDetailedCostInput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ModelPricing{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.ModelPricing{
		ModelID:               "gpt-estimate",
		Mode:                  "chat",
		InputCostPerToken:     1,
		OutputCostPerToken:    10,
		CacheReadCostPerToken: 0.5,
		PricingJSON:           `{"output_cost_per_token_high":25,"output_cost_per_reasoning_token":40}`,
		FetchedAt:             time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatal(err)
	}

	handler := newPricingHandlers(pricing.NewService(db, zerolog.Nop()))
	req := httptest.NewRequest(http.MethodGet, "/admin/api/pricing/estimate?model=gpt-estimate&input_tokens=100&output_tokens=20&cache_read_tokens=30&reasoning_tokens=5&pricing_tier=high&context_window=250000", nil)
	res := httptest.NewRecorder()

	handler.handleEstimate(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var body struct {
		EstimatedUSD float64 `json:"estimated_usd"`
		PricingTier  string  `json:"pricing_tier"`
		Context      int64   `json:"context_window"`
		Breakdown    struct {
			InputUSD     float64 `json:"input_usd"`
			CacheReadUSD float64 `json:"cache_read_usd"`
			ReasoningUSD float64 `json:"reasoning_usd"`
			OutputUSD    float64 `json:"output_usd"`
		} `json:"cost_breakdown"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PricingTier != "high" || body.Context != 250000 {
		t.Fatalf("expected estimate to preserve tier/context, got %+v", body)
	}
	if body.Breakdown.InputUSD != 70 || body.Breakdown.CacheReadUSD != 15 || body.Breakdown.ReasoningUSD != 200 || body.Breakdown.OutputUSD != 375 {
		t.Fatalf("unexpected detailed estimate breakdown: %+v", body)
	}
	if body.EstimatedUSD != 660 {
		t.Fatalf("unexpected total estimate: %+v", body)
	}
}
