package server

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"localgateway/internal/provider"
)

// claudeMessagesMeta extracts only the fields needed for gateway routing logic.
// All other fields (tools, tool_choice, thinking, temperature, top_p, top_k,
// stop_sequences, metadata, etc.) are preserved in the raw body for passthrough.
type claudeMessagesMeta struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type claudeMessagesResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (r *Router) handleClaudeMessages(w http.ResponseWriter, req *http.Request) {
	// Read the raw body — this preserves ALL fields for passthrough
	requestBytes, err := readRequestBody(req)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "invalid_body", Message: err.Error(), Retryable: false})
		return
	}

	// Extract only the fields needed for routing decisions
	var meta claudeMessagesMeta
	if err := json.Unmarshal(requestBytes, &meta); err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "invalid_json", Message: err.Error(), Retryable: false})
		return
	}
	if meta.Model == "" {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "model_required", Message: "model 不能为空", Retryable: false})
		return
	}

	localKey, err := validateLocalKey(req.Context(), r.deps.Keys, extractLocalKey(req))
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	decision, err := r.deps.Routing.Decide(req.Context(), meta.Model)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "routing_error", Code: "route_decision_failed", Message: err.Error(), Retryable: true})
		return
	}
	// Check format compatibility: Claude format only works with Anthropic providers
	if err := provider.ValidateFormatCompatibility(decision.Provider.Type, provider.APIFormatClaude); err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "format_error", Code: "format_incompatible", Message: err.Error(), Provider: decision.Provider.Name, Retryable: false})
		return
	}
	if err := ensureKeyAllowed(localKey, decision.Provider, decision.Model); err != nil {
		writeGatewayError(w, err)
		return
	}

	if meta.Stream {
		r.handleClaudeMessagesStream(w, req, requestBytes, meta, localKey, decision)
		return
	}

	trace := newRequestTrace(decision.Provider.Name, meta.Model, decision.Model, "claude")
	client := newOpenAIClient(r.deps.Config.Proxy.RequestTimeout)
	startedAt := time.Now()
	resp, err := client.ClaudeMessages(req.Context(), decision.Provider, requestBytes)
	if err != nil {
		writeGatewayError(w, err)
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/messages", req.Method, http.StatusBadGateway, time.Since(startedAt).Milliseconds(), err.Error(), trace)
		return
	}

	responseBytes, err := readBodyAndClose(resp.Body)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "upstream_read_failed", Message: err.Error(), Provider: decision.Provider.Name, Retryable: true})
		return
	}
	if resp.StatusCode >= 400 {
		gwErr := mapUpstreamError(resp.StatusCode, responseBytes, decision.Provider.Name)
		writeGatewayError(w, gwErr)
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/messages", req.Method, resp.StatusCode, time.Since(startedAt).Milliseconds(), gwErr.Error(), trace)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Trace-Id", trace.ID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)

	// Parse Claude response for usage recording
	var claudeResp claudeMessagesResponse
	if err := json.Unmarshal(responseBytes, &claudeResp); err == nil {
		recordClaudeUsageBestEffort(req.Context(), r.deps.Usage, r.deps.Pricing, r.deps.Keys, localKey.ID, decision.Provider.ID, meta.Model, decision.Model, "claude", time.Since(startedAt).Milliseconds(), true, claudeResp)
	}
	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/messages", req.Method, http.StatusOK, time.Since(startedAt).Milliseconds(), "", trace)
}

// readRequestBody reads and returns the full request body bytes.
func readRequestBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	return io.ReadAll(req.Body)
}
