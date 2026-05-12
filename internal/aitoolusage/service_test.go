package aitoolusage

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
)

func TestParseUsageFileExtractsNestedClaudeUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := `{"session_id":"s1","cwd":"D:\\repo\\tokenbridge","message":{"id":"m1","model":"claude-3-5-sonnet","usage":{"input_tokens":1000,"output_tokens":200,"cache_creation_input_tokens":100,"cache_read_input_tokens":300}},"timestamp":"2026-05-12T10:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := parseUsageFile("Claude Code", path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	record := records[0]
	if record.Model != "claude-3-5-sonnet" || record.InputTokens != 1400 || record.OutputTokens != 200 {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.TotalTokens != 1600 {
		t.Fatalf("expected normalized total tokens to include cache, got %+v", record)
	}
	if record.CacheCreationTokens != 100 || record.CacheReadTokens != 300 {
		t.Fatalf("unexpected cache tokens: %+v", record)
	}
	if record.ProjectName != "tokenbridge" {
		t.Fatalf("unexpected project name: %s", record.ProjectName)
	}
}

func TestParseUsageFileUsesCodexLastTokenUsage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex.jsonl")
	body := `{"timestamp":"2026-05-12T04:40:08.746Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":4390495,"cached_input_tokens":4133504,"output_tokens":29672,"total_tokens":4420167},"last_token_usage":{"input_tokens":145242,"cached_input_tokens":144768,"output_tokens":438,"total_tokens":145680}}}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := parseUsageFile("Codex", path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(records), records)
	}
	record := records[0]
	if record.InputTokens != 145242 || record.OutputTokens != 438 || record.TotalTokens != 145680 {
		t.Fatalf("expected last_token_usage, got %+v", record)
	}
	if record.CacheReadTokens != 144768 {
		t.Fatalf("expected cached_input_tokens to be cache read tokens, got %+v", record)
	}
}

func TestParseUsageFileCarriesCodexContextAcrossLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex.jsonl")
	body := `{"timestamp":"2026-05-12T04:00:00Z","type":"turn_context","payload":{"cwd":"D:\\idea\\tokenbridge","model":"gpt-5.4"}}` + "\n" +
		`{"timestamp":"2026-05-12T04:01:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1000,"cached_input_tokens":300,"output_tokens":200,"total_tokens":1200}}}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := parseUsageFile("Codex", path, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d: %+v", len(records), records)
	}
	record := records[0]
	if record.Model != "gpt-5.4" {
		t.Fatalf("expected model from prior context line, got %+v", record)
	}
	if record.ProjectName != "tokenbridge" {
		t.Fatalf("expected project from prior context line, got %+v", record)
	}
	if record.InputTokens != 1000 || record.CacheReadTokens != 300 {
		t.Fatalf("unexpected token normalization for cached Codex usage: %+v", record)
	}
}

func TestBuildRecordIDForLineLogsIgnoresEnrichedModel(t *testing.T) {
	base := parsedRecord{
		Tool:         "Codex",
		SourcePath:   filepath.Join(t.TempDir(), "session.jsonl"),
		SourceOffset: 128,
		InputTokens:  1000,
		OutputTokens: 200,
		OccurredAt:   time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
		Model:        "unknown",
	}
	enriched := base
	enriched.Model = "gpt-5.4"
	if buildRecordID(base) != buildRecordID(enriched) {
		t.Fatal("line-log record id should remain stable when parser later enriches model context")
	}
}

func TestScanFileIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	priceService := pricing.NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	svc := NewService(db, priceService, zerolog.Nop())
	svc.nowFunc = func() time.Time { return now }

	db.Create(&models.ModelPricing{
		ModelID:                   "gpt-5",
		Mode:                      "chat",
		InputCostPerToken:         1.0 / 1_000_000,
		OutputCostPerToken:        2.0 / 1_000_000,
		CacheCreationCostPerToken: 0.5 / 1_000_000,
		CacheReadCostPerToken:     0.1 / 1_000_000,
		FetchedAt:                 now,
	})

	path := filepath.Join(t.TempDir(), "codex.jsonl")
	body := `{"request_id":"r1","session_id":"s1","model":"gpt-5","input_tokens":1000,"output_tokens":100,"cache_read_input_tokens":400,"cwd":"D:\\repo\\x","timestamp":"2026-05-12T10:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, found, created, err := svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path})
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || created != 1 {
		t.Fatalf("first scan found=%d created=%d", found, created)
	}
	_, found, created, err = svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path})
	if err != nil {
		t.Fatal(err)
	}
	if found != 0 || created != 0 {
		t.Fatalf("unchanged second scan should skip, found=%d created=%d", found, created)
	}
	var count int64
	db.Model(&models.AICodingUsageRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one usage row, got %d", count)
	}
	var row models.AICodingUsageRecord
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.InputTokens != 1400 {
		t.Fatalf("expected separate cache input tokens to be normalized, got %d", row.InputTokens)
	}
	if math.Abs(row.TotalCostUSD-0.00124) > 0.00000001 {
		t.Fatalf("unexpected cache-aware cost: %.8f", row.TotalCostUSD)
	}
}

