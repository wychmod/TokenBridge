package aitoolusage

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
)

const (
	maxLogFileSize = 64 * 1024 * 1024
	parserVersion  = 4
)

type Service struct {
	db      *gorm.DB
	prices  *pricing.Service
	logger  zerolog.Logger
	nowFunc func() time.Time
}

type ScanResult struct {
	FilesSeen           int       `json:"files_seen"`
	FilesScanned        int       `json:"files_scanned"`
	FilesSkipped        int       `json:"files_skipped"`
	RecordsFound        int64     `json:"records_found"`
	RecordsCreated      int64     `json:"records_created"`
	DuplicateGroups     int64     `json:"duplicate_groups"`
	DroppedRecords      int64     `json:"dropped_records"`
	TimeFallbackRecords int64     `json:"time_fallback_records"`
	PricingFallbacks    int64     `json:"pricing_fallbacks"`
	CompletedAt         time.Time `json:"completed_at"`
}

type Summary struct {
	TotalCostUSD     float64 `json:"total_cost_usd"`
	TotalRequests    int64   `json:"total_requests"`
	TotalTokens      int64   `json:"total_tokens"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheCreation    int64   `json:"cache_creation_tokens"`
	CacheRead        int64   `json:"cache_read_tokens"`
	ReasoningTokens  int64   `json:"reasoning_tokens"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	PricingFallbacks int64   `json:"pricing_fallbacks"`
	TimeFallbacks    int64   `json:"time_fallbacks"`
	LocalOnly        bool    `json:"local_only"`
	ScannedSources   int64   `json:"scanned_sources"`
	LastScan         string  `json:"last_scan"`
}

type Breakdown struct {
	Name             string  `json:"name"`
	Tool             string  `json:"tool,omitempty"`
	CostUSD          float64 `json:"cost_usd"`
	Requests         int64   `json:"requests"`
	Tokens           int64   `json:"tokens"`
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheReadTokens  int64   `json:"cache_read_tokens"`
	ReasoningTokens  int64   `json:"reasoning_tokens"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	PricingFallbacks int64   `json:"pricing_fallbacks"`
	LastSeen         string  `json:"last_seen,omitempty"`
}

type TrendPoint struct {
	Day       string  `json:"day"`
	CostUSD   float64 `json:"cost_usd"`
	Requests  int64   `json:"requests"`
	Tokens    int64   `json:"tokens"`
	CacheRead int64   `json:"cache_read_tokens"`
}

type HeatmapPoint struct {
	Day      string  `json:"day"`
	Hour     int     `json:"hour"`
	CostUSD  float64 `json:"cost_usd"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
}

type SourceInfo struct {
	Tool           string `json:"tool"`
	Path           string `json:"path"`
	RecordsFound   int64  `json:"records_found"`
	RecordsCreated int64  `json:"records_created"`
	LastScannedAt  string `json:"last_scanned_at"`
	ErrorMessage   string `json:"error_message"`
}

type Dashboard struct {
	Summary       Summary                      `json:"summary"`
	Trend         []TrendPoint                 `json:"trend"`
	Heatmap       []HeatmapPoint               `json:"heatmap"`
	ModelRank     []Breakdown                  `json:"model_rank"`
	ProjectSpend  []Breakdown                  `json:"project_spend"`
	ToolBreakdown []Breakdown                  `json:"tool_breakdown"`
	Sources       []SourceInfo                 `json:"sources"`
	Recent        []models.AICodingUsageRecord `json:"recent"`
}

type RealtimeSnapshot struct {
	Today     Summary      `json:"today"`
	Total     Summary      `json:"total"`
	Trend     []TrendPoint `json:"trend"`
	TopTool   *Breakdown   `json:"top_tool,omitempty"`
	TopModel  *Breakdown   `json:"top_model,omitempty"`
	UpdatedAt string       `json:"updated_at"`
	LocalOnly bool         `json:"local_only"`
}

type UsageEvent struct {
	Tool                string
	SessionID           string
	RequestID           string
	EventKey            string
	ProjectPath         string
	ProjectName         string
	Model               string
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	ReasoningTokens     int64
	ContextWindow       int64
	PricingTier         string
	TotalTokens         int64
	SourcePath          string
	SourceOffset        int64
	TimeSource          string
	RawJSON             string
	OccurredAt          time.Time
}

type parsedRecord = UsageEvent

type logCandidate struct {
	tool string
	path string
}

func NewService(db *gorm.DB, prices *pricing.Service, logger zerolog.Logger) *Service {
	return &Service{db: db, prices: prices, logger: logger, nowFunc: time.Now}
}

func (s *Service) Scan(ctx context.Context) (ScanResult, error) {
	result := ScanResult{CompletedAt: s.nowFunc()}
	result.DroppedRecords += s.cleanupStoredRecords(ctx)
	candidates := discoverLogFiles()
	result.FilesSeen = len(candidates)
	for _, candidate := range candidates {
		scanned, found, created, err := s.scanFile(ctx, candidate)
		if scanned {
			result.FilesScanned++
		} else {
			result.FilesSkipped++
		}
		result.RecordsFound += found
		result.RecordsCreated += created
		if err != nil {
			s.logger.Debug().Err(err).Str("path", candidate.path).Msg("ai tool usage: scan file failed")
		}
	}
	result.DroppedRecords += s.cleanupStoredRecords(ctx)
	result.DuplicateGroups, _ = s.duplicateAudit(ctx)
	result.TimeFallbackRecords = s.countRows(ctx, "time_source = ?", "file_mod_time")
	result.PricingFallbacks = s.countRows(ctx, "pricing_matched = ?", false)
	return result, nil
}

func (s *Service) cleanupStoredRecords(ctx context.Context) int64 {
	var dropped int64
	dropped += s.execCleanup(ctx, `
		DELETE FROM ai_coding_usage_records
		WHERE input_tokens = 0
		  AND output_tokens = 0
		  AND total_tokens = 0
		  AND (cache_read_tokens > 0 OR cache_creation_tokens > 0)
	`)
	dropped += s.execCleanup(ctx, `
		DELETE FROM ai_coding_usage_records
		WHERE event_key <> ''
		  AND id IN (
		    SELECT id
		    FROM (
		      SELECT id,
		             ROW_NUMBER() OVER (
		               PARTITION BY tool, event_key
		               ORDER BY total_tokens DESC, created_at ASC, id ASC
		             ) AS row_num
		      FROM ai_coding_usage_records
		      WHERE event_key <> ''
		    )
		    WHERE row_num > 1
		  )
	`)
	dropped += s.execCleanup(ctx, `
		DELETE FROM ai_coding_usage_records
		WHERE request_id <> ''
		  AND id IN (
		    SELECT id
		    FROM (
		      SELECT id,
		             ROW_NUMBER() OVER (
		               PARTITION BY tool, session_id, request_id
		               ORDER BY total_tokens DESC, created_at ASC, id ASC
		             ) AS row_num
		      FROM ai_coding_usage_records
		      WHERE request_id <> ''
		    )
		    WHERE row_num > 1
		  )
	`)
	dropped += s.execCleanup(ctx, `
		DELETE FROM ai_coding_usage_records
		WHERE lower(model) = 'unknown'
		  AND (lower(source_path) LIKE '%.jsonl' OR lower(source_path) LIKE '%.ndjson' OR lower(source_path) LIKE '%.log')
		  AND EXISTS (
		    SELECT 1
		    FROM ai_coding_usage_records AS known
		    WHERE known.tool = ai_coding_usage_records.tool
		      AND known.source_path = ai_coding_usage_records.source_path
		      AND known.source_offset = ai_coding_usage_records.source_offset
		      AND known.id <> ai_coding_usage_records.id
		      AND lower(known.model) <> 'unknown'
		  )
	`)
	dropped += s.execCleanup(ctx, `
		DELETE FROM ai_coding_usage_records
		WHERE (lower(source_path) LIKE '%.jsonl' OR lower(source_path) LIKE '%.ndjson' OR lower(source_path) LIKE '%.log')
		  AND id IN (
		    SELECT id
		    FROM (
		      SELECT id,
		             ROW_NUMBER() OVER (
		               PARTITION BY tool, source_path, source_offset
		               ORDER BY total_tokens ASC, created_at ASC, id ASC
		             ) AS row_num
		      FROM ai_coding_usage_records
		      WHERE lower(source_path) LIKE '%.jsonl'
		         OR lower(source_path) LIKE '%.ndjson'
		         OR lower(source_path) LIKE '%.log'
		    )
		    WHERE row_num > 1
		  )
	`)
	return dropped
}

