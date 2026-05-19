package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"tokenbridge/internal/models"
)

func TestTestConnectionInputOpenAICompatibleSuccess(t *testing.T) {
	svc, db := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("probe path = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test-openai" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
	}))
	defer upstream.Close()

	result, err := svc.TestConnectionInput(context.Background(), ProviderInput{
		Name:    "DeepSeek",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "sk-test-openai",
	})
	if err != nil {
		t.Fatalf("TestConnectionInput() error = %v", err)
	}
	if result.Status != "healthy" {
		t.Fatalf("Status = %q, want healthy; message: %s", result.Status, result.Message)
	}
	if len(result.Models) != 1 || result.Models[0] != "deepseek-chat" {
		t.Fatalf("Models = %#v, want discovered model", result.Models)
	}

	var count int64
	if err := db.Model(&models.Provider{}).Count(&count).Error; err != nil {
		t.Fatalf("count providers: %v", err)
	}
	if count != 0 {
		t.Fatalf("temporary test wrote %d providers, want 0", count)
	}
}

func TestTestConnectionInputReturnsWarningForAuthenticationFailure(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer upstream.Close()

	result, err := svc.TestConnectionInput(context.Background(), ProviderInput{
		Name:    "Bad auth",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "sk-invalid",
	})
	if err != nil {
		t.Fatalf("TestConnectionInput() error = %v", err)
	}
	if result.Status != "warning" {
		t.Fatalf("Status = %q, want warning", result.Status)
	}
	if !strings.Contains(result.Message, "认证") && !strings.Contains(result.Message, "401") {
		t.Fatalf("Message = %q, want authentication detail", result.Message)
	}
}

func TestTestConnectionInputRejectsInvalidBaseURL(t *testing.T) {
	svc, _ := newProviderTestService(t)

	result, err := svc.TestConnectionInput(context.Background(), ProviderInput{
		Name:    "Invalid",
		Type:    TypeOpenAICompatible,
		BaseURL: "://bad-url",
		APIKey:  "sk-test",
	})
	if err != nil {
		t.Fatalf("TestConnectionInput() error = %v", err)
	}
	if result.Status != "warning" {
		t.Fatalf("Status = %q, want warning", result.Status)
	}
	if !strings.Contains(result.Message, "base_url") && !strings.Contains(result.Message, "地址") {
		t.Fatalf("Message = %q, want invalid base URL detail", result.Message)
	}
}

