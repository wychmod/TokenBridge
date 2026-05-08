package server

import (
	"encoding/json"
	"net/http"
	"time"
)

type claudeMessagesRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	System    string `json:"system"`
	Stream    bool   `json:"stream"`
	Messages  []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
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
	var payload claudeMessagesRequest
	if err := decodeJSON(req, &payload); err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "invalid_json", Message: err.Error(), Retryable: false})
		return
	}
	if payload.Model == "" {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "model_required", Message: "model 不能为空", Retryable: false})
		return
	}

	localKey, err := validateLocalKey(req.Context(), r.deps.Keys, extractLocalKey(req))
	if err != nil {
		writeGatewayError(w, err)
		return
	}

	decision, err := r.deps.Routing.Decide(req.Context(), payload.Model)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "routing_error", Code: "route_decision_failed", Message: err.Error(), Retryable: true})
		return
	}
	if err := ensureKeyAllowed(localKey, decision.Provider, decision.Model); err != nil {
		writeGatewayError(w, err)
		return
	}

	if payload.Stream {
		r.handleClaudeMessagesStream(w, req, payload, localKey, decision)
		return
	}

	requestBytes, err := json.Marshal(payload)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusInternalServerError, Type: "gateway_error", Code: "request_marshal_failed", Message: err.Error(), Provider: decision.Provider.Name, Retryable: false})
		return
	}

	trace := newRequestTrace(decision.Provider.Name, payload.Model, decision.Model, "claude")
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
		recordClaudeUsageBestEffort(req.Context(), r.deps.Usage, r.deps.Pricing, r.deps.Keys, localKey.ID, decision.Provider.ID, payload.Model, decision.Model, "claude", time.Since(startedAt).Milliseconds(), true, claudeResp)
	}
	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/messages", req.Method, http.StatusOK, time.Since(startedAt).Milliseconds(), "", trace)
}