func (s *Service) execCleanup(ctx context.Context, statement string) int64 {
	tx := s.db.WithContext(ctx).Exec(statement)
	if tx.Error != nil {
		return 0
	}
	return tx.RowsAffected
}

func (s *Service) duplicateAudit(ctx context.Context) (int64, int64) {
	var groups int64
	_ = s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			SELECT tool, event_key, COUNT(*) AS rows
			FROM ai_coding_usage_records
			WHERE event_key <> ''
			GROUP BY tool, event_key
			HAVING COUNT(*) > 1
		)
	`).Scan(&groups).Error
	var dropped int64
	_ = s.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(rows - 1), 0) FROM (
			SELECT COUNT(*) AS rows
			FROM ai_coding_usage_records
			WHERE event_key <> ''
			GROUP BY tool, event_key
			HAVING COUNT(*) > 1
		)
	`).Scan(&dropped).Error
	return groups, dropped
}

func (s *Service) countRows(ctx context.Context, query string, args ...any) int64 {
	var count int64
	_ = s.db.WithContext(ctx).Model(&models.AICodingUsageRecord{}).Where(query, args...).Count(&count).Error
	return count
}

func (s *Service) Dashboard(ctx context.Context, days int) (Dashboard, error) {
	if days <= 0 {
		days = 30
	}
	localNow := s.nowFunc()
	location := beijingLocation()
	startLocal := startOfDay(localNow.In(location).AddDate(0, 0, -days+1))
	endLocal := startOfDay(localNow.In(location).AddDate(0, 0, 1))
	var records []models.AICodingUsageRecord
	if err := s.db.WithContext(ctx).
		Where("occurred_at >= ? AND occurred_at < ?", startLocal.UTC(), endLocal.UTC()).
		Order("occurred_at asc").
		Find(&records).Error; err != nil {
		return Dashboard{}, err
	}

	var sourceRows []models.AICodingLogSource
	if err := s.db.WithContext(ctx).Order("records_created desc, records_found desc, last_scanned_at desc").Limit(24).Find(&sourceRows).Error; err != nil {
		return Dashboard{}, err
	}

	var sourceCount int64
	s.db.WithContext(ctx).Model(&models.AICodingLogSource{}).Count(&sourceCount)

	dashboard := Dashboard{
		Summary: Summary{LocalOnly: true, ScannedSources: sourceCount},
		Trend:   make([]TrendPoint, 0, days),
	}
	var lastScan time.Time
	for _, source := range sourceRows {
		if source.LastScannedAt.After(lastScan) {
			lastScan = source.LastScannedAt
		}
		dashboard.Sources = append(dashboard.Sources, SourceInfo{
			Tool:           source.Tool,
			Path:           source.Path,
			RecordsFound:   source.RecordsFound,
			RecordsCreated: source.RecordsCreated,
			LastScannedAt:  source.LastScannedAt.Format(time.RFC3339),
			ErrorMessage:   source.ErrorMessage,
		})
	}
	if !lastScan.IsZero() {
		dashboard.Summary.LastScan = lastScan.Format(time.RFC3339)
	}

	trend := map[string]*TrendPoint{}
	heatmap := map[string]*HeatmapPoint{}
	modelsByName := map[string]*Breakdown{}
	projectsByName := map[string]*Breakdown{}
	toolsByName := map[string]*Breakdown{}

	for _, record := range records {
		localRecord := record
		localRecord.OccurredAt = record.OccurredAt.In(location)
		if localRecord.OccurredAt.Before(startLocal) || !localRecord.OccurredAt.Before(endLocal) {
			continue
		}

		addSummary(&dashboard.Summary, localRecord)
		dayKey := localRecord.OccurredAt.Format("2006-01-02")
		if _, ok := trend[dayKey]; !ok {
			trend[dayKey] = &TrendPoint{Day: localRecord.OccurredAt.Format("01-02")}
		}
		trend[dayKey].CostUSD += localRecord.TotalCostUSD
		trend[dayKey].Requests++
		trend[dayKey].Tokens += localRecord.TotalTokens
		trend[dayKey].CacheRead += localRecord.CacheReadTokens

		heatKey := fmt.Sprintf("%s-%02d", dayKey, localRecord.OccurredAt.Hour())
		if _, ok := heatmap[heatKey]; !ok {
			heatmap[heatKey] = &HeatmapPoint{Day: localRecord.OccurredAt.Format("01-02"), Hour: localRecord.OccurredAt.Hour()}
		}
		heatmap[heatKey].CostUSD += localRecord.TotalCostUSD
		heatmap[heatKey].Requests++
		heatmap[heatKey].Tokens += localRecord.TotalTokens

		addBreakdown(modelsByName, nonEmpty(localRecord.Model, "unknown"), "", localRecord)
		addBreakdown(projectsByName, nonEmpty(localRecord.ProjectName, "unknown"), localRecord.Tool, localRecord)
		addBreakdown(toolsByName, localRecord.Tool, "", localRecord)
	}
	finalizeSummary(&dashboard.Summary)
	dashboard.Trend = completeTrend(trend, days, localNow.In(location))
	dashboard.Heatmap = completeHeatmap(heatmap, days, localNow.In(location))
	dashboard.ModelRank = sortedBreakdowns(modelsByName, 10)
	dashboard.ProjectSpend = sortedBreakdowns(projectsByName, 10)
	dashboard.ToolBreakdown = sortedBreakdowns(toolsByName, 10)
	if err := s.db.WithContext(ctx).Order("occurred_at desc").Limit(20).Find(&dashboard.Recent).Error; err != nil {
		return Dashboard{}, err
	}
	return dashboard, nil
}

func (s *Service) RealtimeSnapshot(ctx context.Context) (RealtimeSnapshot, error) {
	localNow := s.nowFunc()
	location := beijingLocation()
	startLocal := startOfDay(localNow.In(location))
	endLocal := startLocal.AddDate(0, 0, 1)

	today, err := s.aggregateSummary(ctx, &startLocal, &endLocal)
	if err != nil {
		return RealtimeSnapshot{}, err
	}
	total, err := s.aggregateSummary(ctx, nil, nil)
	if err != nil {
		return RealtimeSnapshot{}, err
	}

	dashboard, err := s.Dashboard(ctx, 7)
	if err != nil {
		return RealtimeSnapshot{}, err
	}

	result := RealtimeSnapshot{
		Today:     today,
		Total:     total,
		Trend:     dashboard.Trend,
		UpdatedAt: localNow.Format(time.RFC3339),
		LocalOnly: true,
	}
	if len(dashboard.ToolBreakdown) > 0 {
		tool := dashboard.ToolBreakdown[0]
		result.TopTool = &tool
	}
	if len(dashboard.ModelRank) > 0 {
		model := dashboard.ModelRank[0]
		result.TopModel = &model
	}
	return result, nil
}

