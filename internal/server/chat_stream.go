package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"tokenbridge/internal/provider"
)

func (r *Router) handleChatCompletionsStream(w http.ResponseWriter, req *http.Request, requestBytes []byte, meta chatCompletionMeta) {
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
	if err := provider.ValidateFormatCompatibility(decision.Provider.Type, provider.APIFormatOpenAI); err != nil {
		writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "format_error", Code: "format_incompatible", Message: err.Error(), Provider: decision.Provider.Name, Retryable: false})
		return
	}
	if err := ensureKeyAllowed(localKey, decision.Provider, decision.Model); err != nil {
		writeGatewayError(w, err)
		return
	}

	// Inject stream_options so upstream includes usage in the final SSE chunk
	requestBytes = injectStreamOptions(requestBytes)

	trace := newRequestTrace(decision.Provider.Name, meta.Model, decision.Model, "openai_stream")
	client := newOpenAIClient(r.deps.Config.Proxy.StreamTimeout)
	startedAt := time.Now()
	resp, err := client.ChatCompletions(req.Context(), decision.Provider, requestBytes)
	if err != nil {
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/chat/completions", req.Method, http.StatusBadGateway, time.Since(startedAt).Milliseconds(), err.Error(), trace)
		writeGatewayError(w, err)
		return
	}

	if resp.StatusCode >= 400 {
		responseBytes, readErr := readBodyAndClose(resp.Body)
		if readErr != nil {
			writeGatewayError(w, &gatewayError{HTTPStatus: http.StatusBadGateway, Type: "provider_error", Code: "upstream_read_failed", Message: readErr.Error(), Provider: decision.Provider.Name, Retryable: true})
			return
		}
		gwErr := mapUpstreamError(resp.StatusCode, responseBytes, decision.Provider.Name)
		logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/chat/completions", req.Method, resp.StatusCode, time.Since(startedAt).Milliseconds(), gwErr.Error(), trace)
		writeGatewayError(w, gwErr)
		return
	}
	defer resp.Body.Close()

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

	// Track usage from the final chunk
	var streamUsage openAIChatResponse

	reader := bufio.NewReader(resp.Body)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = w.Write(line)
			flusher.Flush()

			// Try to extract usage from each SSE data line
			lineStr := strings.TrimSpace(string(line))
			if strings.HasPrefix(lineStr, "data: ") && lineStr != "data: [DONE]" {
				var chunk openAIChatResponse
				if json.Unmarshal([]byte(lineStr[6:]), &chunk) == nil {
					if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
						streamUsage = chunk
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	// Record usage if we got it from the stream
	if streamUsage.Usage.PromptTokens > 0 || streamUsage.Usage.CompletionTokens > 0 {
		recordUsageBestEffort(req.Context(), r.deps.Usage, r.deps.Pricing, r.deps.Keys, localKey.ID, decision.Provider.ID, meta.Model, decision.Model, "openai_stream", time.Since(startedAt).Milliseconds(), true, streamUsage)
	}
	logRequestBestEffort(req.Context(), r.deps.DB, localKey.ID, decision.Provider.ID, "/v1/chat/completions", req.Method, http.StatusOK, time.Since(startedAt).Milliseconds(), "", trace)
}
