package pricing

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

func TestCalculateCostDetailedUsesCacheAndReasoningRates(t *testing.T) {
	db := openPricingTestDB(t)
	svc := NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:                   "gpt-test",
		Mode:                      "chat",
		InputCostPerToken:         1.0 / 1_000_000,
		OutputCostPerToken:        2.0 / 1_000_000,
		CacheCreationCostPerToken: 0.5 / 1_000_000,
		CacheReadCostPerToken:     0.1 / 1_000_000,
		PricingJSON:               `{"output_cost_per_reasoning_token":0.000004}`,
		FetchedAt:                 now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	breakdown := svc.CalculateCostDetailed(context.Background(), CostInput{
		ModelID:             "gpt-test",
		InputTokens:         1000,
		OutputTokens:        300,
		CacheCreationTokens: 100,
		CacheReadTokens:     200,
		ReasoningTokens:     50,
	})

	want := 700.0/1_000_000 + 100.0*0.5/1_000_000 + 200.0*0.1/1_000_000 + 250.0*2.0/1_000_000 + 50.0*4.0/1_000_000
	if math.Abs(breakdown.TotalUSD-want) > 0.000000001 {
		t.Fatalf("unexpected total: got %.12f want %.12f (%+v)", breakdown.TotalUSD, want, breakdown)
	}
	if breakdown.ReasoningUSD == 0 {
		t.Fatalf("expected reasoning-specific cost, got %+v", breakdown)
	}
	if breakdown.PricingRuleJSON == "" || breakdown.CostBreakdownJSON == "" {
		t.Fatalf("expected serialized pricing evidence, got %+v", breakdown)
	}
}

func TestCalculateCostDetailedUsesPriorityAboveContextTier(t *testing.T) {
	db := openPricingTestDB(t)
	svc := NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:            "gpt-tiered",
		Mode:               "chat",
		InputCostPerToken:  1,
		OutputCostPerToken: 10,
		PricingJSON:        `{"input_cost_per_token_above_200k_tokens_priority":4,"output_cost_per_token_above_200k_tokens_priority":40,"cache_read_input_token_cost_above_200k_tokens_priority":0.4}`,
		FetchedAt:          now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	breakdown := svc.CalculateCostDetailed(context.Background(), CostInput{
		ModelID:         "gpt-tiered",
		InputTokens:     10,
		OutputTokens:    5,
		CacheReadTokens: 2,
		ContextWindow:   250000,
		PricingTier:     "priority",
	})

	if breakdown.InputUSD != 32 || breakdown.CacheReadUSD != 0.8 || breakdown.OutputUSD != 200 {
		t.Fatalf("expected above-200k priority rates, got %+v", breakdown)
	}
}

func TestCalculateCostDetailedUsesExplicitReasoningEffortTier(t *testing.T) {
	db := openPricingTestDB(t)
	svc := NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:            "gpt-effort",
		Mode:               "chat",
		InputCostPerToken:  1,
		OutputCostPerToken: 10,
		PricingJSON:        `{"output_cost_per_token_high":25}`,
		FetchedAt:          now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	breakdown := svc.CalculateCostDetailed(context.Background(), CostInput{
		ModelID:      "gpt-effort",
		InputTokens:  1,
		OutputTokens: 2,
		PricingTier:  "high",
	})

	if breakdown.OutputUSD != 50 {
		t.Fatalf("expected explicit high-effort output rate, got %+v", breakdown)
	}

	baseBreakdown := svc.CalculateCostDetailed(context.Background(), CostInput{
		ModelID:      "gpt-effort",
		InputTokens:  1,
		OutputTokens: 2,
		PricingTier:  "xhigh",
	})
	if baseBreakdown.OutputUSD != 20 {
		t.Fatalf("expected unknown effort tier to fall back to base rate, got %+v", baseBreakdown)
	}
}

func TestCalculateCostDetailedUsesTieredReasoningRateAndSkipsOneHourCacheRate(t *testing.T) {
	db := openPricingTestDB(t)
	svc := NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:                   "gpt-reasoning-tier",
		Mode:                      "chat",
		InputCostPerToken:         1,
		OutputCostPerToken:        10,
		CacheCreationCostPerToken: 2,
		PricingJSON:               `{"cache_creation_input_token_cost_above_1hr":100,"output_cost_per_reasoning_token":20,"output_cost_per_reasoning_token_high":40}`,
		FetchedAt:                 now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	breakdown := svc.CalculateCostDetailed(context.Background(), CostInput{
		ModelID:             "gpt-reasoning-tier",
		InputTokens:         100,
		OutputTokens:        10,
		CacheCreationTokens: 20,
		ReasoningTokens:     5,
		PricingTier:         "high",
	})

	if breakdown.CacheCreationUSD != 40 {
		t.Fatalf("expected normal cache creation rate to ignore above_1hr without explicit tier, got %+v", breakdown)
	}
	if breakdown.ReasoningUSD != 200 {
		t.Fatalf("expected high-tier reasoning token rate, got %+v", breakdown)
	}
}

func openPricingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ModelPricing{}); err != nil {
		t.Fatal(err)
	}
	return db
}