func (s *Service) Export(ctx context.Context, format string, days int, exchangeRate float64) ([]byte, string, string, error) {
	dashboard, err := s.Dashboard(ctx, days)
	if err != nil {
		return nil, "", "", err
	}
	if exchangeRate <= 0 {
		exchangeRate = 7.2
	}
	stamp := s.nowFunc().Format("20060102-150405")
	switch strings.ToLower(format) {
	case "json":
		body, err := json.MarshalIndent(dashboard, "", "  ")
		return body, "application/json; charset=utf-8", "ai-coding-usage-" + stamp + ".json", err
	case "xlsx", "excel":
		body, err := exportXLSX(dashboard, exchangeRate)
		return body, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "ai-coding-usage-" + stamp + ".xlsx", err
	case "xls":
		return exportExcelHTML(dashboard, exchangeRate), "application/vnd.ms-excel; charset=utf-8", "ai-coding-usage-" + stamp + ".xls", nil
	default:
		body, err := exportCSV(dashboard, exchangeRate)
		return body, "text/csv; charset=utf-8", "ai-coding-usage-" + stamp + ".csv", err
	}
}

func (s *Service) aggregateSummary(ctx context.Context, startLocal *time.Time, endLocal *time.Time) (Summary, error) {
	query := s.db.WithContext(ctx).Model(&models.AICodingUsageRecord{})
	if startLocal != nil && endLocal != nil {
		query = query.Where("occurred_at >= ? AND occurred_at < ?", startLocal.UTC(), endLocal.UTC())
	}

	var summary Summary
	err := query.Select(`
		COALESCE(SUM(total_cost_usd), 0) AS total_cost_usd,
		COUNT(*) AS total_requests,
		COALESCE(SUM(total_tokens), 0) AS total_tokens,
		COALESCE(SUM(input_tokens), 0) AS input_tokens,
		COALESCE(SUM(output_tokens), 0) AS output_tokens,
		COALESCE(SUM(cache_creation_tokens), 0) AS cache_creation,
		COALESCE(SUM(cache_read_tokens), 0) AS cache_read,
		COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
		COALESCE(SUM(CASE WHEN pricing_matched = false THEN 1 ELSE 0 END), 0) AS pricing_fallbacks,
		COALESCE(SUM(CASE WHEN time_source = 'file_mod_time' THEN 1 ELSE 0 END), 0) AS time_fallbacks
	`).Scan(&summary).Error
	if err != nil {
		return Summary{}, err
	}
	summary.LocalOnly = true
	finalizeSummary(&summary)
	return summary, nil
}

func (s *Service) scanFile(ctx context.Context, candidate logCandidate) (bool, int64, int64, error) {
	info, err := os.Stat(candidate.path)
	if err != nil || info.IsDir() {
		return false, 0, 0, err
	}
	sourceID := "src_" + hashText(strings.ToLower(candidate.path))
	var existing models.AICodingLogSource
	hasExisting := s.db.WithContext(ctx).Where("id = ?", sourceID).First(&existing).Error == nil
	if hasExisting && existing.Size == info.Size() && existing.ModTime.Equal(info.ModTime()) && existing.ErrorMessage == "" && existing.ParserVersion >= parserVersion {
		return false, 0, 0, nil
	}
	if info.Size() > maxLogFileSize {
		s.upsertSource(ctx, sourceID, candidate, info, 0, 0, "file is larger than scan limit")
		return true, 0, 0, nil
	}

	records, err := parseUsageFile(candidate.tool, candidate.path, info.ModTime())
	if err != nil {
		s.upsertSource(ctx, sourceID, candidate, info, 0, 0, err.Error())
		return true, 0, 0, err
	}
	recordIDs := make([]string, 0, len(records))
	parsedByID := make(map[string]parsedRecord, len(records))
	for _, parsed := range records {
		id := buildRecordID(parsed)
		if _, ok := parsedByID[id]; ok {
			continue
		}
		recordIDs = append(recordIDs, id)
		parsedByID[id] = parsed
	}
	existingIDs := s.existingRecordIDs(ctx, recordIDs)
	if err := s.deleteStaleSourceRecords(ctx, candidate.path, recordIDs); err != nil {
		s.logger.Debug().Err(err).Str("path", candidate.path).Msg("ai tool usage: delete stale source records failed")
	}

	created := int64(0)
	for _, id := range recordIDs {
		parsed := parsedByID[id]
		model := nonEmpty(parsed.Model, "unknown")
		cost := s.prices.CalculateCostDetailed(ctx, pricing.CostInput{
			ModelID:             model,
			InputTokens:         parsed.InputTokens,
			OutputTokens:        parsed.OutputTokens,
			CacheCreationTokens: parsed.CacheCreationTokens,
			CacheReadTokens:     parsed.CacheReadTokens,
			ReasoningTokens:     parsed.ReasoningTokens,
			ContextWindow:       parsed.ContextWindow,
			PricingTier:         parsed.PricingTier,
		})
		row := models.AICodingUsageRecord{
			ID:                  id,
			Tool:                parsed.Tool,
			SessionID:           parsed.SessionID,
			RequestID:           parsed.RequestID,
			EventKey:            parsed.EventKey,
			ProjectPath:         parsed.ProjectPath,
			ProjectName:         parsed.ProjectName,
			Model:               model,
			InputTokens:         parsed.InputTokens,
			OutputTokens:        parsed.OutputTokens,
			CacheCreationTokens: parsed.CacheCreationTokens,
			CacheReadTokens:     parsed.CacheReadTokens,
			ReasoningTokens:     parsed.ReasoningTokens,
			ContextWindow:       parsed.ContextWindow,
			PricingTier:         parsed.PricingTier,
			TotalTokens:         parsed.TotalTokens,
			TotalCostUSD:        cost.TotalUSD,
			CostBreakdownJSON:   cost.CostBreakdownJSON,
			PricingRuleJSON:     cost.PricingRuleJSON,
			PricingMatched:      cost.Matched,
			PricingFallback:     cost.FallbackModel,
			SourcePath:          parsed.SourcePath,
			SourceOffset:        parsed.SourceOffset,
			TimeSource:          parsed.TimeSource,
			ParserVersion:       parserVersion,
			RawJSON:             parsed.RawJSON,
			OccurredAt:          parsed.OccurredAt,
			CreatedAt:           s.nowFunc(),
		}
		if row.TotalTokens == 0 {
			row.TotalTokens = row.InputTokens + row.OutputTokens
		}
		tx := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"tool", "session_id", "request_id", "event_key", "project_path", "project_name", "model",
				"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "reasoning_tokens",
				"context_window", "pricing_tier", "total_tokens", "total_cost_usd", "cost_breakdown_json", "pricing_rule_json",
				"pricing_matched", "pricing_fallback", "source_path", "source_offset", "time_source", "parser_version",
				"raw_json", "occurred_at",
			}),
		}).Create(&row)
		if tx.Error != nil {
			s.logger.Debug().Err(tx.Error).Str("record", row.ID).Msg("ai tool usage: insert record failed")
			continue
		}
		if !existingIDs[id] {
			created++
		}
	}
	s.upsertSource(ctx, sourceID, candidate, info, int64(len(records)), created, "")
	return true, int64(len(records)), created, nil
}

