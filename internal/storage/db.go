package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/config"
	"tokenbridge/internal/models"
)

func OpenDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
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
		&models.AICodingUsageRecord{},
		&models.AICodingLogSource{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	return db, nil
}
