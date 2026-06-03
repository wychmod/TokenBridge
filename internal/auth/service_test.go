package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

func TestValidateTreatsZeroBudgetsAsUnlimited(t *testing.T) {
	db := openAuthTestDB(t)
	svc := NewService(db)

	key, rawKey, err := svc.Create(context.Background(), CreateKeyInput{
		Name:          "unlimited",
		MonthlyBudget: 0,
		TokenBudget:   0,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.LocalKey{}).Where("id = ?", key.ID).Updates(map[string]any{
		"current_spend":  999999.0,
		"current_tokens": int64(999999999),
	}).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Validate(context.Background(), rawKey); err != nil {
		t.Fatalf("zero budgets should be unlimited, got validation error: %v", err)
	}
}

func TestValidateRejectsPositiveBudgetExceeded(t *testing.T) {
	db := openAuthTestDB(t)
	svc := NewService(db)

	key, rawKey, err := svc.Create(context.Background(), CreateKeyInput{
		Name:          "limited",
		MonthlyBudget: 1,
		TokenBudget:   10,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.LocalKey{}).Where("id = ?", key.ID).Updates(map[string]any{
		"current_spend": 1.0,
	}).Error; err != nil {
		t.Fatal(err)
	}

	_, err = svc.Validate(context.Background(), rawKey)
	if err == nil || !strings.Contains(err.Error(), "monthly budget exceeded") {
		t.Fatalf("expected monthly budget exceeded error, got %v", err)
	}
}

func openAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.LocalKey{}); err != nil {
		t.Fatal(err)
	}
	return db
}
