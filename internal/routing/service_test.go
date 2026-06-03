package routing

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
	"tokenbridge/internal/provider"
)

func TestSimulateReportsDecisionScopeAndFormatCompatibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Provider{}, &models.RoutingRule{}, &models.ModelAlias{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&models.Provider{
		ID:         "prov-openai",
		Name:       "OpenAI 主线路",
		Type:       provider.TypeOpenAI,
		Enabled:    true,
		Status:     "active",
		Priority:   1,
		ModelsJSON: `["gpt-4o-mini"]`,
	}).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := db.Create(&models.Provider{
		ID:         "prov-claude",
		Name:       "Claude 备用线路",
		Type:       provider.TypeAnthropic,
		Enabled:    true,
		Status:     "active",
		Priority:   2,
		ModelsJSON: `["claude-sonnet-4"]`,
	}).Error; err != nil {
		t.Fatalf("create fallback provider: %v", err)
	}
	if err := db.Create(&models.ModelAlias{
		ID:            "alias-fast",
		Alias:         "gpt-fast",
		Target:        "gpt-4o-mini",
		FallbackChain: `["prov-claude"]`,
	}).Error; err != nil {
		t.Fatalf("create alias: %v", err)
	}
	if err := db.Create(&models.RoutingRule{
		ID:            "route-gpt",
		ModelPattern:  "gpt-4o*",
		Strategy:      "priority",
		ProviderChain: `["prov-openai"]`,
		FallbackChain: `["prov-claude"]`,
		Enabled:       true,
	}).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}

	svc := NewService(db, provider.NewService(db), "priority")
	result, err := svc.Simulate(context.Background(), TestInput{Model: "gpt-fast", Format: provider.APIFormatClaude})
	if err != nil {
		t.Fatalf("Simulate() error = %v", err)
	}

	if result.ResolvedModel != "gpt-4o-mini" {
		t.Fatalf("ResolvedModel = %q, want alias target", result.ResolvedModel)
	}
	if result.ProviderID != "prov-openai" || result.ProviderName != "OpenAI 主线路" {
		t.Fatalf("provider = %q/%q, want selected Provider id and name", result.ProviderID, result.ProviderName)
	}
	if result.FormatCompatible {
		t.Fatal("FormatCompatible = true, want false for Claude request against OpenAI provider")
	}
	if !strings.Contains(result.FormatWarning, "不兼容") {
		t.Fatalf("FormatWarning = %q, want compatibility explanation", result.FormatWarning)
	}
	if result.EstimatedCost != "未实际请求" || result.EstimatedTTFT != "未实际请求" {
		t.Fatalf("estimates = %q/%q, want no fake request metrics", result.EstimatedCost, result.EstimatedTTFT)
	}
	if !strings.Contains(result.SimulationScope, "未发送上游请求") {
		t.Fatalf("SimulationScope = %q, want decision-only scope", result.SimulationScope)
	}
	if len(result.FallbackChain) != 1 || result.FallbackChain[0] != "prov-claude" {
		t.Fatalf("FallbackChain = %#v, want rule fallback chain", result.FallbackChain)
	}
}
