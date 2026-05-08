package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"localgateway/internal/models"
)

type Service struct {
	db *gorm.DB
}

type CreateKeyInput struct {
	Name             string      `json:"name"`
	AllowedModels    []string    `json:"allowed_models"`
	AllowedProviders []string    `json:"allowed_providers"`
	MonthlyBudget    float64     `json:"monthly_budget"`
	TokenBudget      int64       `json:"token_budget"`
	Enabled          bool        `json:"enabled"`
	ExpiresAt        *time.Time  `json:"expires_at"`
}

type UpdateKeyInput struct {
	Name             string      `json:"name"`
	AllowedModels    []string    `json:"allowed_models"`
	AllowedProviders []string    `json:"allowed_providers"`
	MonthlyBudget    float64     `json:"monthly_budget"`
	TokenBudget      int64       `json:"token_budget"`
	Enabled          bool        `json:"enabled"`
	ExpiresAt        *time.Time  `json:"expires_at"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, input CreateKeyInput) (models.LocalKey, string, error) {
	rawKey := generateLocalKey()
	entity := models.LocalKey{
		ID:                   "key_" + uuid.NewString(),
		Name:                 input.Name,
		KeyHash:              hashKey(rawKey),
		AllowedModelsJSON:    toJSON(input.AllowedModels),
		AllowedProvidersJSON: toJSON(input.AllowedProviders),
		MonthlyBudget:        input.MonthlyBudget,
		TokenBudget:          input.TokenBudget,
		Enabled:              input.Enabled,
		ExpiresAt:            input.ExpiresAt,
	}
	if err := s.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return models.LocalKey{}, "", err
	}
	entity.DisplayKey = maskKey(rawKey)
	return entity, rawKey, nil
}

func (s *Service) List(ctx context.Context) ([]models.LocalKey, error) {
	var keys []models.LocalKey
	if err := s.db.WithContext(ctx).Order("created_at desc").Find(&keys).Error; err != nil {
		return nil, err
	}
	for i := range keys {
		keys[i].DisplayKey = "lg-****"
	}
	return keys, nil
}

func (s *Service) Get(ctx context.Context, id string) (*models.LocalKey, error) {
	var entity models.LocalKey
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	entity.DisplayKey = "lg-****"
	return &entity, nil
}

func (s *Service) Update(ctx context.Context, id string, input UpdateKeyInput) (models.LocalKey, error) {
	entity, err := s.Get(ctx, id)
	if err != nil {
		return models.LocalKey{}, err
	}
	entity.Name = input.Name
	entity.AllowedModelsJSON = toJSON(input.AllowedModels)
	entity.AllowedProvidersJSON = toJSON(input.AllowedProviders)
	entity.MonthlyBudget = input.MonthlyBudget
	entity.TokenBudget = input.TokenBudget
	entity.Enabled = input.Enabled
	entity.ExpiresAt = input.ExpiresAt
	if err := s.db.WithContext(ctx).Save(entity).Error; err != nil {
		return models.LocalKey{}, err
	}
	entity.DisplayKey = "lg-****"
	return *entity, nil
}

func (s *Service) Revoke(ctx context.Context, id string) error {
	entity, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	now := time.Now()
	entity.Enabled = false
	entity.RevokedAt = &now
	return s.db.WithContext(ctx).Save(entity).Error
}

func (s *Service) Extend(ctx context.Context, id string, expiresAt *time.Time) (models.LocalKey, error) {
	entity, err := s.Get(ctx, id)
	if err != nil {
		return models.LocalKey{}, err
	}
	entity.ExpiresAt = expiresAt
	if err := s.db.WithContext(ctx).Save(entity).Error; err != nil {
		return models.LocalKey{}, err
	}
	entity.DisplayKey = "lg-****"
	return *entity, nil
}

func (s *Service) Rotate(ctx context.Context, id string) (models.LocalKey, string, error) {
	entity, err := s.Get(ctx, id)
	if err != nil {
		return models.LocalKey{}, "", err
	}
	rawKey := generateLocalKey()
	entity.KeyHash = hashKey(rawKey)
	entity.Enabled = true
	entity.RevokedAt = nil
	if err := s.db.WithContext(ctx).Save(entity).Error; err != nil {
		return models.LocalKey{}, "", err
	}
	entity.DisplayKey = maskKey(rawKey)
	return *entity, rawKey, nil
}

func (s *Service) Validate(ctx context.Context, key string) (*models.LocalKey, error) {
	var entity models.LocalKey
	if err := s.db.WithContext(ctx).Where("key_hash = ?", hashKey(key)).First(&entity).Error; err != nil {
		return nil, err
	}
	if !entity.Enabled {
		return nil, errors.New("local key disabled")
	}
	if entity.RevokedAt != nil {
		return nil, errors.New("local key revoked")
	}
	if entity.ExpiresAt != nil && entity.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("local key expired")
	}
	// Budget enforcement: reject if monthly budget is set and exceeded
	if entity.MonthlyBudget > 0 && entity.CurrentSpend >= entity.MonthlyBudget {
		return nil, errors.New("monthly budget exceeded")
	}
	// Token budget enforcement: reject if token budget is set and exceeded
	if entity.TokenBudget > 0 && entity.CurrentTokens >= entity.TokenBudget {
		return nil, errors.New("token budget exceeded")
	}
	now := time.Now()
	s.db.WithContext(ctx).Model(&entity).Update("last_used_at", &now)
	return &entity, nil
}

// DeductUsage atomically increments CurrentSpend and CurrentTokens for a LocalKey after a successful request.
func (s *Service) DeductUsage(ctx context.Context, keyID string, costUSD float64, inputTokens, outputTokens int64) error {
	if keyID == "" {
		return nil
	}
	totalTokens := inputTokens + outputTokens
	return s.db.WithContext(ctx).Model(&models.LocalKey{}).Where("id = ?", keyID).Updates(map[string]any{
		"current_spend":  gorm.Expr("current_spend + ?", costUSD),
		"current_tokens": gorm.Expr("current_tokens + ?", totalTokens),
	}).Error
}

func generateLocalKey() string {
	seed := strings.ReplaceAll(uuid.NewString(), "-", "")
	return "lg-" + seed[:16] + "_" + strings.ToLower(seed[16:20])
}

func hashKey(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func maskKey(value string) string {
	if len(value) < 12 {
		return "lg-****"
	}
	return value[:7] + "****" + value[len(value)-4:]
}
