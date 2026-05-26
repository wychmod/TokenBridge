package server

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
	"tokenbridge/internal/usage"
)

func TestBuildProviderEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		operationPath string
		want          string
	}{
		{
			name:          "openai root base appends v1 chat completions",
			baseURL:       "https://api.openai.com",
			operationPath: "/v1/chat/completions",
			want:          "https://api.openai.com/v1/chat/completions",
		},
		{
			name:          "openai v1 base does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/v1",
			operationPath: "/v1/chat/completions",
			want:          "https://api.minimaxi.com/v1/chat/completions",
		},
		{
			name:          "openai v1 base trailing slash does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/v1/",
			operationPath: "/v1/chat/completions",
			want:          "https://api.minimaxi.com/v1/chat/completions",
		},
		{
			name:          "anthropic prefix base appends v1 messages",
			baseURL:       "https://api.minimaxi.com/anthropic",
			operationPath: "/v1/messages",
			want:          "https://api.minimaxi.com/anthropic/v1/messages",
		},
		{
			name:          "anthropic v1 prefix base does not duplicate v1",
			baseURL:       "https://api.minimaxi.com/anthropic/v1",
			operationPath: "/v1/messages",
			want:          "https://api.minimaxi.com/anthropic/v1/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildProviderEndpoint(tt.baseURL, tt.operationPath)
			if err != nil {
				t.Fatalf("buildProviderEndpoint() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildProviderEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildProviderEndpointRejectsEmptyBaseURL(t *testing.T) {
	_, err := buildProviderEndpoint("   ", "/v1/chat/completions")
	if err == nil {
		t.Fatal("buildProviderEndpoint() expected error for empty baseURL")
	}
}

func TestRecordUsageBestEffortPersistsOpenAICacheAndReasoningDetails(t *testing.T) {
	db := openServerUsageTestDB(t)
	pricingSvc := pricing.NewService(db, zerolog.Nop())
	usageSvc := usage.NewService(db)
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:               "gpt-5.5",
		Mode:                  "chat",
		InputCostPerToken:     1.0 / 1_000_000,
		OutputCostPerToken:    2.0 / 1_000_000,
		CacheReadCostPerToken: 0.1 / 1_000_000,
		PricingJSON:           `{"output_cost_per_reasoning_token":0.000004}`,
		FetchedAt:             now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	resp := openAIChatResponse{Model: "gpt-5.5"}
	resp.Usage.PromptTokens = 1000
	resp.Usage.CompletionTokens = 200
	resp.Usage.PromptTokensDetails.CachedTokens = 300
	resp.Usage.CompletionTokensDetails.ReasoningTokens = 40
	recordUsageBestEffort(context.Background(), usageSvc, pricingSvc, nil, "key_1", "provider_1", "gpt-5.5", "gpt-5.5", "openai", 42, true, resp)

	var row models.UsageRecord
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CacheReadTokens != 300 || row.ReasoningTokens != 40 || row.CostBreakdownJSON == "" || row.PricingRuleJSON == "" {
		t.Fatalf("expected detailed usage fields, got %+v", row)
	}
	if row.TimeSource != "gateway_created_at" || row.ParserVersion == 0 {
		t.Fatalf("expected gateway time source and parser version, got %+v", row)
	}
	if math.Abs(row.TotalCostUSD-0.00121) > 0.00000001 {
		t.Fatalf("unexpected detailed cost: %.8f", row.TotalCostUSD)
	}
}

func TestRecordClaudeUsageBestEffortPersistsCacheDetails(t *testing.T) {
	db := openServerUsageTestDB(t)
	pricingSvc := pricing.NewService(db, zerolog.Nop())
	usageSvc := usage.NewService(db)
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if err := db.Create(&models.ModelPricing{
		ModelID:                   "claude-sonnet",
		Mode:                      "chat",
		InputCostPerToken:         3.0 / 1_000_000,
		OutputCostPerToken:        15.0 / 1_000_000,
		CacheCreationCostPerToken: 3.75 / 1_000_000,
		CacheReadCostPerToken:     0.3 / 1_000_000,
		FetchedAt:                 now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	resp := claudeMessagesResponse{Model: "claude-sonnet"}
	resp.Usage.InputTokens = 1000
	resp.Usage.OutputTokens = 100
	resp.Usage.CacheCreationInputTokens = 200
	resp.Usage.CacheReadInputTokens = 300
	recordClaudeUsageBestEffort(context.Background(), usageSvc, pricingSvc, nil, "key_1", "provider_1", "claude-sonnet", "claude-sonnet", "claude", 42, true, resp)

	var row models.UsageRecord
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.CacheCreationTokens != 200 || row.CacheReadTokens != 300 {
		t.Fatalf("expected Claude cache usage details, got %+v", row)
	}
	if row.InputTokens != 1500 {
		t.Fatalf("expected Claude separate cache input to be normalized into input tokens, got %+v", row)
	}
	if math.Abs(row.TotalCostUSD-0.00534) > 0.00000001 {
		t.Fatalf("unexpected Claude cache-aware cost: %.8f", row.TotalCostUSD)
	}
}

func TestClaudeStreamUsageParserKeepsCacheDetails(t *testing.T) {
	var usage claudeStreamUsage
	parseClaudeStreamUsageLine("message_start", `data: {"message":{"usage":{"input_tokens":1000,"cache_creation_input_tokens":200,"cache_read_input_tokens":300}}}`, &usage)
	parseClaudeStreamUsageLine("message_delta", `data: {"usage":{"output_tokens":120}}`, &usage)

	if usage.InputTokens != 1000 || usage.CacheCreationInputTokens != 200 || usage.CacheReadInputTokens != 300 || usage.OutputTokens != 120 {
		t.Fatalf("expected Claude stream usage details, got %+v", usage)
	}
	if usage.NormalizedInputTokens() != 1500 {
		t.Fatalf("expected normalized stream input to include cache tokens, got %d", usage.NormalizedInputTokens())
	}
}

func openServerUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ModelPricing{}, &models.UsageRecord{}); err != nil {
		t.Fatal(err)
	}
	return db
}
