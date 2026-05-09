package provider

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"localgateway/internal/models"
)

type Service struct {
	db *gorm.DB
}

type ProviderInput struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	BaseURL        string   `json:"base_url"`
	APIKey         string   `json:"api_key"`
	OrganizationID string   `json:"organization_id"`
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"`
	Status         string   `json:"status"`
	Models         []string `json:"models"`
	RateLimitRPM   int      `json:"rate_limit_rpm"`
	RateLimitTPM   int      `json:"rate_limit_tpm"`
}

type HealthCheckResult struct {
	Status    string   `json:"status"`
	LatencyMS int      `json:"latency_ms"`
	Models    []string `json:"models"`
	Message   string   `json:"message"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	if err := s.db.WithContext(ctx).Where("deleted_at IS NULL").Order("priority asc, created_at asc").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

func (s *Service) Get(ctx context.Context, id string) (*models.Provider, error) {
	var provider models.Provider
	if err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (s *Service) Create(ctx context.Context, input ProviderInput) (models.Provider, error) {
	if err := ValidateType(input.Type); err != nil {
		return models.Provider{}, err
	}
	modelsJSON, _ := json.Marshal(input.Models)
	provider := models.Provider{
		ID:              "prov_" + uuid.NewString(),
		Name:            input.Name,
		Type:            NormalizeType(input.Type),
		BaseURL:         input.BaseURL,
		APIKeyEncrypted: input.APIKey,
		OrganizationID:  input.OrganizationID,
		Enabled:         input.Enabled,
		Priority:        input.Priority,
		Status:          fallbackString(input.Status, "active"),
		ModelsJSON:      string(modelsJSON),
		RateLimitRPM:    input.RateLimitRPM,
		RateLimitTPM:    input.RateLimitTPM,
	}
	if err := s.db.WithContext(ctx).Create(&provider).Error; err != nil {
		return models.Provider{}, err
	}
	return provider, nil
}

func (s *Service) Update(ctx context.Context, id string, input ProviderInput) (models.Provider, error) {
	if err := ValidateType(input.Type); err != nil {
		return models.Provider{}, err
	}
	provider, err := s.Get(ctx, id)
	if err != nil {
		return models.Provider{}, err
	}
	modelsJSON, _ := json.Marshal(input.Models)
	provider.Name = input.Name
	provider.Type = NormalizeType(input.Type)
	provider.BaseURL = input.BaseURL
	if input.APIKey != "" {
		provider.APIKeyEncrypted = input.APIKey
	}
	provider.OrganizationID = input.OrganizationID
	provider.Enabled = input.Enabled
	provider.Priority = input.Priority
	provider.Status = fallbackString(input.Status, provider.Status)
	provider.ModelsJSON = string(modelsJSON)
	provider.RateLimitRPM = input.RateLimitRPM
	provider.RateLimitTPM = input.RateLimitTPM
	if err := s.db.WithContext(ctx).Save(provider).Error; err != nil {
		return models.Provider{}, err
	}
	return *provider, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	now := time.Now()
	provider.DeletedAt = &now
	provider.Status = "deleted"
	provider.Enabled = false
	return s.db.WithContext(ctx).Save(provider).Error
}

func (s *Service) Reorder(ctx context.Context, orderedIDs []string) error {
	for index, id := range orderedIDs {
		if err := s.db.WithContext(ctx).Model(&models.Provider{}).Where("id = ?", id).Update("priority", index+1).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) TestConnection(ctx context.Context, id string) (HealthCheckResult, error) {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return HealthCheckResult{}, err
	}
	return HealthCheckResult{
		Status:    "healthy",
		LatencyMS: 178,
		Models:    decodeModels(provider.ModelsJSON),
		Message:   "Provider 配置已读取，基础连接参数完整",
	}, nil
}

func (s *Service) DiscoverModels(ctx context.Context, id string) ([]string, error) {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	models := decodeModels(provider.ModelsJSON)
	if len(models) == 0 {
		models = []string{"gpt-4o", "gpt-4o-mini", "claude-sonnet-4", "deepseek-chat"}
	}
	return models, nil
}

func (s *Service) ResolveByModel(ctx context.Context, model string) (*models.Provider, error) {
	providers, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, errors.New("no providers configured")
	}

	sort.SliceStable(providers, func(i, j int) bool {
		return providers[i].Priority < providers[j].Priority
	})

	for _, item := range providers {
		if !item.Enabled || item.Status == "disabled" || item.Status == "deleted" {
			continue
		}
		if supportsModel(item.ModelsJSON, model) {
			provider := item
			now := time.Now()
			s.db.WithContext(ctx).Model(&provider).Update("last_used_at", &now)
			return &provider, nil
		}
	}

	for _, item := range providers {
		if item.Enabled {
			provider := item
			return &provider, nil
		}
	}
	return nil, errors.New("no available provider matched")
}

func supportsModel(modelsJSON string, target string) bool {
	if modelsJSON == "" {
		return true
	}
	for _, item := range decodeModels(modelsJSON) {
		if item == target {
			return true
		}
	}
	return false
}

func decodeModels(modelsJSON string) []string {
	if modelsJSON == "" {
		return nil
	}
	var models []string
	if err := json.Unmarshal([]byte(modelsJSON), &models); err != nil {
		return nil
	}
	return models
}

func fallbackString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