func (s *Service) existingRecordIDs(ctx context.Context, ids []string) map[string]bool {
	result := make(map[string]bool, len(ids))
	if len(ids) == 0 {
		return result
	}
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		var rows []models.AICodingUsageRecord
		if err := s.db.WithContext(ctx).Select("id").Where("id IN ?", ids[start:end]).Find(&rows).Error; err != nil {
			continue
		}
		for _, row := range rows {
			result[row.ID] = true
		}
	}
	return result
}

func (s *Service) deleteStaleSourceRecords(ctx context.Context, sourcePath string, currentIDs []string) error {
	if len(currentIDs) == 0 {
		return s.db.WithContext(ctx).Where("source_path = ?", sourcePath).Delete(&models.AICodingUsageRecord{}).Error
	}

	keep := make(map[string]struct{}, len(currentIDs))
	for _, id := range currentIDs {
		keep[id] = struct{}{}
	}

	var rows []models.AICodingUsageRecord
	if err := s.db.WithContext(ctx).Select("id").Where("source_path = ?", sourcePath).Find(&rows).Error; err != nil {
		return err
	}
	staleIDs := make([]string, 0)
	for _, row := range rows {
		if _, ok := keep[row.ID]; !ok {
			staleIDs = append(staleIDs, row.ID)
		}
	}
	for start := 0; start < len(staleIDs); start += 500 {
		end := start + 500
		if end > len(staleIDs) {
			end = len(staleIDs)
		}
		if err := s.db.WithContext(ctx).Where("id IN ?", staleIDs[start:end]).Delete(&models.AICodingUsageRecord{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) upsertSource(ctx context.Context, id string, candidate logCandidate, info os.FileInfo, found int64, created int64, errorMessage string) {
	now := s.nowFunc()
	row := models.AICodingLogSource{
		ID:             id,
		Tool:           candidate.tool,
		Path:           candidate.path,
		Size:           info.Size(),
		ModTime:        info.ModTime(),
		ParserVersion:  parserVersion,
		LastScannedAt:  now,
		RecordsFound:   found,
		RecordsCreated: created,
		ErrorMessage:   errorMessage,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"tool", "path", "size", "mod_time", "last_scanned_at", "records_found", "records_created", "error_message", "updated_at",
			"parser_version",
		}),
	}).Create(&row).Error
}

func discoverLogFiles() []logCandidate {
	roots := candidateRoots()
	seen := map[string]bool{}
	result := make([]logCandidate, 0)
	for _, root := range roots {
		if root.path == "" || seen[root.tool+"|"+root.path] {
			continue
		}
		seen[root.tool+"|"+root.path] = true
		info, err := os.Stat(root.path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if isLogFile(root.path) {
				result = append(result, root)
			}
			continue
		}
		_ = filepath.WalkDir(root.path, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				name := strings.ToLower(entry.Name())
				if name == "node_modules" || name == ".git" || name == "cache" {
					return filepath.SkipDir
				}
				return nil
			}
			if isLogFile(path) {
				result = append(result, logCandidate{tool: root.tool, path: path})
			}
			return nil
		})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result
}

func candidateRoots() []logCandidate {
	home, _ := os.UserHomeDir()
	configDir, _ := os.UserConfigDir()
	roaming := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")
	cwd, _ := os.Getwd()
	joinHome := func(parts ...string) string {
		if home == "" {
			return ""
		}
		return filepath.Join(append([]string{home}, parts...)...)
	}
	roots := []logCandidate{
		{"Claude Code", joinHome(".claude", "projects")},
		{"Claude Code", filepath.Join(configDir, "Claude Code")},
		{"Codex", joinHome(".codex", "sessions")},
		{"Codex", joinHome(".codex", "history.jsonl")},
		{"Kimi Code", joinHome(".kimi")},
		{"Kimi Code", joinHome(".kimicode")},
		{"Kimi Code", filepath.Join(roaming, "Kimi Code")},
		{"Kimi Code", filepath.Join(localAppData, "Kimi Code")},
		{"Qoder", joinHome(".qoder")},
		{"Qoder", filepath.Join(roaming, "Qoder")},
		{"Qoder", filepath.Join(localAppData, "Qoder")},
		{"WorkBuddy", joinHome(".workbuddy")},
		{"WorkBuddy", filepath.Join(cwd, ".workbuddy")},
		{"Hermes", joinHome(".hermes")},
		{"Hermes", filepath.Join(roaming, "Hermes")},
	}
	return roots
}

func isLogFile(path string) bool {
	lowerPath := strings.ToLower(path)
	if strings.Contains(lowerPath, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) ||
		strings.Contains(lowerPath, string(filepath.Separator)+".git"+string(filepath.Separator)) ||
		strings.Contains(lowerPath, string(filepath.Separator)+"plugins"+string(filepath.Separator)) ||
		strings.Contains(lowerPath, string(filepath.Separator)+"extensions"+string(filepath.Separator)) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jsonl" || ext == ".ndjson" {
		return true
	}
	if ext == ".log" {
		return strings.Contains(lowerPath, string(filepath.Separator)+"logs"+string(filepath.Separator)) ||
			strings.Contains(lowerPath, "usage") ||
			strings.Contains(lowerPath, "session")
	}
	base := strings.ToLower(filepath.Base(path))
	if ext == ".json" {
		if strings.HasPrefix(base, "tsconfig") || strings.HasPrefix(base, "package") || strings.Contains(base, "manifest") {
			return false
		}
		return strings.Contains(base, "usage") ||
			strings.Contains(base, "session") ||
			strings.Contains(base, "history") ||
			strings.Contains(base, "trace") ||
			strings.Contains(lowerPath, string(filepath.Separator)+"traces"+string(filepath.Separator))
	}
	return false
}

func parseUsageFile(tool string, path string, fallbackTime time.Time) ([]parsedRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		var payload any
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return extractRecords(tool, path, payload, string(data), 0, map[string]string{}, fallbackTime), nil
	}

	records := []parsedRecord{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	contextState := map[string]string{}
	var offset int64
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineOffset := offset
		offset += int64(len(scanner.Bytes())) + 1
		if line == "" {
			continue
		}
		jsonText := line
		if idx := strings.Index(line, "{"); idx > 0 {
			jsonText = line[idx:]
		}
		var payload any
		if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
			continue
		}
		mergePersistentContextFromValue(contextState, payload)
		records = append(records, extractRecords(tool, path, payload, jsonText, lineOffset, contextState, fallbackTime)...)
	}
	return records, scanner.Err()
}

func extractRecords(tool string, sourcePath string, value any, raw string, offset int64, inherited map[string]string, fallbackTime time.Time) []parsedRecord {
	switch typed := value.(type) {
	case map[string]any:
		ctx := cloneStringMap(inherited)
		mergeContext(ctx, typed)
		record, ok := recordFromMap(tool, sourcePath, typed, raw, offset, ctx, fallbackTime)
		result := []parsedRecord{}
		if ok {
			result = append(result, record)
			return result
		}
		for key, child := range typed {
			if shouldSkipChildTokenContainer(key) {
				continue
			}
			result = append(result, extractRecords(tool, sourcePath, child, raw, offset, ctx, fallbackTime)...)
		}
		return dedupeParsed(result)
	case []any:
		result := []parsedRecord{}
		for _, child := range typed {
			result = append(result, extractRecords(tool, sourcePath, child, raw, offset, inherited, fallbackTime)...)
		}
		return dedupeParsed(result)
	default:
		return nil
	}
}

