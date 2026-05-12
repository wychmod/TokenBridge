package pricing

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tokenbridge/internal/models"
)

const (
	litellmURL      = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	fallbackModelID = "gpt-5"
	fetchTimeout    = 30 * time.Second
)

//go:embed model_prices_and_context_window.json
var embeddedPricingFS embed.FS

type Service struct {
	db     *gorm.DB
	logger zerolog.Logger
}

type LookupResult struct {
	Pricing       *models.ModelPricing `json:"pricing"`
	Matched       bool                 `json:"matched"`
	FallbackUsed  bool                 `json:"fallback_used"`
	FallbackModel string               `json:"fallback_model,omitempty"`
}

type CostBreakdown struct {
	InputUSD         float64 `json:"input_usd"`
	OutputUSD        float64 `json:"output_usd"`
	CacheCreationUSD float64 `json:"cache_creation_usd"`
	CacheReadUSD     float64 `json:"cache_read_usd"`
	TotalUSD         float64 `json:"total_usd"`
	Matched          bool    `json:"matched"`
	FallbackUsed     bool    `json:"fallback_used"`
	FallbackModel    string  `json:"fallback_model,omitempty"`
}

type litellmModelEntry struct {
	LitellmProvider             string  `json:"litellm_provider"`
	Mode                        string  `json:"mode"`
	MaxInputTokens              int64   `json:"max_input_tokens"`
	MaxOutputTokens             int64   `json:"max_output_tokens"`
	InputCostPerToken           float64 `json:"input_cost_per_token"`
	OutputCostPerToken          float64 `json:"output_cost_per_token"`
	CacheCreationInputTokenCost float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64 `json:"cache_read_input_token_cost"`
	SupportsVision              bool    `json:"supports_vision"`
	SupportsFunctionCalling     bool    `json:"supports_function_calling"`
	SupportsPromptCaching       bool    `json:"supports_prompt_caching"`
	SupportsReasoning           bool    `json:"supports_reasoning"`
	DeprecationDate             string  `json:"deprecation_date"`
}

