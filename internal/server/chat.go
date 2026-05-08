package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"localgateway/internal/models"
	"localgateway/internal/routing"
)

type chatCompletionRequest struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func (r *Router) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	var payload chatCompletionRequest
	if err := decodeJSON(req, &payload); err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "invalid_json", Message: err.Error(), Retryable: false})
		return
	}
	if payload.Model == "" {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "model_required", Message: "model 不能为空", Retryable: false})
		return
	}
	if len(payload.Messages) == 0 {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadRequest, Type: "request_error", Code: "messages_required", Message: "messages 不能为空", Retryable: false})
		return
	}
	if payload.Stream {
		r.handleChatCompletionsStream(w, req, payload)
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

	trace := newRequestTrace(decision.Provider.Name, payload.Model, decision.Model, "openai")
	requestBytes, err := json.Marshal(payload)
	if err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusInternalServerError, Type: "gateway_error", Code: "request_marshal_failed", Message: err.Error(), Provider: decision.Provider.Name, Retryable: false})
		return
	}

	responseBytes, providerID, providerName, upstream, statusCode, latencyMS, fallbackTried, err := r.forwardChatWithFallback(req, localKey, decision, requestBytes)
	trace.Provider = providerName
	trace.FallbackTried = fallbackTried
	if err != nil {
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, providerID, "/v1/chat/completions", req.Method, statusCode, latencyMS, err.Error(), trace)
		writeGatewayError(w, err)
		return
	}

	recordUsageBestEffort(req.Context(), r.deps.Usage, r.deps.Pricing, r.deps.Keys, localKey.ID, providerID, payload.Model, decision.Model, "openai", latencyMS, true, upstream)
	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, providerID, "/v1/chat/completions", req.Method, http.StatusOK, latencyMS, "", trace)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Trace-Id", trace.ID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBytes)
}

func (r *Router) forwardChatWithFallback(req *http.Request, localKey *models.LocalKey, decision *routing.Decision, requestBytes []byte) ([]byte, string, string, openAIChatResponse, int, int64, []string, error) {
	client := newOpenAIClient(r.deps.Config.Proxy.RequestTimeout)
	startedAt := time.Now()
	attempts := []models.Provider{decision.Provider}
	fallbackTried := []string{}

	if len(decision.Fallback) > 0 {
		providers, err := r.deps.Providers.List(req.Context())
		if err == nil {
			for _, fallbackID := range decision.Fallback {
				for _, item := range providers {
					if strings.EqualFold(item.ID, fallbackID) || strings.EqualFold(item.Name, fallbackID) {
						attempts = append(attempts, item)
					}
				}
			}
		}
	}

	var lastErr error
	for index, attempt := range attempts {
		if err := ensureKeyAllowed(localKey, attempt, decision.Model); err != nil {
			lastErr = err
			continue
		}
		resp, err := client.ChatCompletions(req.Context(), attempt, requestBytes)
		if err != nil {
			lastErr = err
			if index > 0 {
				fallbackTried = append(fallbackTried, attempt.Name)
			}
			continue
		}
		responseBytes, readErr := readBodyAndClose(resp.Body)
		if readErr != nil {
			lastErr = &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "upstream_read_failed", Message: readErr.Error(), Provider: attempt.Name, Retryable: true}
			continue
		}
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = mapUpstreamError(resp.StatusCode, responseBytes, attempt.Name)
			if index > 0 {
				fallbackTried = append(fallbackTried, attempt.Name)
			}
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, attempt.ID, attempt.Name, openAIChatResponse{}, resp.StatusCode, time.Since(startedAt).Milliseconds(), fallbackTried, mapUpstreamError(resp.StatusCode, responseBytes, attempt.Name)
		}
		var upstream openAIChatResponse
		if err := json.Unmarshal(responseBytes, &upstream); err != nil {
			return nil, attempt.ID, attempt.Name, openAIChatResponse{}, http.StatusBadGateway, time.Since(startedAt).Milliseconds(), fallbackTried, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "upstream_invalid_json", Message: "上游返回了无法解析的 JSON 响应", Provider: attempt.Name, Retryable: true}
		}
		return responseBytes, attempt.ID, attempt.Name, upstream, http.StatusOK, time.Since(startedAt).Milliseconds(), fallbackTried, nil
	}

	if lastErr == nil {
		lastErr = &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "no_available_provider", Message: "没有可用的 Provider 完成请求", Retryable: true}
	}
	return nil, decision.Provider.ID, decision.Provider.Name, openAIChatResponse{}, http.StatusBadGateway, time.Since(startedAt).Milliseconds(), fallbackTried, lastErr
}

func mapUpstreamError(statusCode int, body []byte, providerName string) error {
	message := http.StatusText(statusCode)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if errorObj, ok := payload["error"].(map[string]any); ok {
			if msg, ok := errorObj["message"].(string); ok && msg != "" {
				message = msg
			}
		}
	}

	mapped := &gatewayError{HTTPStatus: statusCode, Type: "provider_error", Provider: providerName, Message: message, Retryable: statusCode == http.StatusTooManyRequests || statusCode >= 500}
	switch statusCode {
	case http.StatusUnauthorized:
		mapped.Code = "upstream_401"
	case http.StatusForbidden:
		mapped.Code = "upstream_403"
	case http.StatusNotFound:
		mapped.Code = "upstream_404"
	case http.StatusTooManyRequests:
		mapped.Code = "upstream_429"
	case http.StatusBadRequest:
		mapped.Code = "upstream_400"
	default:
		if statusCode >= 500 {
			mapped.Code = "upstream_5xx"
		} else {
			mapped.Code = "upstream_error"
		}
	}
	return mapped
}
