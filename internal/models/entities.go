package models

import "time"

type Provider struct {
	ID              string     `json:"id" gorm:"primaryKey"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	BaseURL         string     `json:"base_url"`
	APIKeyEncrypted string     `json:"-"`
	OrganizationID  string     `json:"organization_id"`
	Enabled         bool       `json:"enabled"`
	Priority        int        `json:"priority"`
	Status          string     `json:"status"`
	ModelsJSON      string     `json:"models_json"`
	RateLimitRPM    int        `json:"rate_limit_rpm"`
	RateLimitTPM    int        `json:"rate_limit_tpm"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type LocalKey struct {
	ID                   string     `json:"id" gorm:"primaryKey"`
	Name                 string     `json:"name"`
	KeyHash              string     `json:"-"`
	DisplayKey           string     `json:"display_key" gorm:"-"`
	AllowedModelsJSON    string     `json:"allowed_models_json"`
	AllowedProvidersJSON string     `json:"allowed_providers_json"`
	MonthlyBudget        float64    `json:"monthly_budget"`
	CurrentSpend         float64    `json:"current_spend"`
	TokenBudget          int64      `json:"token_budget"`
	CurrentTokens        int64      `json:"current_tokens"`
	Enabled              bool       `json:"enabled"`
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
}

type RoutingRule struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	ModelPattern  string    `json:"model_pattern"`
	Strategy      string    `json:"strategy"`
	ProviderChain string    `json:"provider_chain"`
	FallbackChain string    `json:"fallback_chain"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ModelAlias struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	Alias         string    `json:"alias"`
	Target        string    `json:"target"`
	FallbackChain string    `json:"fallback_chain"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey"`
	ValueJSON string    `json:"value_json"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UsageRecord struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	LocalKeyID     string    `json:"local_key_id"`
	ProviderID     string    `json:"provider_id"`
	ModelRequested string    `json:"model_requested"`
	ModelActual    string    `json:"model_actual"`
	APIFormat      string    `json:"api_format"`
	InputTokens    int64     `json:"input_tokens"`
	OutputTokens   int64     `json:"output_tokens"`
	TotalCostUSD   float64   `json:"total_cost_usd"`
	LatencyMS      int64     `json:"latency_ms"`
	Success        bool      `json:"success"`
	CreatedAt      time.Time `json:"created_at"`
}

type RequestLog struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	LocalKeyID   string    `json:"local_key_id"`
	ProviderID   string    `json:"provider_id"`
	Path         string    `json:"path"`
	Method       string    `json:"method"`
	StatusCode   int       `json:"status_code"`
	LatencyMS    int64     `json:"latency_ms"`
	ErrorMessage string    `json:"error_message"`
	MetadataJSON string    `json:"metadata_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type ModelPricing struct {
	ModelID                   string    `json:"model_id" gorm:"primaryKey"`
	LitellmProvider           string    `json:"litellm_provider"`
	Mode                      string    `json:"mode"`
	MaxInputTokens            int64     `json:"max_input_tokens"`
	MaxOutputTokens           int64     `json:"max_output_tokens"`
	InputCostPerToken         float64   `json:"input_cost_per_token"`
	OutputCostPerToken        float64   `json:"output_cost_per_token"`
	CacheCreationCostPerToken float64   `json:"cache_creation_cost_per_token"`
	CacheReadCostPerToken     float64   `json:"cache_read_cost_per_token"`
	SupportsVision            bool      `json:"supports_vision"`
	SupportsFunctionCalling   bool      `json:"supports_function_calling"`
	SupportsPromptCaching     bool      `json:"supports_prompt_caching"`
	FetchedAt                 time.Time `json:"fetched_at"`
}