func recordFromMap(tool string, sourcePath string, item map[string]any, raw string, offset int64, ctx map[string]string, fallbackTime time.Time) (parsedRecord, bool) {
	tokens := tokenContainer(item)
	if tokens == nil {
		return parsedRecord{}, false
	}
	input, _ := firstIntShallowWithKey(tokens, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens", "prompt_token_count")
	output, _ := firstIntShallowWithKey(tokens, "output_tokens", "outputTokens", "completion_tokens", "completionTokens", "completion_token_count")
	cacheCreate, cacheCreateKey := firstIntShallowWithKey(tokens, "cache_creation_input_tokens", "cacheCreationInputTokens", "cache_creation_tokens", "cacheWriteTokens")
	cacheRead, cacheReadKey := firstIntShallowWithKey(tokens, "cache_read_input_tokens", "cacheReadInputTokens", "cache_read_tokens", "cacheReadTokens", "cached_tokens", "cached_input_tokens", "cachedInputTokens")
	if cacheRead == 0 {
		cacheRead = detailsCachedTokens(tokens)
		if cacheRead > 0 {
			cacheReadKey = "cached_tokens_details"
		}
	}
	reasoningTokens := firstIntShallow(tokens, "reasoning_output_tokens", "reasoningOutputTokens", "reasoning_tokens", "reasoningTokens")
	if reasoningTokens == 0 {
		reasoningTokens = detailsReasoningTokens(tokens)
	}
	contextWindow := firstInt(item, "model_context_window", "modelContextWindow", "context_window", "contextWindow", "max_context_tokens")
	pricingTier := firstString(item, "service_tier", "serviceTier", "pricing_tier", "pricingTier")
	total, _ := firstIntShallowWithKey(tokens, "total_tokens", "totalTokens")
	if input == 0 && output == 0 && cacheCreate == 0 && cacheRead == 0 && total == 0 {
		return parsedRecord{}, false
	}
	if input == 0 && output == 0 && total == 0 {
		return parsedRecord{}, false
	}
	input = normalizeInputTokens(input, cacheCreate, cacheRead, cacheCreateKey, cacheReadKey)
	if total == 0 {
		total = input + output
	}
	if total < input+output {
		total = input + output
	}
	if input == 0 && total > output {
		input = total - output
	}
	model := firstString(item, "model", "model_id", "modelId", "model_actual", "actualModel", "requestedModel")
	sessionID := firstString(item, "session_id", "sessionId", "conversation_id", "conversationId", "thread_id", "threadId")
	requestID := firstString(item, "request_id", "requestId", "message_id", "messageId", "id", "uuid")
	projectPath := firstString(item, "project_path", "projectPath", "cwd", "workspace", "workspacePath", "root", "repo", "directory")
	if model == "" {
		model = ctx["model"]
	}
	if sessionID == "" {
		sessionID = ctx["session_id"]
	}
	if requestID == "" {
		requestID = ctx["request_id"]
	}
	if projectPath == "" {
		projectPath = ctx["project_path"]
	}
	occurredAt, timeSource := firstTimeWithSource(item, fallbackTime, "timestamp", "created_at", "createdAt", "time", "date", "request_time", "requestTime")
	eventKey := usageEventKey(tool, sourcePath, item, tokens, sessionID, requestID)
	return parsedRecord{
		Tool:                tool,
		SessionID:           sessionID,
		RequestID:           requestID,
		EventKey:            eventKey,
		ProjectPath:         projectPath,
		ProjectName:         projectName(projectPath, sourcePath),
		Model:               model,
		InputTokens:         input,
		OutputTokens:        output,
		CacheCreationTokens: cacheCreate,
		CacheReadTokens:     cacheRead,
		ReasoningTokens:     reasoningTokens,
		ContextWindow:       contextWindow,
		PricingTier:         pricingTier,
		TotalTokens:         total,
		SourcePath:          sourcePath,
		SourceOffset:        offset,
		TimeSource:          timeSource,
		RawJSON:             truncate(raw, 8192),
		OccurredAt:          occurredAt,
	}, true
}

func tokenContainer(item map[string]any) map[string]any {
	candidates := []map[string]any{item}
	for _, key := range []string{"usage", "rawUsage", "last_token_usage", "lastTokenUsage"} {
		if child, ok := mapChild(item, key); ok {
			candidates = append(candidates, child)
		}
	}
	if message, ok := mapChild(item, "message"); ok {
		if usage, ok := mapChild(message, "usage"); ok {
			candidates = append(candidates, usage)
		}
	}
	if providerData, ok := mapChild(item, "providerData"); ok {
		if usage, ok := mapChild(providerData, "rawUsage"); ok {
			candidates = append(candidates, usage)
		}
		if usage, ok := mapChild(providerData, "usage"); ok {
			candidates = append(candidates, usage)
		}
	}
	if payload, ok := mapChild(item, "payload"); ok {
		if info, ok := mapChild(payload, "info"); ok {
			if usage, ok := mapChild(info, "last_token_usage"); ok {
				candidates = append(candidates, usage)
			}
		}
	}
	if info, ok := mapChild(item, "info"); ok {
		if usage, ok := mapChild(info, "last_token_usage"); ok {
			candidates = append(candidates, usage)
		}
	}
	for _, candidate := range candidates {
		if hasTokenKeys(candidate) {
			return candidate
		}
	}
	return nil
}

func mapChild(item map[string]any, key string) (map[string]any, bool) {
	for itemKey, value := range item {
		if !strings.EqualFold(itemKey, key) {
			continue
		}
		child, ok := value.(map[string]any)
		return child, ok
	}
	return nil, false
}

func hasTokenKeys(item map[string]any) bool {
	return firstIntShallow(item, "input_tokens", "inputTokens", "prompt_tokens", "promptTokens", "output_tokens", "outputTokens", "completion_tokens", "completionTokens", "total_tokens", "totalTokens") > 0
}

func shouldSkipChildTokenContainer(key string) bool {
	normalized := strings.ToLower(key)
	return normalized == "total_token_usage" ||
		normalized == "totaltokenusage" ||
		normalized == "last_token_usage" ||
		normalized == "lasttokenusage" ||
		normalized == "usage" ||
		normalized == "rawusage" ||
		normalized == "prompt_tokens_details" ||
		normalized == "completion_tokens_details" ||
		normalized == "inputtokensdetails" ||
		normalized == "outputtokensdetails"
}

func usageEventKey(tool string, sourcePath string, item map[string]any, tokens map[string]any, sessionID string, requestID string) string {
	if strings.EqualFold(tool, "Codex") {
		if key := codexTokenCountEventKey(item, tokens); key != "" {
			return strings.Join([]string{"codex", sessionID, key}, "|")
		}
	}
	if strings.EqualFold(tool, "WorkBuddy") {
		if key := workBuddyEventKey(item, tokens); key != "" {
			return key
		}
	}
	if strings.EqualFold(tool, "Claude Code") && requestID != "" {
		return strings.Join([]string{"claude", sessionID, requestID}, "|")
	}
	return ""
}

func codexTokenCountEventKey(item map[string]any, tokens map[string]any) string {
	payload, ok := mapChild(item, "payload")
	if !ok || !strings.EqualFold(firstStringShallow(payload, "type"), "token_count") {
		return ""
	}
	info, ok := mapChild(payload, "info")
	if !ok {
		return ""
	}
	totalUsage, ok := mapChild(info, "total_token_usage")
	if !ok {
		return ""
	}
	parts := []string{
		"codex-token-count",
		tokenFingerprintPart("total", totalUsage),
		tokenFingerprintPart("last", tokens),
		strconv.FormatInt(firstIntShallow(info, "model_context_window", "modelContextWindow"), 10),
	}
	return strings.Join(parts, "|")
}

func workBuddyEventKey(item map[string]any, tokens map[string]any) string {
	providerData := item
	if child, ok := mapChild(item, "providerData"); ok {
		providerData = child
	}
	messageID := firstStringShallow(providerData, "messageId")
	if messageID != "" {
		return "workbuddy|message|" + messageID
	}
	traceID := firstStringShallow(providerData, "traceId")
	conversationRequestID := firstStringShallow(providerData, "conversationRequestId")
	if traceID != "" || conversationRequestID != "" {
		return strings.Join([]string{"workbuddy", traceID, conversationRequestID, tokenFingerprintPart("usage", tokens)}, "|")
	}
	return ""
}

func tokenFingerprintPart(prefix string, item map[string]any) string {
	values := []string{prefix}
	for _, key := range []string{
		"input_tokens", "inputTokens", "prompt_tokens", "promptTokens",
		"output_tokens", "outputTokens", "completion_tokens", "completionTokens",
		"cached_input_tokens", "cachedInputTokens", "cache_read_input_tokens", "cacheReadInputTokens", "cache_read_tokens", "cacheReadTokens",
		"cache_creation_input_tokens", "cacheCreationInputTokens", "cache_creation_tokens", "cacheWriteTokens",
		"reasoning_output_tokens", "reasoningOutputTokens", "reasoning_tokens", "reasoningTokens",
		"total_tokens", "totalTokens",
	} {
		value, _ := firstIntShallowWithKey(item, key)
		values = append(values, strconv.FormatInt(value, 10))
	}
	return strings.Join(values, ":")
}

func firstStringShallow(item map[string]any, key string) string {
	for itemKey, value := range item {
		if !strings.EqualFold(itemKey, key) {
			continue
		}
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func mergeContext(ctx map[string]string, item map[string]any) {
	if value := firstString(item, "model", "model_id", "modelId", "model_actual", "actualModel", "requestedModel"); value != "" {
		ctx["model"] = value
	}
	if value := firstString(item, "session_id", "sessionId", "conversation_id", "conversationId", "thread_id", "threadId"); value != "" {
		ctx["session_id"] = value
	}
	if value := firstString(item, "request_id", "requestId", "message_id", "messageId", "id", "uuid"); value != "" {
		ctx["request_id"] = value
	}
	if value := firstString(item, "project_path", "projectPath", "cwd", "workspace", "workspacePath", "root", "repo", "directory"); value != "" {
		ctx["project_path"] = value
	}
}

func mergePersistentContextFromValue(ctx map[string]string, value any) {
	switch typed := value.(type) {
	case map[string]any:
		mergePersistentContext(ctx, typed)
		for key, child := range typed {
			if shouldSkipChildTokenContainer(key) {
				continue
			}
			mergePersistentContextFromValue(ctx, child)
		}
	case []any:
		for _, child := range typed {
			mergePersistentContextFromValue(ctx, child)
		}
	}
}

func mergePersistentContext(ctx map[string]string, item map[string]any) {
	if value := firstString(item, "model", "model_id", "modelId", "model_actual", "actualModel", "requestedModel"); value != "" {
		ctx["model"] = value
	}
	if value := firstString(item, "session_id", "sessionId", "conversation_id", "conversationId", "thread_id", "threadId"); value != "" {
		ctx["session_id"] = value
	}
	if value := firstString(item, "project_path", "projectPath", "cwd", "workspace", "workspacePath", "root", "repo", "directory"); value != "" {
		ctx["project_path"] = value
	}
}

func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := findValue(item, key); ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case fmt.Stringer:
				return typed.String()
			}
		}
	}
	return ""
}