func TestTestConnectionInputAnthropicUsesAnthropicAndAuthorizationHeaders(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("probe path = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-ant-test" {
			t.Fatalf("x-api-key = %q, want token", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("anthropic-version header is empty")
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-ant-test" {
			t.Fatalf("Authorization = %q, want bearer token for Anthropic-compatible providers", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4"}]}`))
	}))
	defer upstream.Close()

	result, err := svc.TestConnectionInput(context.Background(), ProviderInput{
		Name:    "Claude",
		Type:    TypeAnthropic,
		BaseURL: upstream.URL,
		APIKey:  "sk-ant-test",
	})
	if err != nil {
		t.Fatalf("TestConnectionInput() error = %v", err)
	}
	if result.Status != "healthy" {
		t.Fatalf("Status = %q, want healthy; message: %s", result.Status, result.Message)
	}
}

func TestDiscoverModelsInputOpenAIUsesBearerHeader(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-openai" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("x-api-key"); got != "" {
			t.Fatalf("x-api-key = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer upstream.Close()

	models, err := svc.DiscoverModelsInput(context.Background(), ProviderInput{
		Name:    "OpenAI",
		Type:    TypeOpenAI,
		BaseURL: upstream.URL,
		APIKey:  "sk-openai",
	})
	if err != nil {
		t.Fatalf("DiscoverModelsInput() error = %v", err)
	}
	if want := []string{"gpt-4o", "gpt-4o-mini"}; !equalStrings(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestDiscoverModelsInputOpenAICompatibleUsesBearerHeader(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-compatible" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-chat"}]}`))
	}))
	defer upstream.Close()

	models, err := svc.DiscoverModelsInput(context.Background(), ProviderInput{
		Name:    "DeepSeek",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "sk-compatible",
	})
	if err != nil {
		t.Fatalf("DiscoverModelsInput() error = %v", err)
	}
	if want := []string{"deepseek-chat"}; !equalStrings(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestDiscoverModelsInputAnthropicUsesBothAuthHeaders(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-anthropic" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-anthropic" {
			t.Fatalf("x-api-key = %q, want token", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("anthropic-version header is empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4"}]}`))
	}))
	defer upstream.Close()

	models, err := svc.DiscoverModelsInput(context.Background(), ProviderInput{
		Name:    "Claude",
		Type:    TypeAnthropic,
		BaseURL: upstream.URL,
		APIKey:  "sk-anthropic",
	})
	if err != nil {
		t.Fatalf("DiscoverModelsInput() error = %v", err)
	}
	if want := []string{"claude-sonnet-4"}; !equalStrings(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestDiscoverModelsInputUsesSavedTokenWhenIDProvidedAndTokenOmitted(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-discover-saved" {
			t.Fatalf("Authorization = %q, want saved token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"saved-model"}]}`))
	}))
	defer upstream.Close()

	created, err := svc.Create(context.Background(), ProviderInput{
		Name:    "Saved",
		Type:    TypeOpenAICompatible,
		BaseURL: "https://old.example.com",
		APIKey:  "sk-discover-saved",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	models, err := svc.DiscoverModelsInput(context.Background(), ProviderInput{
		ID:      created.ID,
		Name:    "Saved draft",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
	})
	if err != nil {
		t.Fatalf("DiscoverModelsInput() error = %v", err)
	}
	if want := []string{"saved-model"}; !equalStrings(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestSavedProviderTestKeepsAndUsesExistingTokenWhenUpdateOmitsToken(t *testing.T) {
	svc, db := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-saved-token" {
			t.Fatalf("Authorization = %q, want saved token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
	}))
	defer upstream.Close()

	created, err := svc.Create(context.Background(), ProviderInput{
		Name:    "Saved",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "sk-saved-token",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := svc.Update(context.Background(), created.ID, ProviderInput{
		Name:    "Saved updated",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "",
		Enabled: true,
		Status:  "active",
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	result, err := svc.TestConnection(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
	if result.Status != "healthy" {
		t.Fatalf("Status = %q, want healthy; message: %s", result.Status, result.Message)
	}

	var saved models.Provider
	if err := db.First(&saved, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("load saved provider: %v", err)
	}
	if saved.APIKeyEncrypted != "sk-saved-token" {
		t.Fatalf("saved API key = %q, want original token", saved.APIKeyEncrypted)
	}
}

func TestTestConnectionInputUsesSavedTokenWhenIDProvidedAndTokenOmitted(t *testing.T) {
	svc, _ := newProviderTestService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-existing-token" {
			t.Fatalf("Authorization = %q, want existing token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o"}]}`))
	}))
	defer upstream.Close()

	created, err := svc.Create(context.Background(), ProviderInput{
		Name:    "Existing",
		Type:    TypeOpenAICompatible,
		BaseURL: "https://old.example.com",
		APIKey:  "sk-existing-token",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result, err := svc.TestConnectionInput(context.Background(), ProviderInput{
		ID:      created.ID,
		Name:    "Existing draft",
		Type:    TypeOpenAICompatible,
		BaseURL: upstream.URL,
		APIKey:  "",
	})
	if err != nil {
		t.Fatalf("TestConnectionInput() error = %v", err)
	}
	if result.Status != "healthy" {
		t.Fatalf("Status = %q, want healthy; message: %s", result.Status, result.Message)
	}
}

func newProviderTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Provider{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewService(db), db
}

func equalStrings(a []string, b []string) bool {
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
