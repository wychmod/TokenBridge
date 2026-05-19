package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

type Service struct {
	db         *gorm.DB
	httpClient *http.Client
}

type ProviderInput struct {
	ID             string   `json:"id"`
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

type modelProbeResult struct {
	LatencyMS int
	Models    []string
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
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
	return s.testConnection(ctx, *provider)
}

func (s *Service) TestConnectionInput(ctx context.Context, input ProviderInput) (HealthCheckResult, error) {
	if err := ValidateType(input.Type); err != nil {
		return HealthCheckResult{}, err
	}
	return s.testConnection(ctx, s.providerFromInput(ctx, input))
}

func (s *Service) testConnection(ctx context.Context, provider models.Provider) (HealthCheckResult, error) {
	configuredModels := decodeModels(provider.ModelsJSON)
	probe, err := s.probeModels(ctx, provider)
	if err != nil {
		return HealthCheckResult{
			Status:    "warning",
			LatencyMS: probe.LatencyMS,
			Models:    configuredModels,
			Message:   err.Error(),
		}, nil
	}

	discoveredModels := probe.Models
	if len(discoveredModels) == 0 {
		discoveredModels = configuredModels
	}
	message := "供应商连接正常"
	if len(discoveredModels) > 0 {
		message = fmt.Sprintf("供应商连接正常，模型列表可读取（%d 个）", len(discoveredModels))
	}
	return HealthCheckResult{
		Status:    "healthy",
		LatencyMS: probe.LatencyMS,
		Models:    discoveredModels,
		Message:   message,
	}, nil
}

func (s *Service) DiscoverModels(ctx context.Context, id string) ([]string, error) {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.discoverModels(ctx, *provider)
}

func (s *Service) DiscoverModelsInput(ctx context.Context, input ProviderInput) ([]string, error) {
	if err := ValidateType(input.Type); err != nil {
		return nil, err
	}
	return s.discoverModels(ctx, s.providerFromInput(ctx, input))
}

func (s *Service) discoverModels(ctx context.Context, provider models.Provider) ([]string, error) {
	probe, err := s.probeModels(ctx, provider)
	if err != nil {
		return nil, err
	}
	if len(probe.Models) == 0 {
		return nil, errors.New("供应商响应中未发现模型 ID")
	}
	return probe.Models, nil
}

func (s *Service) probeModels(ctx context.Context, provider models.Provider) (modelProbeResult, error) {
	startedAt := time.Now()
	endpoint, err := buildProbeEndpoint(provider.BaseURL)
	if err != nil {
		return modelProbeResult{LatencyMS: elapsedMilliseconds(startedAt)}, fmt.Errorf("供应商 base_url 无效：%v", err)
	}

	apiKey := strings.TrimSpace(provider.APIKeyEncrypted)
	if apiKey == "" {
		return modelProbeResult{LatencyMS: elapsedMilliseconds(startedAt)}, errors.New("请填写 API Key/Token 后再测试连接")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return modelProbeResult{LatencyMS: elapsedMilliseconds(startedAt)}, fmt.Errorf("构造供应商探测请求失败：%v", err)
	}
	req.Header.Set("Accept", "application/json")
	setProviderProbeHeaders(req, provider, apiKey)

	client := s.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	latencyMS := elapsedMilliseconds(startedAt)
	if err != nil {
		return modelProbeResult{LatencyMS: latencyMS}, fmt.Errorf("连接供应商失败：%v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return modelProbeResult{LatencyMS: latencyMS}, errors.New(probeFailureMessage(resp.StatusCode, body))
	}

	return modelProbeResult{
		LatencyMS: latencyMS,
		Models:    extractModelIDs(body),
	}, nil
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

func (s *Service) providerFromInput(ctx context.Context, input ProviderInput) models.Provider {
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" && input.ID != "" {
		if saved, err := s.Get(ctx, input.ID); err == nil {
			apiKey = saved.APIKeyEncrypted
		}
	}
	modelsJSON, _ := json.Marshal(input.Models)
	return models.Provider{
		ID:              input.ID,
		Name:            input.Name,
		Type:            NormalizeType(input.Type),
		BaseURL:         input.BaseURL,
		APIKeyEncrypted: apiKey,
		OrganizationID:  input.OrganizationID,
		ModelsJSON:      string(modelsJSON),
	}
}

func setProviderProbeHeaders(req *http.Request, provider models.Provider, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if IsAnthropic(provider.Type) {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		return
	}
	if provider.OrganizationID != "" {
		req.Header.Set("OpenAI-Organization", provider.OrganizationID)
	}
}

func buildProbeEndpoint(baseURL string) (string, error) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "", fmt.Errorf("Provider 未配置 base_url")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base_url 必须包含协议和主机")
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	probePath := "/v1/models"
	if strings.HasSuffix(basePath, "/v1") {
		probePath = "/models"
	}
	parsed.Path = basePath + probePath
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func elapsedMilliseconds(startedAt time.Time) int {
	ms := time.Since(startedAt).Milliseconds()
	if ms < 0 {
		return 0
	}
	return int(ms)
}

func extractModelIDs(body []byte) []string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	var models []string
	for _, key := range []string{"data", "models"} {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		models = append(models, extractModelIDsFromArray(raw)...)
		if len(models) > 0 {
			return uniqueStrings(models)
		}
	}

	var list []string
	if err := json.Unmarshal(body, &list); err == nil {
		return uniqueStrings(list)
	}
	return nil
}

func extractModelIDsFromArray(raw json.RawMessage) []string {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	models := make([]string, 0, len(items))
	for _, item := range items {
		var id string
		if err := json.Unmarshal(item, &id); err == nil {
			if strings.TrimSpace(id) != "" {
				models = append(models, strings.TrimSpace(id))
			}
			continue
		}

		var object map[string]any
		if err := json.Unmarshal(item, &object); err != nil {
			continue
		}
		for _, key := range []string{"id", "name", "model"} {
			if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
				models = append(models, strings.TrimSpace(value))
				break
			}
		}
	}
	return models
}

func uniqueStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func probeFailureMessage(statusCode int, body []byte) string {
	detail := extractUpstreamErrorMessage(body)
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if detail == "" {
			return fmt.Sprintf("认证失败（%d）：请检查 API Key/Token", statusCode)
		}
		return fmt.Sprintf("认证失败（%d）：%s", statusCode, detail)
	default:
		if detail == "" {
			return fmt.Sprintf("供应商返回 HTTP %d", statusCode)
		}
		return fmt.Sprintf("供应商返回 HTTP %d：%s", statusCode, detail)
	}
}

func extractUpstreamErrorMessage(body []byte) string {
	var payload struct {
		Error   any    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Message != "" {
			return payload.Message
		}
		switch value := payload.Error.(type) {
		case string:
			return value
		case map[string]any:
			if message, ok := value["message"].(string); ok {
				return message
			}
			if code, ok := value["code"].(string); ok {
				return code
			}
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 180 {
		return text[:180]
	}
	return text
}

func MaskAPIKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 7 {
		return "****"
	}
	return trimmed[:3] + "..." + trimmed[len(trimmed)-4:]
}