func TestScanFileAppendOnlyCreatesNewRows(t *testing.T) {
	db := openTestDB(t)
	priceService := pricing.NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	svc := NewService(db, priceService, zerolog.Nop())
	svc.nowFunc = func() time.Time { return now }

	db.Create(&models.ModelPricing{
		ModelID:            "gpt-5",
		Mode:               "chat",
		InputCostPerToken:  1.0 / 1_000_000,
		OutputCostPerToken: 2.0 / 1_000_000,
		FetchedAt:          now,
	})

	path := filepath.Join(t.TempDir(), "codex.jsonl")
	first := `{"request_id":"r1","session_id":"s1","model":"gpt-5","input_tokens":1000,"output_tokens":100,"cwd":"D:\\repo\\x","timestamp":"2026-05-12T10:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	_, found, created, err := svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path})
	if err != nil {
		t.Fatal(err)
	}
	if found != 1 || created != 1 {
		t.Fatalf("first scan found=%d created=%d", found, created)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(`{"request_id":"r2","session_id":"s1","model":"gpt-5","input_tokens":2000,"output_tokens":200,"cwd":"D:\\repo\\x","timestamp":"2026-05-12T11:00:00Z"}` + "\n")
	if closeErr := file.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	_, found, created, err = svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path})
	if err != nil {
		t.Fatal(err)
	}
	if found != 2 || created != 1 {
		t.Fatalf("append scan should parse both rows but create only one new row, found=%d created=%d", found, created)
	}
	var count int64
	db.Model(&models.AICodingUsageRecord{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected two usage rows after append, got %d", count)
	}
}

func TestScanFileRewriteReplacesStaleSourceRows(t *testing.T) {
	db := openTestDB(t)
	priceService := pricing.NewService(db, zerolog.Nop())
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	svc := NewService(db, priceService, zerolog.Nop())
	svc.nowFunc = func() time.Time { return now }

	db.Create(&models.ModelPricing{
		ModelID:            "gpt-5",
		Mode:               "chat",
		InputCostPerToken:  1.0 / 1_000_000,
		OutputCostPerToken: 2.0 / 1_000_000,
		FetchedAt:          now,
	})

	path := filepath.Join(t.TempDir(), "snapshot.jsonl")
	first := `{"request_id":"r1","session_id":"s1","model":"gpt-5","input_tokens":1000,"output_tokens":100,"timestamp":"2026-05-12T10:00:00Z"}` + "\n" +
		`{"request_id":"r2","session_id":"s1","model":"gpt-5","input_tokens":2000,"output_tokens":200,"timestamp":"2026-05-12T11:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, found, created, err := svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path}); err != nil || found != 2 || created != 2 {
		t.Fatalf("first scan found=%d created=%d err=%v", found, created, err)
	}

	second := `{"request_id":"r2","session_id":"s1","model":"gpt-5","input_tokens":3000,"output_tokens":300,"timestamp":"2026-05-12T11:00:00Z"}` + "\n" +
		`{"request_id":"r3","session_id":"s1","model":"gpt-5","input_tokens":4000,"output_tokens":400,"timestamp":"2026-05-12T12:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}
	rewriteTime := now.Add(time.Minute)
	if err := os.Chtimes(path, rewriteTime, rewriteTime); err != nil {
		t.Fatal(err)
	}

	if _, found, created, err := svc.scanFile(context.Background(), logCandidate{tool: "Codex", path: path}); err != nil || found != 2 || created != 1 {
		t.Fatalf("rewrite scan found=%d created=%d err=%v", found, created, err)
	}
	var rows []models.AICodingUsageRecord
	if err := db.Order("request_id asc").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected stale r1 to be removed, got %d rows: %+v", len(rows), rows)
	}
	if rows[0].RequestID != "r2" || rows[0].InputTokens != 3000 {
		t.Fatalf("expected r2 to be updated from rewritten source, got %+v", rows[0])
	}
	if rows[1].RequestID != "r3" {
		t.Fatalf("expected r3 to be inserted, got %+v", rows[1])
	}
}

func TestDashboardUsesLocalTimeWindowAndBuckets(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db, pricing.NewService(db, zerolog.Nop()), zerolog.Nop())
	location := time.FixedZone("CST", 8*60*60)
	svc.nowFunc = func() time.Time {
		return time.Date(2026, 5, 12, 10, 0, 0, 0, location)
	}

	if err := db.Create(&models.AICodingUsageRecord{
		ID:           "utc-evening",
		Tool:         "Codex",
		ProjectName:  "tokenbridge",
		Model:        "gpt-5",
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		OccurredAt:   time.Date(2026, 5, 11, 20, 0, 0, 0, time.UTC),
		CreatedAt:    time.Date(2026, 5, 12, 1, 0, 0, 0, time.UTC),
	}).Error; err != nil {
		t.Fatal(err)
	}

	dashboard, err := svc.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Summary.TotalRequests != 1 {
		t.Fatalf("expected UTC evening record in local current-day report, got %d requests", dashboard.Summary.TotalRequests)
	}
	if len(dashboard.Trend) != 1 || dashboard.Trend[0].Day != "05-12" || dashboard.Trend[0].Requests != 1 {
		t.Fatalf("expected local trend bucket 05-12, got %+v", dashboard.Trend)
	}
	var foundHour bool
	for _, point := range dashboard.Heatmap {
		if point.Day == "05-12" && point.Hour == 4 && point.Requests == 1 {
			foundHour = true
			break
		}
	}
	if !foundHour {
		t.Fatalf("expected local heatmap bucket 05-12 04:00, got %+v", dashboard.Heatmap)
	}
}

func TestProjectSpendClearsToolWhenMixed(t *testing.T) {
	db := openTestDB(t)
	svc := NewService(db, pricing.NewService(db, zerolog.Nop()), zerolog.Nop())
	now := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	svc.nowFunc = func() time.Time { return now }

	rows := []models.AICodingUsageRecord{
		{ID: "codex", Tool: "Codex", ProjectName: "tokenbridge", Model: "gpt-5", InputTokens: 100, TotalTokens: 100, OccurredAt: now, CreatedAt: now},
		{ID: "claude", Tool: "Claude Code", ProjectName: "tokenbridge", Model: "claude", InputTokens: 100, TotalTokens: 100, OccurredAt: now, CreatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	dashboard, err := svc.Dashboard(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboard.ProjectSpend) != 1 {
		t.Fatalf("expected one project row, got %+v", dashboard.ProjectSpend)
	}
	if dashboard.ProjectSpend[0].Tool != "" {
		t.Fatalf("expected mixed-tool project to clear tool label, got %+v", dashboard.ProjectSpend[0])
	}
}

func TestExportXLSXProducesWorkbook(t *testing.T) {
	body, err := exportXLSX(Dashboard{
		Summary: Summary{TotalRequests: 2, TotalTokens: 3000, TotalCostUSD: 0.0123, LocalOnly: true},
		Trend:   []TrendPoint{{Day: "05-12", Requests: 2, Tokens: 3000, CostUSD: 0.0123}},
		ModelRank: []Breakdown{{
			Name:     "gpt-5",
			CostUSD:  0.0123,
			Requests: 2,
			Tokens:   3000,
		}},
	}, 7.2)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(handle)
		_ = handle.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = string(data)
	}
	if !strings.Contains(files["xl/workbook.xml"], `name="Models"`) {
		t.Fatalf("workbook should include Models sheet: %s", files["xl/workbook.xml"])
	}
	if !strings.Contains(files["xl/worksheets/sheet1.xml"], "total_requests") {
		t.Fatalf("summary sheet should contain total_requests")
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ModelPricing{}, &models.AICodingUsageRecord{}, &models.AICodingLogSource{}); err != nil {
		t.Fatal(err)
	}
	return db
}
