package usage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tokenbridge/internal/models"
)

type Service struct {
	db *gorm.DB
}

type RecordInput struct {
	LocalKeyID          string
	ProviderID          string
	ModelRequested      string
	ModelActual         string
	APIFormat           string
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	ReasoningTokens     int64
	ContextWindow       int64
	PricingTier         string
	TotalCostUSD        float64
	CostBreakdownJSON   string
	PricingRuleJSON     string
	TimeSource          string
	EventKey            string
	ParserVersion       int
	LatencyMS           int64
	Success             bool
}

type Summary struct {
	TotalRequests       int64   `json:"total_requests"`
	SuccessRate         float64 `json:"success_rate"`
	TotalCostUSD        float64 `json:"total_cost_usd"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	ReasoningTokens     int64   `json:"reasoning_tokens"`
}

type TrendPoint struct {
	Day      string  `json:"day"`
	Cost     float64 `json:"cost"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
}

type ProviderBreakdown struct {
	Name        string  `json:"name"`
	Cost        float64 `json:"cost"`
	Requests    int64   `json:"requests"`
	Tokens      int64   `json:"tokens"`
	SuccessRate float64 `json:"success_rate"`
}

type ModelBreakdown struct {
	Name     string  `json:"name"`
	Cost     float64 `json:"cost"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
}

type KeyBreakdown struct {
	Name     string  `json:"name"`
	Cost     float64 `json:"cost"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(ctx context.Context, input RecordInput) error {
	now := time.Now().UTC()
	row := models.UsageRecord{
		ID:                  gatewayUsageRecordID(input),
		LocalKeyID:          input.LocalKeyID,
		ProviderID:          input.ProviderID,
		ModelRequested:      input.ModelRequested,
		ModelActual:         input.ModelActual,
		APIFormat:           input.APIFormat,
		InputTokens:         input.InputTokens,
		OutputTokens:        input.OutputTokens,
		CacheCreationTokens: input.CacheCreationTokens,
		CacheReadTokens:     input.CacheReadTokens,
		ReasoningTokens:     input.ReasoningTokens,
		ContextWindow:       input.ContextWindow,
		PricingTier:         input.PricingTier,
		TotalCostUSD:        input.TotalCostUSD,
		CostBreakdownJSON:   input.CostBreakdownJSON,
		PricingRuleJSON:     input.PricingRuleJSON,
		TimeSource:          nonEmpty(input.TimeSource, "gateway_created_at"),
		EventKey:            input.EventKey,
		ParserVersion:       input.ParserVersion,
		LatencyMS:           input.LatencyMS,
		Success:             input.Success,
		CreatedAt:           now,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"local_key_id", "provider_id", "model_requested", "model_actual", "api_format",
			"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "reasoning_tokens",
			"context_window", "pricing_tier", "total_cost_usd", "cost_breakdown_json", "pricing_rule_json",
			"time_source", "event_key", "parser_version", "latency_ms", "success",
		}),
	}).Create(&row).Error
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	var records []models.UsageRecord
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return Summary{}, err
	}

	var summary Summary
	var success int64
	for _, item := range records {
		summary.TotalRequests++
		summary.TotalCostUSD += item.TotalCostUSD
		summary.InputTokens += item.InputTokens
		summary.OutputTokens += item.OutputTokens
		summary.CacheCreationTokens += item.CacheCreationTokens
		summary.CacheReadTokens += item.CacheReadTokens
		summary.ReasoningTokens += item.ReasoningTokens
		if item.Success {
			success++
		}
	}
	if summary.TotalRequests > 0 {
		summary.SuccessRate = float64(success) / float64(summary.TotalRequests)
	}
	return summary, nil
}

func (s *Service) Trend(ctx context.Context, days int) ([]TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	var records []models.UsageRecord
	location := beijingLocation()
	now := time.Now().In(location)
	startLocal := startOfDay(now.AddDate(0, 0, -days+1))
	if err := s.db.WithContext(ctx).Where("created_at >= ?", startLocal.UTC()).Find(&records).Error; err != nil {
		return nil, err
	}
	buckets := map[string]*TrendPoint{}
	for _, item := range records {
		localCreated := item.CreatedAt.In(location)
		key := localCreated.Format("2006-01-02")
		point, ok := buckets[key]
		if !ok {
			point = &TrendPoint{Day: localCreated.Format("01-02")}
			buckets[key] = point
		}
		point.Cost += item.TotalCostUSD
		point.Requests++
		point.Tokens += item.InputTokens + item.OutputTokens
	}
	points := make([]TrendPoint, 0, len(buckets))
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		points = append(points, *buckets[key])
	}
	return points, nil
}

func (s *Service) ProviderBreakdown(ctx context.Context) ([]ProviderBreakdown, error) {
	var records []models.UsageRecord
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	buckets := map[string]*ProviderBreakdown{}
	successMap := map[string]int64{}
	for _, item := range records {
		name := item.ProviderID
		bucket, ok := buckets[name]
		if !ok {
			bucket = &ProviderBreakdown{Name: name}
			buckets[name] = bucket
		}
		bucket.Cost += item.TotalCostUSD
		bucket.Requests++
		bucket.Tokens += item.InputTokens + item.OutputTokens
		if item.Success {
			successMap[name]++
		}
	}
	result := make([]ProviderBreakdown, 0, len(buckets))
	for name, bucket := range buckets {
		if bucket.Requests > 0 {
			bucket.SuccessRate = float64(successMap[name]) / float64(bucket.Requests)
		}
		result = append(result, *bucket)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Cost > result[j].Cost })
	return result, nil
}

func beijingLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func startOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func nonEmpty(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func gatewayUsageRecordID(input RecordInput) string {
	if strings.TrimSpace(input.EventKey) == "" {
		return "req_" + uuid.NewString()
	}
	parts := []string{
		input.APIFormat,
		input.LocalKeyID,
		input.ProviderID,
		input.ModelActual,
		input.EventKey,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "req_" + hex.EncodeToString(sum[:])[:32]
}

func (s *Service) ModelBreakdown(ctx context.Context) ([]ModelBreakdown, error) {
	var records []models.UsageRecord
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	buckets := map[string]*ModelBreakdown{}
	for _, item := range records {
		name := item.ModelActual
		if name == "" {
			name = item.ModelRequested
		}
		bucket, ok := buckets[name]
		if !ok {
			bucket = &ModelBreakdown{Name: name}
			buckets[name] = bucket
		}
		bucket.Cost += item.TotalCostUSD
		bucket.Requests++
		bucket.Tokens += item.InputTokens + item.OutputTokens
	}
	result := make([]ModelBreakdown, 0, len(buckets))
	for _, bucket := range buckets {
		result = append(result, *bucket)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Requests > result[j].Requests })
	return result, nil
}

func (s *Service) KeyBreakdown(ctx context.Context) ([]KeyBreakdown, error) {
	var records []models.UsageRecord
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}
	buckets := map[string]*KeyBreakdown{}
	for _, item := range records {
		name := item.LocalKeyID
		if name == "" {
			name = "anonymous"
		}
		bucket, ok := buckets[name]
		if !ok {
			bucket = &KeyBreakdown{Name: name}
			buckets[name] = bucket
		}
		bucket.Cost += item.TotalCostUSD
		bucket.Requests++
		bucket.Tokens += item.InputTokens + item.OutputTokens
	}
	result := make([]KeyBreakdown, 0, len(buckets))
	for _, bucket := range buckets {
		result = append(result, *bucket)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Cost > result[j].Cost })
	return result, nil
}
