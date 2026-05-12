package requestlog

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

type Service struct {
	db *gorm.DB
}

type Item struct {
	ID            string         `json:"id"`
	LocalKeyID    string         `json:"local_key_id"`
	ProviderID    string         `json:"provider_id"`
	Path          string         `json:"path"`
	Method        string         `json:"method"`
	StatusCode    int            `json:"status_code"`
	LatencyMS     int64          `json:"latency_ms"`
	ErrorMessage  string         `json:"error_message"`
	Metadata      map[string]any `json:"metadata"`
	CreatedAt     time.Time      `json:"created_at"`
	StatusLabel   string         `json:"status_label"`
	Detail        string         `json:"detail"`
	TraceID       string         `json:"trace_id"`
	FallbackUsed  bool           `json:"fallback_used"`
	FallbackTried []string       `json:"fallback_tried"`
}

type Query struct {
	Text         string
	TraceID      string
	OnlyFallback bool
	Provider     string
	APIFormat    string
	Status       string
	From         string
	To           string
	Limit        int
}

type Stats struct {
	Total        int `json:"total"`
	Failures     int `json:"failures"`
	Fallbacks    int `json:"fallbacks"`
	AvgLatencyMS int `json:"avg_latency_ms"`
}

type TrendPoint struct {
	Day       string `json:"day"`
	Failures  int    `json:"failures"`
	Fallbacks int    `json:"fallbacks"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, query string, onlyFallback bool, limit int) ([]Item, error) {
	return s.ListWithQuery(ctx, Query{Text: query, OnlyFallback: onlyFallback, Limit: limit})
}

func (s *Service) ListWithQuery(ctx context.Context, query Query) ([]Item, error) {
	if query.Limit <= 0 || query.Limit > 1000 {
		query.Limit = 200
	}

	var rows []models.RequestLog
	tx := s.db.WithContext(ctx).Order("created_at desc").Limit(query.Limit)
	if query.Text != "" {
		like := "%" + strings.ToLower(query.Text) + "%"
		tx = tx.Where("lower(path) LIKE ? OR lower(method) LIKE ? OR lower(error_message) LIKE ? OR lower(metadata_json) LIKE ?", like, like, like, like)
	}
	if query.TraceID != "" {
		like := "%" + strings.ToLower(query.TraceID) + "%"
		tx = tx.Where("lower(metadata_json) LIKE ?", like)
	}
	if query.Provider != "" {
		like := "%" + strings.ToLower(query.Provider) + "%"
		tx = tx.Where("lower(provider_id) LIKE ? OR lower(metadata_json) LIKE ?", like, like)
	}
	if query.APIFormat != "" {
		like := "%\"apiformat\":\"" + strings.ToLower(query.APIFormat) + "\"%"
		tx = tx.Where("lower(metadata_json) LIKE ?", like)
	}
	if query.Status == "failed" {
		tx = tx.Where("status_code >= 400 OR error_message <> ''")
	}
	if query.From != "" {
		if parsed, err := time.Parse("2006-01-02", query.From); err == nil {
			tx = tx.Where("created_at >= ?", parsed)
		}
	}
	if query.To != "" {
		if parsed, err := time.Parse("2006-01-02", query.To); err == nil {
			tx = tx.Where("created_at < ?", parsed.Add(24*time.Hour))
		}
	}
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		metadata := map[string]any{}
		_ = json.Unmarshal([]byte(row.MetadataJSON), &metadata)
		fallbackTried := toStringSlice(metadata["fallbackTried"])
		fallbackUsed := len(fallbackTried) > 0
		if query.OnlyFallback && !fallbackUsed {
			continue
		}
		traceID, _ := metadata["id"].(string)
		provider, _ := metadata["provider"].(string)
		if provider == "" {
			provider = row.ProviderID
		}
		statusLabel := "成功"
		detail := "请求执行完成"
		if row.ErrorMessage != "" || row.StatusCode >= 400 {
			statusLabel = "失败"
			detail = row.ErrorMessage
			if detail == "" {
				detail = "请求执行失败"
			}
		}
		if fallbackUsed {
			statusLabel = "已切换备用"
			detail = "主链路失败后已尝试备用 Provider：" + strings.Join(fallbackTried, "、")
		}
		items = append(items, Item{ID: row.ID, LocalKeyID: row.LocalKeyID, ProviderID: provider, Path: row.Path, Method: row.Method, StatusCode: row.StatusCode, LatencyMS: row.LatencyMS, ErrorMessage: row.ErrorMessage, Metadata: metadata, CreatedAt: row.CreatedAt, StatusLabel: statusLabel, Detail: detail, TraceID: traceID, FallbackUsed: fallbackUsed, FallbackTried: fallbackTried})
	}
	return items, nil
}

func (s *Service) ExportCSV(ctx context.Context, query Query) ([]byte, error) {
	query.Limit = 1000
	items, err := s.ListWithQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	_ = writer.Write([]string{"时间", "路径", "方法", "Provider", "状态", "状态码", "耗时(ms)", "TraceID", "Fallback", "请求模型", "实际模型", "错误信息"})
	for _, item := range items {
		_ = writer.Write([]string{
			item.CreatedAt.Format(time.RFC3339),
			item.Path,
			item.Method,
			item.ProviderID,
			item.StatusLabel,
			intToString(item.StatusCode),
			int64ToString(item.LatencyMS),
			item.TraceID,
			strings.Join(item.FallbackTried, " -> "),
			toString(item.Metadata["requestedModel"]),
			toString(item.Metadata["actualModel"]),
			item.ErrorMessage,
		})
	}
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	items, err := s.ListWithQuery(ctx, Query{Limit: 200})
	if err != nil {
		return Stats{}, err
	}
	stats := Stats{Total: len(items)}
	var latency int64
	for _, item := range items {
		latency += item.LatencyMS
		if item.StatusLabel == "失败" {
			stats.Failures++
		}
		if item.FallbackUsed {
			stats.Fallbacks++
		}
	}
	if len(items) > 0 {
		stats.AvgLatencyMS = int(latency / int64(len(items)))
	}
	return stats, nil
}

func (s *Service) Trend(ctx context.Context, days int) ([]TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	items, err := s.ListWithQuery(ctx, Query{Limit: 500, From: time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")})
	if err != nil {
		return nil, err
	}
	buckets := map[string]*TrendPoint{}
	for _, item := range items {
		key := item.CreatedAt.Format("2006-01-02")
		point, ok := buckets[key]
		if !ok {
			point = &TrendPoint{Day: item.CreatedAt.Format("01-02")}
			buckets[key] = point
		}
		if item.StatusLabel == "失败" {
			point.Failures++
		}
		if item.FallbackUsed {
			point.Fallbacks++
		}
	}
	result := make([]TrendPoint, 0, len(buckets))
	for _, item := range buckets {
		result = append(result, *item)
	}
	return result, nil
}

func toStringSlice(value any) []string {
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

func toString(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func intToString(v int) string {
	return strconv.Itoa(v)
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