func NewService(db *gorm.DB, logger zerolog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Sync fetches pricing data from LiteLLM GitHub and upserts into SQLite.
// Returns the number of models synced. On failure, returns error but local cache remains untouched.
func (s *Service) Sync(ctx context.Context) (int, error) {
	s.logger.Info().Msg("pricing: syncing model prices from LiteLLM...")

	entries, err := s.fetchRemote(ctx)
	if err != nil {
		return 0, fmt.Errorf("pricing: fetch remote: %w", err)
	}

	count := s.upsert(ctx, entries)

	s.logger.Info().Int("models", count).Msg("pricing: sync complete")
	return count, nil
}

// SyncBestEffort tries to sync remote pricing first. If remote sync fails and local cache is empty,
// it initializes the cache from the embedded snapshot so first startup always has model pricing data.
func (s *Service) SyncBestEffort(ctx context.Context) {
	count, err := s.Sync(ctx)
	if err == nil {
		s.logger.Info().Int("models", count).Msg("pricing: models synced successfully")
		return
	}

	cached := s.Count(ctx)
	if cached > 0 {
		s.logger.Warn().Err(err).Int("cached_models", cached).Msg("pricing: remote sync failed, using local cache")
		return
	}

	fallbackCount, fallbackErr := s.SyncEmbedded(ctx)
	if fallbackErr != nil {
		s.logger.Error().Err(err).Err(fallbackErr).Msg("pricing: remote sync failed and embedded snapshot could not be loaded")
		return
	}
	s.logger.Warn().Err(err).Int("models", fallbackCount).Msg("pricing: remote sync failed, initialized from embedded snapshot")
}

// EnsureLocalCache loads the embedded snapshot when the local pricing table is empty.
func (s *Service) EnsureLocalCache(ctx context.Context) (int, error) {
	if count := s.Count(ctx); count > 0 {
		return count, nil
	}
	return s.SyncEmbedded(ctx)
}

// SyncEmbedded imports the embedded LiteLLM pricing snapshot into SQLite.
func (s *Service) SyncEmbedded(ctx context.Context) (int, error) {
	entries, err := s.loadEmbedded()
	if err != nil {
		return 0, fmt.Errorf("pricing: load embedded snapshot: %w", err)
	}
	count := s.upsert(ctx, entries)
	return count, nil
}

// Lookup finds the pricing entry for a model using exact match then prefix match.
func (s *Service) Lookup(ctx context.Context, modelID string) *models.ModelPricing {
	result := s.LookupWithFallback(ctx, modelID)
	return result.Pricing
}

// LookupWithFallback finds model pricing. If no direct/fuzzy match is found, it returns a conservative GPT-5 fallback.
func (s *Service) LookupWithFallback(ctx context.Context, modelID string) LookupResult {
	if modelID != "" {
		if entry := s.lookupExactOrFuzzy(ctx, modelID); entry != nil {
			return LookupResult{Pricing: entry, Matched: true}
		}
	}

	if fallback := s.lookupExactOrFuzzy(ctx, fallbackModelID); fallback != nil {
		return LookupResult{Pricing: fallback, Matched: false, FallbackUsed: true, FallbackModel: fallback.ModelID}
	}

	fallback := defaultFallbackPricing()
	return LookupResult{Pricing: &fallback, Matched: false, FallbackUsed: true, FallbackModel: fallback.ModelID}
}

// CalculateCost computes the USD cost for a request given model pricing and token counts.
func (s *Service) CalculateCost(ctx context.Context, modelID string, inputTokens, outputTokens int64) float64 {
	return s.CalculateCostDetailed(ctx, modelID, inputTokens, outputTokens, 0, 0).TotalUSD
}

// CalculateCostDetailed computes request cost using the same local pricing cache as CalculateCost,
// including prompt-cache write/read token rates when they are available.
func (s *Service) CalculateCostDetailed(ctx context.Context, modelID string, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int64) CostBreakdown {
	lookup := s.LookupWithFallback(ctx, modelID)
	p := lookup.Pricing
	if p == nil {
		return CostBreakdown{}
	}
	uncachedInput := inputTokens - cacheCreationTokens - cacheReadTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}
	cacheCreationRate := p.CacheCreationCostPerToken
	if cacheCreationRate == 0 {
		cacheCreationRate = p.InputCostPerToken
	}
	cacheReadRate := p.CacheReadCostPerToken
	if cacheReadRate == 0 {
		cacheReadRate = p.InputCostPerToken
	}
	breakdown := CostBreakdown{
		InputUSD:         float64(uncachedInput) * p.InputCostPerToken,
		OutputUSD:        float64(outputTokens) * p.OutputCostPerToken,
		CacheCreationUSD: float64(cacheCreationTokens) * cacheCreationRate,
		CacheReadUSD:     float64(cacheReadTokens) * cacheReadRate,
		Matched:          lookup.Matched,
		FallbackUsed:     lookup.FallbackUsed,
		FallbackModel:    lookup.FallbackModel,
	}
	breakdown.TotalUSD = breakdown.InputUSD + breakdown.OutputUSD + breakdown.CacheCreationUSD + breakdown.CacheReadUSD
	return breakdown
}

func (s *Service) lookupExactOrFuzzy(ctx context.Context, modelID string) *models.ModelPricing {
	if modelID == "" {
		return nil
	}

	var entry models.ModelPricing
	if err := s.db.WithContext(ctx).Where("model_id = ?", modelID).First(&entry).Error; err == nil {
		return &entry
	}

	normalized := normalizeModelID(modelID)
	if normalized != modelID {
		if err := s.db.WithContext(ctx).Where("model_id = ?", normalized).First(&entry).Error; err == nil {
			return &entry
		}
	}

	var candidates []models.ModelPricing
	prefix := normalized
	if err := s.db.WithContext(ctx).Where("model_id LIKE ? AND mode = ?", prefix+"%", "chat").Order("LENGTH(model_id) ASC").Limit(1).Find(&candidates).Error; err == nil && len(candidates) > 0 {
		return &candidates[0]
	}

	segments := strings.Split(normalized, "-")
	if len(segments) >= 2 {
		broadPrefix := segments[0] + "-" + segments[1]
		if err := s.db.WithContext(ctx).Where("model_id LIKE ? AND mode = ?", broadPrefix+"%", "chat").Order("LENGTH(model_id) ASC").Limit(1).Find(&candidates).Error; err == nil && len(candidates) > 0 {
			return &candidates[0]
		}
	}

	return nil
}

// Count returns how many pricing entries are cached locally.
func (s *Service) Count(ctx context.Context) int {
	var count int64
	s.db.WithContext(ctx).Model(&models.ModelPricing{}).Count(&count)
	return int(count)
}