func firstInt(item map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if value, ok := findValue(item, key); ok {
			if parsed, ok := toInt64(value); ok {
				return parsed
			}
		}
	}
	return 0
}

func firstIntShallow(item map[string]any, keys ...string) int64 {
	value, _ := firstIntShallowWithKey(item, keys...)
	return value
}

func firstIntShallowWithKey(item map[string]any, keys ...string) (int64, string) {
	for _, key := range keys {
		for itemKey, value := range item {
			if !strings.EqualFold(itemKey, key) {
				continue
			}
			if parsed, ok := toInt64(value); ok {
				return parsed, itemKey
			}
		}
	}
	return 0, ""
}

func normalizeInputTokens(input, cacheCreate, cacheRead int64, cacheCreateKey, cacheReadKey string) int64 {
	if input <= 0 {
		return input
	}
	if cacheTokensAreSeparate(cacheCreateKey) {
		input += cacheCreate
	}
	if cacheTokensAreSeparate(cacheReadKey) {
		input += cacheRead
	}
	return input
}

func cacheTokensAreSeparate(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
	return strings.HasPrefix(normalized, "cachecreation") ||
		strings.HasPrefix(normalized, "cachewrite") ||
		strings.HasPrefix(normalized, "cacheread")
}

func detailsCachedTokens(item map[string]any) int64 {
	var total int64
	for _, key := range []string{"inputTokensDetails", "prompt_tokens_details"} {
		for itemKey, value := range item {
			if !strings.EqualFold(itemKey, key) {
				continue
			}
			total += cachedTokensFromDetails(value)
		}
	}
	return total
}

func detailsReasoningTokens(item map[string]any) int64 {
	var total int64
	for _, key := range []string{"outputTokensDetails", "completion_tokens_details"} {
		for itemKey, value := range item {
			if !strings.EqualFold(itemKey, key) {
				continue
			}
			total += reasoningTokensFromDetails(value)
		}
	}
	return total
}

func cachedTokensFromDetails(value any) int64 {
	switch typed := value.(type) {
	case map[string]any:
		return firstIntShallow(typed, "cached_tokens", "cached_input_tokens", "cache_read_input_tokens", "cacheReadTokens")
	case []any:
		var total int64
		for _, item := range typed {
			total += cachedTokensFromDetails(item)
		}
		return total
	default:
		return 0
	}
}

func reasoningTokensFromDetails(value any) int64 {
	switch typed := value.(type) {
	case map[string]any:
		return firstIntShallow(typed, "reasoning_tokens", "reasoning_output_tokens", "reasoningTokens", "reasoningOutputTokens")
	case []any:
		var total int64
		for _, item := range typed {
			total += reasoningTokensFromDetails(item)
		}
		return total
	default:
		return 0
	}
}

func firstTime(item map[string]any, fallback time.Time, keys ...string) time.Time {
	value, _ := firstTimeWithSource(item, fallback, keys...)
	return value
}

func firstTimeWithSource(item map[string]any, fallback time.Time, keys ...string) (time.Time, string) {
	for _, key := range keys {
		value, ok := findValue(item, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
				if parsed, err := time.Parse(layout, typed); err == nil {
					return parsed.UTC(), "timestamp"
				}
			}
			location := beijingLocation()
			for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
				if parsed, err := time.ParseInLocation(layout, typed, location); err == nil {
					return parsed.UTC(), "timestamp"
				}
			}
		case float64:
			return unixTime(int64(typed), fallback).UTC(), "timestamp"
		case int64:
			return unixTime(typed, fallback).UTC(), "timestamp"
		case int:
			return unixTime(int64(typed), fallback).UTC(), "timestamp"
		}
	}
	return fallback.UTC(), "file_mod_time"
}

func beijingLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}

