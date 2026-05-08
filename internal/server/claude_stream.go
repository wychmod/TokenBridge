package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"localgateway/internal/models"
	"localgateway/internal/routing"
	"localgateway/internal/usage"
)

func (r *Router) handleClaudeMessagesStream(w http.ResponseWriter, req *http.Request, payload claudeMessagesRequest, localKey *models.LocalKey, decision *routing.Decision) {
	trace := newRequestTrace(decision.Provider.Name, payload.Model, decision.Model, "claude_stream")
	startedAt := time.Now()

	responseBody, providerID, providerName, statusCode, fallbackTried, err := r.forwardClaudeStreamWithFallback(req, localKey, decision, payload)
	trace.Provider = providerName
	trace.FallbackTried = fallbackTried
	if err != nil {
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, providerID, "/v1/messages", req.Method, statusCode, time.Since(startedAt).Milliseconds(), err.Error(), trace)
		writeGatewayError(w, err)
		return
	}
	defer responseBody.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusInternalServerError, Type: "gateway_error", Code: "streaming_not_supported", Message: "当前服务端不支持流式刷新", Retryable: false})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Request-Trace-Id", trace.ID)
	w.WriteHeader(http.StatusOK)

	// Track usage from Claude SSE stream
	// Claude sends usage in "message_start" (input_tokens) and "message_delta" (output_tokens)
	var streamInputTokens int64
	var streamOutputTokens int64
	currentEvent := ""

	reader := bufio.NewReader(responseBody)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = w.Write(line)
			flusher.Flush()

			lineStr := strings.TrimSpace(string(line))
			// Track SSE event type
			if strings.HasPrefix(lineStr, "event: ") {
				currentEvent = strings.TrimPrefix(lineStr, "event: ")
			}
			// Parse usage from message_start (has input_tokens)
			if currentEvent == "message_start" && strings.HasPrefix(lineStr, "data: ") {
				var msgStart struct {
					Message struct {
						Usage struct {
							InputTokens  int64 `json:"input_tokens"`
							OutputTokens int64 `json:"output_tokens"`
						} `json:"usage"`
					} `json:"message"`
				}
				if json.Unmarshal([]byte(lineStr[6:]), &msgStart) == nil {
					if msgStart.Message.Usage.InputTokens > 0 {
						streamInputTokens = msgStart.Message.Usage.InputTokens
					}
				}
			}
			// Parse usage from message_delta (has output_tokens)
			if currentEvent == "message_delta" && strings.HasPrefix(lineStr, "data: ") {
				var msgDelta struct {
					Usage struct {
						OutputTokens int64 `json:"output_tokens"`
					} `json:"usage"`
				}
				if json.Unmarshal([]byte(lineStr[6:]), &msgDelta) == nil {
					if msgDelta.Usage.OutputTokens > 0 {
						streamOutputTokens = msgDelta.Usage.OutputTokens
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	// Record usage if we got it from the stream
	if streamInputTokens > 0 || streamOutputTokens > 0 {
		modelID := decision.Model
		cost := 0.0
		if r.deps.Pricing != nil {
			cost = r.deps.Pricing.CalculateCost(req.Context(), modelID, streamInputTokens, streamOutputTokens)
		}
		_ = r.deps.Usage.Record(req.Context(), usage.RecordInput{
			LocalKeyID:     localKey.ID,
			ProviderID:     providerID,
			ModelRequested: payload.Model,
			ModelActual:    decision.Model,
			APIFormat:      "claude_stream",
			InputTokens:    streamInputTokens,
			OutputTokens:   streamOutputTokens,
			TotalCostUSD:   cost,
			LatencyMS:      time.Since(startedAt).Milliseconds(),
			Success:        true,
		})
		if r.deps.Keys != nil {
			_ = r.deps.Keys.DeductUsage(req.Context(), localKey.ID, cost, streamInputTokens, streamOutputTokens)
		}
	}

	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, providerID, "/v1/messages", req.Method, http.StatusOK, time.Since(startedAt).Milliseconds(), "", trace)
}

func (r *Router) forwardClaudeStreamWithFallback(req *http.Request, localKey *models.LocalKey, decision *routing.Decision, payload claudeMessagesRequest) (io.ReadCloser, string, string, int, []string, error) {
	client := newOpenAIClient(r.deps.Config.Proxy.StreamTimeout)
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

	payload.Stream = true
	requestBytes, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return nil, decision.Provider.ID, decision.Provider.Name, http.StatusInternalServerError, fallbackTried, &gatewayError{HTTPStatus: http.StatusInternalServerError, Type: "gateway_error", Code: "request_marshal_failed", Message: marshalErr.Error(), Provider: decision.Provider.Name, Retryable: false}
	}

	var lastErr error
	for index, attempt := range attempts {
		if err := ensureKeyAllowed(localKey, attempt, decision.Model); err != nil {
			lastErr = err
			continue
		}
		resp, err := client.ClaudeMessages(req.Context(), attempt, requestBytes)
		if err != nil {
			lastErr = err
			if index > 0 {
				fallbackTried = append(fallbackTried, attempt.Name)
			}
			continue
		}
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
			body, _ := readBodyAndClose(resp.Body)
			lastErr = mapUpstreamError(resp.StatusCode, body, attempt.Name)
			if index > 0 {
				fallbackTried = append(fallbackTried, attempt.Name)
			}
			continue
		}
		if resp.StatusCode >= 400 {
			body, _ := readBodyAndClose(resp.Body)
			return nil, attempt.ID, attempt.Name, resp.StatusCode, fallbackTried, mapUpstreamError(resp.StatusCode, body, attempt.Name)
		}
		return resp.Body, attempt.ID, attempt.Name, http.StatusOK, fallbackTried, nil
	}

	if lastErr == nil {
		lastErr = &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "no_available_provider", Message: "没有可用的 Provider 完成 Claude 流式请求", Retryable: true}
	}
	return nil, decision.Provider.ID, decision.Provider.Name, http.StatusBadGateway, fallbackTried, lastErr
}
