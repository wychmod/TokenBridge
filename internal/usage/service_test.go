package usage

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

func TestRecordDedupesGatewayEventKey(t *testing.T) {
	db := openUsageTestDB(t)
	svc := NewService(db)

	input := RecordInput{
		LocalKeyID:     "key_1",
		ProviderID:     "provider_1",
		ModelRequested: "gpt-5",
		ModelActual:    "gpt-5",
		APIFormat:      "openai",
		InputTokens:    100,
		OutputTokens:   20,
		TotalCostUSD:   0.001,
		EventKey:       "chatcmpl_same",
		Success:        true,
	}
	if err := svc.Record(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.OutputTokens = 30
	input.TotalCostUSD = 0.002
	if err := svc.Record(context.Background(), input); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&models.UsageRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected repeated gateway event_key to be recorded once, got %d rows", count)
	}
	var row models.UsageRecord
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.OutputTokens != 30 || row.TotalCostUSD != 0.002 {
		t.Fatalf("expected second record to update same event row, got %+v", row)
	}
	if row.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected gateway created_at to be stored in UTC, got %s", row.CreatedAt.Location())
	}
}

func TestTrendUsesBeijingDayBoundary(t *testing.T) {
	db := openUsageTestDB(t)
	svc := NewService(db)
	rows := []models.UsageRecord{
		{
			ID:           "today",
			ProviderID:   "provider",
			InputTokens:  100,
			OutputTokens: 20,
			TotalCostUSD: 0.1,
			Success:      true,
			CreatedAt:    time.Now().In(beijingLocation()),
		},
		{
			ID:           "old",
			ProviderID:   "provider",
			InputTokens:  100,
			OutputTokens: 20,
			TotalCostUSD: 0.1,
			Success:      true,
			CreatedAt:    startOfDay(time.Now().In(beijingLocation())).UTC().Add(-time.Nanosecond),
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	points, err := svc.Trend(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Requests != 1 {
		t.Fatalf("expected only Beijing current-day row in one-day trend, got %+v", points)
	}
}

func openUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.UsageRecord{}); err != nil {
		t.Fatal(err)
	}
	return db
}