func findValue(value any, key string) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for k, v := range typed {
			if strings.EqualFold(k, key) {
				return v, true
			}
		}
		for _, v := range typed {
			if found, ok := findValue(v, key); ok {
				return found, true
			}
		}
	case []any:
		for _, v := range typed {
			if found, ok := findValue(v, key); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func toInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func unixTime(value int64, fallback time.Time) time.Time {
	switch {
	case value > 1_000_000_000_000:
		return time.UnixMilli(value)
	case value > 1_000_000_000:
		return time.Unix(value, 0)
	default:
		return fallback
	}
}

func buildRecordID(record parsedRecord) string {
	if record.EventKey != "" {
		return "aiu_" + hashText(strings.Join([]string{
			record.Tool, record.EventKey,
		}, "|"))
	}
	if record.RequestID != "" {
		return "aiu_" + hashText(strings.Join([]string{
			record.Tool, record.SessionID, record.RequestID,
		}, "|"))
	}
	if isOffsetAddressableLog(record.SourcePath) {
		return "aiu_" + hashText(strings.Join([]string{
			record.Tool, record.SourcePath, strconv.FormatInt(record.SourceOffset, 10),
		}, "|"))
	}
	return "aiu_" + hashText(strings.Join([]string{
		record.Tool, record.SourcePath, strconv.FormatInt(record.SourceOffset, 10), record.Model,
		strconv.FormatInt(record.InputTokens, 10), strconv.FormatInt(record.OutputTokens, 10),
		record.OccurredAt.Format(time.RFC3339Nano),
	}, "|"))
}

func isOffsetAddressableLog(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jsonl" || ext == ".ndjson" || ext == ".log"
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:32]
}

func projectName(projectPath string, sourcePath string) string {
	if projectPath != "" {
		cleaned := filepath.Clean(projectPath)
		base := filepath.Base(cleaned)
		if base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	dir := filepath.Base(filepath.Dir(sourcePath))
	if dir == "projects" || dir == ".codex" || dir == ".claude" || dir == "." {
		return "local"
	}
	return strings.ReplaceAll(dir, "-Users-", "")
}

func addSummary(summary *Summary, record models.AICodingUsageRecord) {
	summary.TotalCostUSD += record.TotalCostUSD
	summary.TotalRequests++
	summary.TotalTokens += record.TotalTokens
	summary.InputTokens += record.InputTokens
	summary.OutputTokens += record.OutputTokens
	summary.CacheCreation += record.CacheCreationTokens
	summary.CacheRead += record.CacheReadTokens
	summary.ReasoningTokens += record.ReasoningTokens
	if !record.PricingMatched {
		summary.PricingFallbacks++
	}
	if record.TimeSource == "file_mod_time" {
		summary.TimeFallbacks++
	}
}

func finalizeSummary(summary *Summary) {
	if summary.InputTokens > 0 {
		summary.CacheHitRate = float64(summary.CacheRead) / float64(summary.InputTokens)
		if summary.CacheHitRate > 1 {
			summary.CacheHitRate = 1
		}
	}
}

func addBreakdown(items map[string]*Breakdown, name string, tool string, record models.AICodingUsageRecord) {
	item, ok := items[name]
	if !ok {
		item = &Breakdown{Name: name, Tool: tool}
		items[name] = item
	} else if item.Tool != "" && tool != "" && item.Tool != tool {
		item.Tool = ""
	}
	item.CostUSD += record.TotalCostUSD
	item.Requests++
	item.Tokens += record.TotalTokens
	item.InputTokens += record.InputTokens
	item.OutputTokens += record.OutputTokens
	item.CacheReadTokens += record.CacheReadTokens
	item.ReasoningTokens += record.ReasoningTokens
	if !record.PricingMatched {
		item.PricingFallbacks++
	}
	if item.LastSeen == "" || record.OccurredAt.Format(time.RFC3339) > item.LastSeen {
		item.LastSeen = record.OccurredAt.Format(time.RFC3339)
	}
}

func sortedBreakdowns(items map[string]*Breakdown, limit int) []Breakdown {
	result := make([]Breakdown, 0, len(items))
	for _, item := range items {
		if item.InputTokens > 0 {
			item.CacheHitRate = float64(item.CacheReadTokens) / float64(item.InputTokens)
			if item.CacheHitRate > 1 {
				item.CacheHitRate = 1
			}
		}
		result = append(result, *item)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CostUSD > result[j].CostUSD })
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func completeTrend(points map[string]*TrendPoint, days int, now time.Time) []TrendPoint {
	result := make([]TrendPoint, 0, days)
	start := startOfDay(now.AddDate(0, 0, -days+1))
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		if point, ok := points[key]; ok {
			result = append(result, *point)
		} else {
			result = append(result, TrendPoint{Day: day.Format("01-02")})
		}
	}
	return result
}

func completeHeatmap(points map[string]*HeatmapPoint, days int, now time.Time) []HeatmapPoint {
	result := make([]HeatmapPoint, 0, days*24)
	start := startOfDay(now.AddDate(0, 0, -days+1))
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		dayKey := day.Format("2006-01-02")
		dayLabel := day.Format("01-02")
		for hour := 0; hour < 24; hour++ {
			key := fmt.Sprintf("%s-%02d", dayKey, hour)
			if point, ok := points[key]; ok {
				result = append(result, *point)
				continue
			}
			result = append(result, HeatmapPoint{Day: dayLabel, Hour: hour})
		}
	}
	return result
}

func exportCSV(dashboard Dashboard, exchangeRate float64) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	_ = writer.Write([]string{"section", "name", "tool", "requests", "tokens", "reasoning_tokens", "cache_hit_rate", "cost_usd", "cost_cny"})
	writeBreakdownCSV(writer, "model", dashboard.ModelRank, exchangeRate)
	writeBreakdownCSV(writer, "project", dashboard.ProjectSpend, exchangeRate)
	writeBreakdownCSV(writer, "tool", dashboard.ToolBreakdown, exchangeRate)
	writer.Flush()
	return buf.Bytes(), writer.Error()
}

func writeBreakdownCSV(writer *csv.Writer, section string, items []Breakdown, exchangeRate float64) {
	for _, item := range items {
		_ = writer.Write([]string{
			section,
			item.Name,
			item.Tool,
			strconv.FormatInt(item.Requests, 10),
			strconv.FormatInt(item.Tokens, 10),
			strconv.FormatInt(item.ReasoningTokens, 10),
			fmt.Sprintf("%.4f", item.CacheHitRate),
			fmt.Sprintf("%.8f", item.CostUSD),
			fmt.Sprintf("%.8f", item.CostUSD*exchangeRate),
		})
	}
}

func exportExcelHTML(dashboard Dashboard, exchangeRate float64) []byte {
	var b strings.Builder
	b.WriteString("<html><head><meta charset=\"utf-8\"></head><body>")
	b.WriteString("<h1>AI Coding Usage Report</h1>")
	b.WriteString("<table border=\"1\"><tr><th>Total Requests</th><th>Total Tokens</th><th>Cost USD</th><th>Cost CNY</th><th>Cache Hit Rate</th></tr>")
	b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%.8f</td><td>%.8f</td><td>%.2f%%</td></tr>",
		dashboard.Summary.TotalRequests, dashboard.Summary.TotalTokens, dashboard.Summary.TotalCostUSD, dashboard.Summary.TotalCostUSD*exchangeRate, dashboard.Summary.CacheHitRate*100))
	b.WriteString("</table>")
	for _, section := range []struct {
		title string
		items []Breakdown
	}{{"Models", dashboard.ModelRank}, {"Projects", dashboard.ProjectSpend}, {"Tools", dashboard.ToolBreakdown}} {
		b.WriteString("<h2>" + html.EscapeString(section.title) + "</h2>")
		b.WriteString("<table border=\"1\"><tr><th>Name</th><th>Tool</th><th>Requests</th><th>Tokens</th><th>Cost USD</th><th>Cost CNY</th></tr>")
		for _, item := range section.items {
			b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%.8f</td><td>%.8f</td></tr>",
				html.EscapeString(item.Name), html.EscapeString(item.Tool), item.Requests, item.Tokens, item.CostUSD, item.CostUSD*exchangeRate))
		}
		b.WriteString("</table>")
	}
	b.WriteString("</body></html>")
	return []byte(b.String())
}

