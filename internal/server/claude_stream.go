package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"tokenbridge/internal/models"
	"tokenbridge/internal/pricing"
	"tokenbridge/internal/provider"
	"tokenbridge/internal/routing"
	"tokenbridge/internal/usage"
)

func (r *Router) handleClaudeMessagesStream(w http.ResponseWriter, req *http.Request, requestBytes []byte, meta claudeMessagesMeta, localKey *models.LocalKey, decision *routing.Decision) {
	trace := newRequestTrace(decision.Provider.Name, meta.Model, decision.Model, "claude_stream")
	startedAt := time.Now()

	responseBody, providerID, providerName, statusCode, fallbackTried, err := r.forwardClaudeStreamWithFallback(req, localKey, decision, requestBytes)
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

	// Track usage from Claude SSE stream.
	currentEvent := ""
	streamUsage := claudeStreamUsage{}

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
			parseClaudeStreamUsageLine(currentEvent, lineStr, &streamUsage)
		}
		if readErr != nil {
			break
		}
	}

	// Record usage if we got it from the stream
	if streamUsage.InputTokens > 0 || streamUsage.OutputTokens > 0 {
		modelID := decision.Model
		inputTokens := streamUsage.NormalizedInputTokens()
		var cost pricing.CostBreakdown
		if r.deps.Pricing != nil {
			cost = r.deps.Pricing.CalculateCostDetailed(req.Context(), pricing.CostInput{
				ModelID:             modelID,
				InputTokens:         inputTokens,
				OutputTokens:        streamUsage.OutputTokens,
				CacheCreationTokens: streamUsage.CacheCreationInputTokens,
				CacheReadTokens:     streamUsage.CacheReadInputTokens,
				ContextWindow:       inputTokens,
			})
		}
		_ = r.deps.Usage.Record(req.Context(), usage.RecordInput{
			LocalKeyID:          localKey.ID,
			ProviderID:          providerID,
			ModelRequested:      meta.Model,
			ModelActual:         decision.Model,
			APIFormat:           "claude_stream",
			InputTokens:         inputTokens,
			OutputTokens:        streamUsage.OutputTokens,
			CacheCreationTokens: streamUsage.CacheCreationInputTokens,
			CacheReadTokens:     streamUsage.CacheReadInputTokens,
			ContextWindow:       inputTokens,
			TotalCostUSD:        cost.TotalUSD,
			CostBreakdownJSON:   cost.CostBreakdownJSON,
			PricingRuleJSON:     cost.PricingRuleJSON,
			TimeSource:          "gateway_created_at",
			EventKey:            trace.ID,
			ParserVersion:       1,
			LatencyMS:           time.Since(startedAt).Milliseconds(),
			Success:             true,
		})
		if r.deps.Keys != nil {
			_ = r.deps.Keys.DeductUsage(req.Context(), localKey.ID, cost.TotalUSD, inputTokens, streamUsage.OutputTokens)
		}
	}

	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, providerID, "/v1/messages", req.Method, http.StatusOK, time.Since(startedAt).Milliseconds(), "", trace)
}

type claudeStreamUsage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

func (u claudeStreamUsage) NormalizedInputTokens() int64 {
	return normalizeSeparateCacheInputTokens(u.InputTokens, u.CacheCreationInputTokens, u.CacheReadInputTokens)
}

func parseClaudeStreamUsageLine(currentEvent string, line string, usage *claudeStreamUsage) {
	if usage == nil || !strings.HasPrefix(line, "data: ") {
		return
	}
	switch currentEvent {
	case "message_start":
		var msgStart struct {
			Message struct {
				Usage struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line[6:]), &msgStart) != nil {
			return
		}
		if msgStart.Message.Usage.InputTokens > 0 {
			usage.InputTokens = msgStart.Message.Usage.InputTokens
		}
		if msgStart.Message.Usage.OutputTokens > 0 {
			usage.OutputTokens = msgStart.Message.Usage.OutputTokens
		}
		if msgStart.Message.Usage.CacheCreationInputTokens > 0 {
			usage.CacheCreationInputTokens = msgStart.Message.Usage.CacheCreationInputTokens
		}
		if msgStart.Message.Usage.CacheReadInputTokens > 0 {
			usage.CacheReadInputTokens = msgStart.Message.Usage.CacheReadInputTokens
		}
	case "message_delta":
		var msgDelta struct {
			Usage struct {
				OutputTokens int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(line[6:]), &msgDelta) != nil {
			return
		}
		if msgDelta.Usage.OutputTokens > 0 {
			usage.OutputTokens = msgDelta.Usage.OutputTokens
		}
	}
}

func (r *Router) forwardClaudeStreamWithFallback(req *http.Request, localKey *models.LocalKey, decision *routing.Decision, requestBytes []byte) (io.ReadCloser, string, string, int, []string, error) {
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

	// Inject stream:true into the raw request bytes for streaming
	streamBytes := injectClaudeStream(requestBytes)

	var lastErr error
	for index, attempt := range attempts {
		if err := ensureKeyAllowed(localKey, attempt, decision.Model); err != nil {
			lastErr = err
			continue
		}
		if err := provider.ValidateFormatCompatibility(attempt.Type, provider.APIFormatClaude); err != nil {
			lastErr = &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "format_error", Code: "format_incompatible", Message: err.Error(), Provider: attempt.Name, Retryable: false}
			if index > 0 {
				fallbackTried = append(fallbackTried, attempt.Name)
			}
			continue
		}
		resp, err := client.ClaudeMessages(req.Context(), attempt, streamBytes)
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

// injectClaudeStream sets stream:true in the raw JSON request while preserving all other fields.
func injectClaudeStream(requestBytes []byte) []byte {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(requestBytes, &m); err != nil {
		return requestBytes
	}
	m["stream"] = json.RawMessage(`true`)
	out, err := json.Marshal(m)
	if err != nil {
		return requestBytes
	}
	return out
}
