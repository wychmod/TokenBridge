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
	ID                  string    `json:"id" gorm:"primaryKey"`
	LocalKeyID          string    `json:"local_key_id"`
	ProviderID          string    `json:"provider_id"`
	ModelRequested      string    `json:"model_requested"`
	ModelActual         string    `json:"model_actual"`
	APIFormat           string    `json:"api_format"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	ReasoningTokens     int64     `json:"reasoning_tokens"`
	ContextWindow       int64     `json:"context_window"`
	PricingTier         string    `json:"pricing_tier"`
	TotalCostUSD        float64   `json:"total_cost_usd"`
	CostBreakdownJSON   string    `json:"cost_breakdown_json"`
	PricingRuleJSON     string    `json:"pricing_rule_json"`
	TimeSource          string    `json:"time_source"`
	EventKey            string    `json:"event_key" gorm:"index"`
	ParserVersion       int       `json:"parser_version"`
	LatencyMS           int64     `json:"latency_ms"`
	Success             bool      `json:"success"`
	CreatedAt           time.Time `json:"created_at"`
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
	SupportsReasoning         bool      `json:"supports_reasoning"`
	PricingJSON               string    `json:"pricing_json"`
	FetchedAt                 time.Time `json:"fetched_at"`
}

type AICodingUsageRecord struct {
	ID                  string    `json:"id" gorm:"primaryKey"`
	Tool                string    `json:"tool" gorm:"index"`
	SessionID           string    `json:"session_id" gorm:"index"`
	RequestID           string    `json:"request_id" gorm:"index"`
	ProjectPath         string    `json:"project_path" gorm:"index"`
	ProjectName         string    `json:"project_name"`
	Model               string    `json:"model" gorm:"index"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	ReasoningTokens     int64     `json:"reasoning_tokens"`
	ContextWindow       int64     `json:"context_window"`
	PricingTier         string    `json:"pricing_tier"`
	TotalTokens         int64     `json:"total_tokens"`
	TotalCostUSD        float64   `json:"total_cost_usd"`
	CostBreakdownJSON   string    `json:"cost_breakdown_json"`
	PricingRuleJSON     string    `json:"pricing_rule_json"`
	PricingMatched      bool      `json:"pricing_matched"`
	PricingFallback     string    `json:"pricing_fallback"`
	EventKey            string    `json:"event_key" gorm:"index"`
	SourcePath          string    `json:"source_path" gorm:"index"`
	SourceOffset        int64     `json:"source_offset"`
	TimeSource          string    `json:"time_source"`
	ParserVersion       int       `json:"parser_version"`
	RawJSON             string    `json:"raw_json"`
	OccurredAt          time.Time `json:"occurred_at" gorm:"index"`
	CreatedAt           time.Time `json:"created_at"`
}

type AICodingLogSource struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	Tool           string    `json:"tool" gorm:"index"`
	Path           string    `json:"path" gorm:"uniqueIndex"`
	Size           int64     `json:"size"`
	ModTime        time.Time `json:"mod_time"`
	ParserVersion  int       `json:"parser_version"`
	LastScannedAt  time.Time `json:"last_scanned_at"`
	RecordsFound   int64     `json:"records_found"`
	RecordsCreated int64     `json:"records_created"`
	ErrorMessage   string    `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
