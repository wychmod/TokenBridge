package routing

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
	"tokenbridge/internal/provider"
)

type Decision struct {
	Strategy string          `json:"strategy"`
	Provider models.Provider `json:"provider"`
	Model    string          `json:"model"`
	Fallback []string        `json:"fallback"`
}

type RuleInput struct {
	ModelPattern  string   `json:"model_pattern"`
	Strategy      string   `json:"strategy"`
	ProviderChain []string `json:"provider_chain"`
	FallbackChain []string `json:"fallback_chain"`
	Enabled       bool     `json:"enabled"`
}

type AliasInput struct {
	Alias         string   `json:"alias"`
	Target        string   `json:"target"`
	FallbackChain []string `json:"fallback_chain"`
}

type TestInput struct {
	Model     string `json:"model"`
	LocalKey  string `json:"local_key"`
	Format    string `json:"format"`
	Streaming bool   `json:"streaming"`
}

type TestResult struct {
	ResolvedModel    string   `json:"resolved_model"`
	ProviderID       string   `json:"provider_id"`
	ProviderName     string   `json:"provider_name"`
	Strategy         string   `json:"strategy"`
	FallbackChain    []string `json:"fallback_chain"`
	EstimatedCost    string   `json:"estimated_cost"`
	EstimatedTTFT    string   `json:"estimated_ttft"`
	FormatCompatible bool     `json:"format_compatible"`
	FormatWarning    string   `json:"format_warning,omitempty"`
	SimulationScope  string   `json:"simulation_scope"`
}

type Service struct {
	db        *gorm.DB
	providers *provider.Service
	strategy  string
}

func NewService(db *gorm.DB, providers *provider.Service, strategy string) *Service {
	return &Service{db: db, providers: providers, strategy: strategy}
}

func (s *Service) ListRules(ctx context.Context) ([]models.RoutingRule, error) {
	var rules []models.RoutingRule
	if err := s.db.WithContext(ctx).Order("created_at asc").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *Service) CreateRule(ctx context.Context, input RuleInput) (models.RoutingRule, error) {
	rule := models.RoutingRule{ID: "route_" + uuid.NewString(), ModelPattern: input.ModelPattern, Strategy: input.Strategy, ProviderChain: toJSON(input.ProviderChain), FallbackChain: toJSON(input.FallbackChain), Enabled: input.Enabled}
	if err := s.db.WithContext(ctx).Create(&rule).Error; err != nil {
		return models.RoutingRule{}, err
	}
	return rule, nil
}

func (s *Service) UpdateRule(ctx context.Context, id string, input RuleInput) (models.RoutingRule, error) {
	var rule models.RoutingRule
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&rule).Error; err != nil {
		return models.RoutingRule{}, err
	}
	rule.ModelPattern = input.ModelPattern
	rule.Strategy = input.Strategy
	rule.ProviderChain = toJSON(input.ProviderChain)
	rule.FallbackChain = toJSON(input.FallbackChain)
	rule.Enabled = input.Enabled
	if err := s.db.WithContext(ctx).Save(&rule).Error; err != nil {
		return models.RoutingRule{}, err
	}
	return rule, nil
}

func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.RoutingRule{}, "id = ?", id).Error
}

func (s *Service) ListAliases(ctx context.Context) ([]models.ModelAlias, error) {
	var aliases []models.ModelAlias
	if err := s.db.WithContext(ctx).Order("created_at asc").Find(&aliases).Error; err != nil {
		return nil, err
	}
	return aliases, nil
}

func (s *Service) UpsertAlias(ctx context.Context, input AliasInput) (models.ModelAlias, error) {
	var alias models.ModelAlias
	result := s.db.WithContext(ctx).Where("alias = ?", input.Alias).First(&alias)
	if result.Error != nil {
		alias = models.ModelAlias{ID: "alias_" + uuid.NewString(), Alias: input.Alias}
	}
	alias.Target = input.Target
	alias.FallbackChain = toJSON(input.FallbackChain)
	if err := s.db.WithContext(ctx).Save(&alias).Error; err != nil {
		return models.ModelAlias{}, err
	}
	return alias, nil
}

func (s *Service) Decide(ctx context.Context, model string) (*Decision, error) {
	resolved, aliasFallback := s.resolveAlias(ctx, model)
	ruleProviderChain, ruleFallback := s.matchRule(ctx, resolved)

	selected, err := s.selectProvider(ctx, resolved, ruleProviderChain)
	if err != nil {
		return nil, err
	}

	fallback := ruleFallback
	if len(fallback) == 0 {
		fallback = aliasFallback
	}

	return &Decision{Strategy: s.strategy, Provider: *selected, Model: resolved, Fallback: fallback}, nil
}

func (s *Service) Simulate(ctx context.Context, input TestInput) (TestResult, error) {
	decision, err := s.Decide(ctx, input.Model)
	if err != nil {
		return TestResult{}, err
	}
	formatCompatible := true
	formatWarning := ""
	if strings.TrimSpace(input.Format) != "" {
		if err := provider.ValidateFormatCompatibility(decision.Provider.Type, input.Format); err != nil {
			formatCompatible = false
			formatWarning = err.Error()
		}
	}
	return TestResult{
		ResolvedModel:    decision.Model,
		ProviderID:       decision.Provider.ID,
		ProviderName:     decision.Provider.Name,
		Strategy:         decision.Strategy,
		FallbackChain:    decision.Fallback,
		EstimatedCost:    "未实际请求",
		EstimatedTTFT:    "未实际请求",
		FormatCompatible: formatCompatible,
		FormatWarning:    formatWarning,
		SimulationScope:  "仅模拟路由决策，未发送上游请求；不会校验 Local Key 权限、实际费用或真实延迟。",
	}, nil
}

func (s *Service) resolveAlias(ctx context.Context, model string) (string, []string) {
	var aliases []models.ModelAlias
	if err := s.db.WithContext(ctx).Find(&aliases).Error; err != nil {
		return model, nil
	}
	for _, alias := range aliases {
		if strings.EqualFold(alias.Alias, model) {
			return alias.Target, decodeJSONList(alias.FallbackChain)
		}
	}
	return model, nil
}

func (s *Service) matchRule(ctx context.Context, model string) ([]string, []string) {
	rules, err := s.ListRules(ctx)
	if err != nil {
		return nil, nil
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if wildcardMatch(rule.ModelPattern, model) {
			return decodeJSONList(rule.ProviderChain), decodeJSONList(rule.FallbackChain)
		}
	}
	return nil, nil
}

func (s *Service) selectProvider(ctx context.Context, model string, preferred []string) (*models.Provider, error) {
	if len(preferred) == 0 {
		return s.providers.ResolveByModel(ctx, model)
	}
	providers, err := s.providers.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, providerID := range preferred {
		for _, item := range providers {
			if strings.EqualFold(item.ID, providerID) && item.Enabled && item.Status != "disabled" && item.Status != "deleted" && supportsModelJSON(item.ModelsJSON, model) {
				provider := item
				return &provider, nil
			}
		}
	}
	return s.providers.ResolveByModel(ctx, model)
}

func wildcardMatch(pattern string, value string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(strings.TrimSuffix(pattern, "*")))
	}
	return strings.EqualFold(pattern, value)
}

func decodeJSONList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []string
	_ = json.Unmarshal([]byte(raw), &items)
	return items
}

func supportsModelJSON(modelsJSON string, target string) bool {
	if strings.TrimSpace(modelsJSON) == "" {
		return true
	}
	for _, item := range decodeJSONList(modelsJSON) {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

func toJSON(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(values)
	return string(data)
}
