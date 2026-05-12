package usage

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

type Service struct {
	db *gorm.DB
}

type RecordInput struct {
	LocalKeyID     string
	ProviderID     string
	ModelRequested string
	ModelActual    string
	APIFormat      string
	InputTokens    int64
	OutputTokens   int64
	TotalCostUSD   float64
	LatencyMS      int64
	Success        bool
}

type Summary struct {
	TotalRequests int64   `json:"total_requests"`
	SuccessRate   float64 `json:"success_rate"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	InputTokens   int64   `json:"input_tokens"`
	OutputTokens  int64   `json:"output_tokens"`
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
	return s.db.WithContext(ctx).Create(&models.UsageRecord{
		ID:             "req_" + uuid.NewString(),
		LocalKeyID:     input.LocalKeyID,
		ProviderID:     input.ProviderID,
		ModelRequested: input.ModelRequested,
		ModelActual:    input.ModelActual,
		APIFormat:      input.APIFormat,
		InputTokens:    input.InputTokens,
		OutputTokens:   input.OutputTokens,
		TotalCostUSD:   input.TotalCostUSD,
		LatencyMS:      input.LatencyMS,
		Success:        input.Success,
		CreatedAt:      time.Now(),
	}).Error
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
	if err := s.db.WithContext(ctx).Where("created_at >= ?", time.Now().AddDate(0, 0, -days+1)).Find(&records).Error; err != nil {
		return nil, err
	}
	buckets := map[string]*TrendPoint{}
	for _, item := range records {
		key := item.CreatedAt.Format("2006-01-02")
		point, ok := buckets[key]
		if !ok {
			point = &TrendPoint{Day: item.CreatedAt.Format("01-02")}
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
