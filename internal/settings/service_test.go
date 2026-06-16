package settings

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

func TestGetDefaultSettingsDisablesStartAtLogin(t *testing.T) {
	svc := newSettingsTestService(t)

	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.StartAtLogin {
		t.Fatal("StartAtLogin = true, want false by default")
	}
}

func TestSavePersistsStartAtLogin(t *testing.T) {
	svc := newSettingsTestService(t)

	saved, err := svc.Save(context.Background(), AppSettings{
		Host:           "127.0.0.1",
		Port:           18743,
		AdminPath:      "/admin",
		AdminUsername:  "admin",
		Theme:          "system",
		UpdateChannel:  "stable",
		BackupInterval: "24h",
		LogLevel:       "standard",
		RetentionDays:  30,
		BundleMode:     "single-binary",
		StartAtLogin:   true,
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !saved.StartAtLogin {
		t.Fatal("saved StartAtLogin = false, want true")
	}

	loaded, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() after Save() error = %v", err)
	}
	if !loaded.StartAtLogin {
		t.Fatal("loaded StartAtLogin = false, want true")
	}
}

func TestSaveReturnsAutostartError(t *testing.T) {
	wantErr := errors.New("autostart denied")
	svc := newSettingsTestServiceWithAutostart(t, failingAutostartManager{err: wantErr})

	_, err := svc.Save(context.Background(), AppSettings{StartAtLogin: true})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Save() error = %v, want %v", err, wantErr)
	}
}

func newSettingsTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(newSettingsTestDB(t))
}

func newSettingsTestServiceWithAutostart(t *testing.T, autostart AutostartManager) *Service {
	t.Helper()
	return NewServiceWithAutostart(newSettingsTestDB(t), autostart)
}

func newSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Setting{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

type failingAutostartManager struct {
	err error
}

func (m failingAutostartManager) Apply(bool) error {
	return m.err
}
