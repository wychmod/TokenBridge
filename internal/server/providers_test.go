package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/config"
	"tokenbridge/internal/models"
	"tokenbridge/internal/provider"
)

func TestHandleProvidersReturnsMaskedAPIKeyStatus(t *testing.T) {
	router, providerService, _ := newProviderRouterTest(t)
	if _, err := providerService.Create(context.Background(), provider.ProviderInput{
		Name:    "Masked",
		Type:    provider.TypeOpenAICompatible,
		BaseURL: "https://api.example.com",
		APIKey:  "sk-test-1234567890abcd",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/providers", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("provider count = %d, want 1", len(payload.Data))
	}
	record := payload.Data[0]
	if _, ok := record["api_key"]; ok {
		t.Fatalf("response exposed api_key: %#v", record["api_key"])
	}
	if got := record["has_api_key"]; got != true {
		t.Fatalf("has_api_key = %#v, want true", got)
	}
	if got := record["api_key_masked"]; got != "sk-...abcd" {
		t.Fatalf("api_key_masked = %#v, want sk-...abcd", got)
	}
}

func TestHandleTestProviderDraftUsesPayloadWithoutSaving(t *testing.T) {
	router, _, db := newProviderRouterTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-draft-token" {
			t.Fatalf("Authorization = %q, want draft token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
	}))
	defer upstream.Close()

	body, _ := json.Marshal(map[string]any{
		"name":     "Draft",
		"type":     provider.TypeOpenAICompatible,
		"base_url": upstream.URL,
		"api_key":  "sk-draft-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data provider.HealthCheckResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Data.Status != "healthy" {
		t.Fatalf("status = %q, want healthy; message: %s", payload.Data.Status, payload.Data.Message)
	}

	var count int64
	if err := db.Model(&models.Provider{}).Count(&count).Error; err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if count != 0 {
		t.Fatalf("draft test wrote %d providers, want 0", count)
	}
}

func TestHandleDiscoverProviderModelsDraftUsesPayloadWithoutSaving(t *testing.T) {
	router, _, db := newProviderRouterTest(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-draft-discover" {
			t.Fatalf("Authorization = %q, want draft token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"},{"id":"deepseek-reasoner"}]}`))
	}))
	defer upstream.Close()

	body, _ := json.Marshal(map[string]any{
		"name":     "Draft",
		"type":     provider.TypeOpenAICompatible,
		"base_url": upstream.URL,
		"api_key":  "sk-draft-discover",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers/discover-models", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if want := []string{"deepseek-chat", "deepseek-reasoner"}; !equalStringSlices(payload.Data, want) {
		t.Fatalf("models = %#v, want %#v", payload.Data, want)
	}

	var count int64
	if err := db.Model(&models.Provider{}).Count(&count).Error; err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if count != 0 {
		t.Fatalf("draft discovery wrote %d providers, want 0", count)
	}
}

func newProviderRouterTest(t *testing.T) (*Router, *provider.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Provider{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	providerService := provider.NewService(db)
	router := NewRouter(Dependencies{
		Config: config.Config{
			Security: config.SecurityConfig{AllowedOrigins: []string{"*"}},
		},
		Logger:    zerolog.Nop(),
		Providers: providerService,
		DB:        db,
	})
	return router, providerService, db
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
