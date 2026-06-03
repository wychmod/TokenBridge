package admin

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/auth"
	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
	"tokenbridge/internal/provider"
	"tokenbridge/internal/requestlog"
	"tokenbridge/internal/routing"
	"tokenbridge/internal/settings"
	"tokenbridge/internal/usage"
)

func TestDashboardNormalizesProviderHealthStatus(t *testing.T) {
	db := openAdminTestDB(t)
	if err := db.Create(&[]models.Provider{
		{
			ID:         "prov-active",
			Name:       "Active provider",
			Type:       provider.TypeOpenAICompatible,
			Enabled:    true,
			Status:     "active",
			Priority:   1,
			ModelsJSON: `["gpt-4o-mini"]`,
		},
		{
			ID:         "prov-disabled",
			Name:       "Disabled provider",
			Type:       provider.TypeOpenAICompatible,
			Enabled:    false,
			Status:     "active",
			Priority:   2,
			ModelsJSON: `["gpt-4o-mini"]`,
		},
		{
			ID:         "prov-warning",
			Name:       "Warning provider",
			Type:       provider.TypeOpenAICompatible,
			Enabled:    true,
			Status:     "warning",
			Priority:   3,
			ModelsJSON: `["gpt-4o-mini"]`,
		},
	}).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}

	dashboard, err := newAdminTestService(db).Dashboard(context.Background())
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if len(dashboard.ProviderHealth) != 3 {
		t.Fatalf("ProviderHealth length = %d, want 3", len(dashboard.ProviderHealth))
	}
	want := []string{"healthy", "disabled", "warning"}
	for i, status := range want {
		if dashboard.ProviderHealth[i].Status != status {
			t.Fatalf("ProviderHealth[%d].Status = %q, want %q", i, dashboard.ProviderHealth[i].Status, status)
		}
	}
}

func openAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Provider{},
		&models.LocalKey{},
		&models.RoutingRule{},
		&models.ModelAlias{},
		&models.Setting{},
		&models.UsageRecord{},
		&models.RequestLog{},
		&models.ModelPricing{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func newAdminTestService(db *gorm.DB) *Service {
	providerService := provider.NewService(db)
	return NewService(
		providerService,
		auth.NewService(db),
		usage.NewService(db),
		pricing.NewService(db, zerolog.Nop()),
		routing.NewService(db, providerService, "priority"),
		settings.NewService(db),
		requestlog.NewService(db),
	)
}