// LastSync returns the FetchedAt time of the most recent sync, or zero time if never synced.
func (s *Service) LastSync(ctx context.Context) time.Time {
	var entry models.ModelPricing
	result := s.db.WithContext(ctx).Order("fetched_at DESC").Limit(1).Find(&entry)
	if result.Error != nil || result.RowsAffected == 0 {
		return time.Time{}
	}
	return entry.FetchedAt
}

// List returns all pricing entries for display in the admin UI.
func (s *Service) List(ctx context.Context, mode string) []models.ModelPricing {
	query := s.db.WithContext(ctx)
	if mode != "" {
		query = query.Where("mode = ?", mode)
	}
	var entries []models.ModelPricing
	query.Order("model_id ASC").Find(&entries)
	return entries
}

// fetchRemote downloads and parses the LiteLLM pricing JSON from GitHub.
func (s *Service) fetchRemote(ctx context.Context) (map[string]litellmModelEntry, error) {
	client := &http.Client{Timeout: fetchTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, litellmURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TokenBridge/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return parsePricingJSON(body)
}

func (s *Service) loadEmbedded() (map[string]litellmModelEntry, error) {
	body, err := embeddedPricingFS.ReadFile("model_prices_and_context_window.json")
	if err != nil {
		return nil, err
	}
	return parsePricingJSON(body)
}

func parsePricingJSON(body []byte) (map[string]litellmModelEntry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}

	entries := make(map[string]litellmModelEntry, len(raw))
	for key, val := range raw {
		if key == "sample_spec" || key == "MODEL_*" {
			continue
		}
		var entry litellmModelEntry
		if err := json.Unmarshal(val, &entry); err != nil {
			continue
		}
		if entry.InputCostPerToken > 0 || entry.OutputCostPerToken > 0 {
			entries[key] = entry
		}
	}

	return entries, nil
}

// upsert batch-inserts or updates pricing entries into SQLite using GORM's Upsert.
func (s *Service) upsert(ctx context.Context, entries map[string]litellmModelEntry) int {
	now := time.Now()
	count := 0

	// Process in batches of 500 to avoid SQLite variable limit
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic order

	batchSize := 500
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := make([]models.ModelPricing, 0, end-i)
		for _, key := range keys[i:end] {
			e := entries[key]
			batch = append(batch, models.ModelPricing{
				ModelID:                   key,
				LitellmProvider:           e.LitellmProvider,
				Mode:                      e.Mode,
				MaxInputTokens:            e.MaxInputTokens,
				MaxOutputTokens:           e.MaxOutputTokens,
				InputCostPerToken:         e.InputCostPerToken,
				OutputCostPerToken:        e.OutputCostPerToken,
				CacheCreationCostPerToken: e.CacheCreationInputTokenCost,
				CacheReadCostPerToken:     e.CacheReadInputTokenCost,
				SupportsVision:            e.SupportsVision,
				SupportsFunctionCalling:   e.SupportsFunctionCalling,
				SupportsPromptCaching:     e.SupportsPromptCaching,
				FetchedAt:                 now,
			})
		}

		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "model_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"litellm_provider", "mode", "max_input_tokens", "max_output_tokens", "input_cost_per_token", "output_cost_per_token", "cache_creation_cost_per_token", "cache_read_cost_per_token", "supports_vision", "supports_function_calling", "supports_prompt_caching", "fetched_at"}),
		}).CreateInBatches(batch, batchSize).Error; err != nil {
			s.logger.Error().Err(err).Int("batch_start", i).Msg("pricing: upsert batch failed")
			continue
		}
		count += len(batch)
	}

	return count
}

// normalizeModelID strips common provider prefixes to improve matching.
func defaultFallbackPricing() models.ModelPricing {
	return models.ModelPricing{
		ModelID:            fallbackModelID,
		LitellmProvider:    "openai",
		Mode:               "chat",
		InputCostPerToken:  1.25 / 1000000,
		OutputCostPerToken: 10.0 / 1000000,
		FetchedAt:          time.Now(),
	}
}

func normalizeModelID(modelID string) string {
	stripPrefixes := []string{
		"openai/",
		"anthropic/",
		"azure/",
		"bedrock/",
		"vertex_ai/",
		"cohere/",
		"deepseek/",
		"ollama/",
		"groq/",
		"together_ai/",
		"mistral/",
		"replicate/",
		"huggingface/",
		"fireworks_ai/",
		"perplexity/",
		"anyscale/",
		"palm/",
		"voyage/",
		"databricks/",
		"codestral/",
	}
	lower := strings.ToLower(modelID)
	for _, prefix := range stripPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return modelID[len(prefix):]
		}
	}
	return modelID
}