type xlsxSheet struct {
	name string
	rows [][]any
}

func exportXLSX(dashboard Dashboard, exchangeRate float64) ([]byte, error) {
	sheets := []xlsxSheet{
		{
			name: "Summary",
			rows: [][]any{
				{"metric", "value"},
				{"total_requests", dashboard.Summary.TotalRequests},
				{"total_tokens", dashboard.Summary.TotalTokens},
				{"input_tokens", dashboard.Summary.InputTokens},
				{"output_tokens", dashboard.Summary.OutputTokens},
				{"cache_creation_tokens", dashboard.Summary.CacheCreation},
				{"cache_read_tokens", dashboard.Summary.CacheRead},
				{"reasoning_tokens", dashboard.Summary.ReasoningTokens},
				{"cache_hit_rate", dashboard.Summary.CacheHitRate},
				{"total_cost_usd", dashboard.Summary.TotalCostUSD},
				{"total_cost_cny", dashboard.Summary.TotalCostUSD * exchangeRate},
				{"pricing_fallbacks", dashboard.Summary.PricingFallbacks},
				{"time_fallbacks", dashboard.Summary.TimeFallbacks},
				{"scanned_sources", dashboard.Summary.ScannedSources},
				{"last_scan", dashboard.Summary.LastScan},
				{"local_only", dashboard.Summary.LocalOnly},
			},
		},
		{name: "Trend", rows: trendRows(dashboard.Trend, exchangeRate)},
		{name: "Heatmap", rows: heatmapRows(dashboard.Heatmap, exchangeRate)},
		{name: "Models", rows: breakdownRows(dashboard.ModelRank, exchangeRate)},
		{name: "Projects", rows: breakdownRows(dashboard.ProjectSpend, exchangeRate)},
		{name: "Tools", rows: breakdownRows(dashboard.ToolBreakdown, exchangeRate)},
		{name: "Sources", rows: sourceRows(dashboard.Sources)},
		{name: "Recent", rows: recentRows(dashboard.Recent, exchangeRate)},
	}

	buf := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buf)
	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypes(sheets),
		"_rels/.rels":                xlsxRootRels(),
		"xl/workbook.xml":            xlsxWorkbook(sheets),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRels(sheets),
	}
	for name, body := range files {
		if err := writeZipFile(zipWriter, name, body); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	for index, sheet := range sheets {
		if err := writeZipFile(zipWriter, fmt.Sprintf("xl/worksheets/sheet%d.xml", index+1), xlsxWorksheet(sheet.rows)); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func trendRows(points []TrendPoint, exchangeRate float64) [][]any {
	rows := [][]any{{"day", "requests", "tokens", "cache_read_tokens", "cost_usd", "cost_cny"}}
	for _, point := range points {
		rows = append(rows, []any{point.Day, point.Requests, point.Tokens, point.CacheRead, point.CostUSD, point.CostUSD * exchangeRate})
	}
	return rows
}

func heatmapRows(points []HeatmapPoint, exchangeRate float64) [][]any {
	rows := [][]any{{"day", "hour", "requests", "tokens", "cost_usd", "cost_cny"}}
	for _, point := range points {
		rows = append(rows, []any{point.Day, point.Hour, point.Requests, point.Tokens, point.CostUSD, point.CostUSD * exchangeRate})
	}
	return rows
}

func breakdownRows(items []Breakdown, exchangeRate float64) [][]any {
	rows := [][]any{{"name", "tool", "requests", "tokens", "input_tokens", "output_tokens", "cache_read_tokens", "reasoning_tokens", "cache_hit_rate", "cost_usd", "cost_cny", "pricing_fallbacks", "last_seen"}}
	for _, item := range items {
		rows = append(rows, []any{
			item.Name,
			item.Tool,
			item.Requests,
			item.Tokens,
			item.InputTokens,
			item.OutputTokens,
			item.CacheReadTokens,
			item.ReasoningTokens,
			item.CacheHitRate,
			item.CostUSD,
			item.CostUSD * exchangeRate,
			item.PricingFallbacks,
			item.LastSeen,
		})
	}
	return rows
}

func sourceRows(items []SourceInfo) [][]any {
	rows := [][]any{{"tool", "path", "records_found", "records_created", "last_scanned_at", "error_message"}}
	for _, item := range items {
		rows = append(rows, []any{item.Tool, item.Path, item.RecordsFound, item.RecordsCreated, item.LastScannedAt, item.ErrorMessage})
	}
	return rows
}

func recentRows(items []models.AICodingUsageRecord, exchangeRate float64) [][]any {
	rows := [][]any{{"occurred_at", "tool", "model", "project", "tokens", "input_tokens", "output_tokens", "cache_read_tokens", "reasoning_tokens", "context_window", "pricing_tier", "time_source", "cost_usd", "cost_cny"}}
	for _, item := range items {
		rows = append(rows, []any{
			item.OccurredAt.Format(time.RFC3339),
			item.Tool,
			item.Model,
			item.ProjectName,
			item.TotalTokens,
			item.InputTokens,
			item.OutputTokens,
			item.CacheReadTokens,
			item.ReasoningTokens,
			item.ContextWindow,
			item.PricingTier,
			item.TimeSource,
			item.TotalCostUSD,
			item.TotalCostUSD * exchangeRate,
		})
	}
	return rows
}

func xlsxContentTypes(sheets []xlsxSheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := range sheets {
		b.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1))
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func xlsxRootRels() string {
	return `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
}

func xlsxWorkbook(sheets []xlsxSheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>`)
	for i, sheet := range sheets {
		b.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlEscape(sheet.name), i+1, i+1))
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}

func xlsxWorkbookRels(sheets []xlsxSheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := range sheets {
		b.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1))
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

func xlsxWorksheet(rows [][]any) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for rowIndex, row := range rows {
		b.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex+1))
		for colIndex, value := range row {
			b.WriteString(xlsxCell(colIndex+1, rowIndex+1, value))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func xlsxCell(col, row int, value any) string {
	ref := columnName(col) + strconv.Itoa(row)
	switch typed := value.(type) {
	case int:
		return fmt.Sprintf(`<c r="%s"><v>%d</v></c>`, ref, typed)
	case int64:
		return fmt.Sprintf(`<c r="%s"><v>%d</v></c>`, ref, typed)
	case float64:
		return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(typed, 'f', -1, 64))
	case bool:
		if typed {
			return fmt.Sprintf(`<c r="%s" t="b"><v>1</v></c>`, ref)
		}
		return fmt.Sprintf(`<c r="%s" t="b"><v>0</v></c>`, ref)
	default:
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, xmlEscape(fmt.Sprint(value)))
	}
}

func columnName(index int) string {
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func writeZipFile(zipWriter *zip.Writer, name string, body string) error {
	file, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(body))
	return err
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}

func dedupeParsed(records []parsedRecord) []parsedRecord {
	seen := map[string]bool{}
	result := make([]parsedRecord, 0, len(records))
	for _, record := range records {
		key := buildRecordID(record)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, record)
	}
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func startOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func nonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
