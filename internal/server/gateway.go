package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"localgateway/internal/auth"
	"localgateway/internal/models"
	"localgateway/internal/pricing"
	"localgateway/internal/usage"
)

type openAIClient struct {
	httpClient *http.Client
}

type gatewayError struct {
	HTTPStatus int    `json:"-"`
	Type       string `json:"type"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Provider   string `json:"provider,omitempty"`
	Retryable  bool   `json:"retryable"`
}

func (e *gatewayError) Error() string {
	if e == nil {
		return "gateway error"
	}
	return e.Message
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
	} `json:"usage"`
}

type requestTrace struct {
	ID             string   `json:"id"`
	Provider       string   `json:"provider"`
	FallbackTried  []string `json:"fallbackTried,omitempty"`
	RequestedModel string   `json:"requestedModel"`
	ActualModel    string   `json:"actualModel"`
	APIFormat      string   `json:"apiFormat"`
}

func newRequestTrace(provider, requestedModel, actualModel, apiFormat string) requestTrace {
	return requestTrace{ID: "trace_" + uuid.NewString(), Provider: provider, RequestedModel: requestedModel, ActualModel: actualModel, APIFormat: apiFormat}
}

func newOpenAIClient(timeoutSeconds int) *openAIClient {
	timeout := 60 * time.Second
	if timeoutSeconds > 0 {
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	return &openAIClient{httpClient: &http.Client{Timeout: timeout}}
}

func (c *openAIClient) PostJSON(ctx context.Context, provider models.Provider, path string, payload []byte, extraHeaders map[string]string) (*http.Response, error) {
	baseURL := strings.TrimRight(provider.BaseURL, "/")
	if baseURL == "" {
		return nil, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "provider_base_url_missing", Message: "Provider 未配置 base_url", Provider: provider.Name, Retryable: false}
	}
	endpoint := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, &gatewayError{HTTPStatus: http.StatusInternalServerError, Type: "gateway_error", Code: "request_build_failed", Message: err.Error(), Provider: provider.Name, Retryable: false}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if provider.APIKeyEncrypted != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKeyEncrypted)
	}
	if provider.OrganizationID != "" {
		req.Header.Set("OpenAI-Organization", provider.OrganizationID)
	}
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "upstream_request_failed", Message: err.Error(), Provider: provider.Name, Retryable: true}
	}
	return resp, nil
}

func (c *openAIClient) ChatCompletions(ctx context.Context, provider models.Provider, payload []byte) (*http.Response, error) {
	return c.PostJSON(ctx, provider, "/v1/chat/completions", payload, nil)
}

func (c *openAIClient) ClaudeMessages(ctx context.Context, provider models.Provider, payload []byte) (*http.Response, error) {
	headers := map[string]string{"anthropic-version": "2023-06-01"}
	if strings.EqualFold(provider.Type, "anthropic") {
		headers["x-api-key"] = provider.APIKeyEncrypted
	}
	return c.PostJSON(ctx, provider, "/v1/messages", payload, headers)
}

func writeGatewayError(w http.ResponseWriter, err error) {
	var gwErr *gatewayError
	if ok := errorAsGateway(err, &gwErr); ok {
		respondJSON(w, gwErr.HTTPStatus, map[string]any{"error": gwErr})
		return
	}
	respondJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"type": "internal_error", "code": "unexpected_error", "message": err.Error(), "retryable": false}})
}

func errorAsGateway(err error, target **gatewayError) bool {
	gwErr, ok := err.(*gatewayError)
	if ok {
		*target = gwErr
		return true
	}
	return false
}

func extractLocalKey(req *http.Request) string {
	authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return strings.TrimSpace(req.Header.Get("X-Api-Key"))
}

func validateLocalKey(ctx context.Context, svc *auth.Service, rawKey string) (*models.LocalKey, error) {
	if strings.TrimSpace(rawKey) == "" {
		return nil, &gatewayError{HTTPStatus: http.StatusUnauthorized, Type: "auth_error", Code: "local_key_missing", Message: "缺少本地访问密钥，请通过 Authorization 或 X-Api-Key 传入。", Retryable: false}
	}
	entity, err := svc.Validate(ctx, rawKey)
	if err != nil {
		return nil, &gatewayError{HTTPStatus: http.StatusUnauthorized, Type: "auth_error", Code: "local_key_invalid", Message: err.Error(), Retryable: false}
	}
	return entity, nil
}

func ensureKeyAllowed(key *models.LocalKey, provider models.Provider, model string) error {
	allowedModels := decodeJSONStringArray(key.AllowedModelsJSON)
	if len(allowedModels) > 0 && !stringInSlice(allowedModels, model) {
		return &gatewayError{HTTPStatus: http.StatusForbidden, Type: "auth_error", Code: "model_not_allowed", Message: fmt.Sprintf("本地密钥未授权访问模型 %s", model), Provider: provider.Name, Retryable: false}
	}
	allowedProviders := decodeJSONStringArray(key.AllowedProvidersJSON)
	if len(allowedProviders) > 0 && !stringInSlice(allowedProviders, provider.Name) && !stringInSlice(allowedProviders, provider.ID) {
		return &gatewayError{HTTPStatus: http.StatusForbidden, Type: "auth_error", Code: "provider_not_allowed", Message: fmt.Sprintf("本地密钥未授权访问 Provider %s", provider.Name), Provider: provider.Name, Retryable: false}
	}
	return nil
}

func decodeJSONStringArray(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var items []string
	_ = json.Unmarshal([]byte(raw), &items)
	return items
}

func stringInSlice(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func recordUsageBestEffort(ctx context.Context, svc *usage.Service, pricingSvc *pricing.Service, authSvc *auth.Service, localKeyID string, providerID string, requestedModel string, actualModel string, apiFormat string, latencyMS int64, success bool, usageInfo openAIChatResponse) {
	modelID := actualModel
	if modelID == "" {
		modelID = requestedModel
	}
	cost := 0.0
	if pricingSvc != nil {
		cost = pricingSvc.CalculateCost(ctx, modelID, usageInfo.Usage.PromptTokens, usageInfo.Usage.CompletionTokens)
	}
	_ = svc.Record(ctx, usage.RecordInput{LocalKeyID: localKeyID, ProviderID: providerID, ModelRequested: requestedModel, ModelActual: actualModel, APIFormat: apiFormat, InputTokens: usageInfo.Usage.PromptTokens, OutputTokens: usageInfo.Usage.CompletionTokens, TotalCostUSD: cost, LatencyMS: latencyMS, Success: success})
	if success && authSvc != nil {
		_ = authSvc.DeductUsage(ctx, localKeyID, cost, usageInfo.Usage.PromptTokens, usageInfo.Usage.CompletionTokens)
	}
}

func recordClaudeUsageBestEffort(ctx context.Context, svc *usage.Service, pricingSvc *pricing.Service, authSvc *auth.Service, localKeyID string, providerID string, requestedModel string, actualModel string, apiFormat string, latencyMS int64, success bool, resp claudeMessagesResponse) {
	modelID := actualModel
	if modelID == "" {
		modelID = requestedModel
	}
	cost := 0.0
	if pricingSvc != nil {
		cost = pricingSvc.CalculateCost(ctx, modelID, resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}
	_ = svc.Record(ctx, usage.RecordInput{LocalKeyID: localKeyID, ProviderID: providerID, ModelRequested: requestedModel, ModelActual: actualModel, APIFormat: apiFormat, InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens, TotalCostUSD: cost, LatencyMS: latencyMS, Success: success})
	if success && authSvc != nil {
		_ = authSvc.DeductUsage(ctx, localKeyID, cost, resp.Usage.InputTokens, resp.Usage.OutputTokens)
	}
}

func readBodyAndClose(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	return io.ReadAll(body)
}

func logRequestBestEffort(ctx context.Context, db *gorm.DB, localKeyID string, providerID string, path string, method string, statusCode int, latencyMS int64, errMsg string, metadata any) {
	if db == nil {
		return
	}
	data, _ := json.Marshal(metadata)
	_ = db.WithContext(ctx).Create(&models.RequestLog{ID: "log_" + uuid.NewString(), LocalKeyID: localKeyID, ProviderID: providerID, Path: path, Method: method, StatusCode: statusCode, LatencyMS: latencyMS, ErrorMessage: errMsg, MetadataJSON: string(data), CreatedAt: time.Now()}).Error
}

// injectStreamOptions adds stream_options.include_usage=true to an OpenAI stream request
// so the upstream returns token usage in the final SSE chunk.
func injectStreamOptions(requestBytes []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(requestBytes, &m); err != nil {
		return requestBytes
	}
	m["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	out, err := json.Marshal(m)
	if err != nil {
		return requestBytes
	}
	return out
}
