package server

import (
	"net/http"
	"strconv"

	"tokenbridge/internal/pricing"
)

type pricingHandlers struct {
	svc *pricing.Service
}

func newPricingHandlers(svc *pricing.Service) *pricingHandlers {
	return &pricingHandlers{svc: svc}
}

func (h *pricingHandlers) handleRefresh(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	count, err := h.svc.Sync(ctx)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   err.Error(),
			"message": "模型定价同步失败，本地缓存保持不变",
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"models_synced": count,
		"last_sync":     h.svc.LastSync(ctx).Format("2006-01-02T15:04:05Z"),
	})
}

func (h *pricingHandlers) handleStatus(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	lastSync := h.svc.LastSync(ctx)
	lastSyncValue := ""
	if !lastSync.IsZero() {
		lastSyncValue = lastSync.Format("2006-01-02T15:04:05Z")
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"total_models": h.svc.Count(ctx),
		"last_sync":    lastSyncValue,
	})
}

func (h *pricingHandlers) handleLookup(w http.ResponseWriter, req *http.Request) {
	modelID := req.URL.Query().Get("model")
	if modelID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{
			"error": "model 参数不能为空",
		})
		return
	}

	result := h.svc.LookupWithFallback(req.Context(), modelID)
	respondJSON(w, http.StatusOK, map[string]any{
		"data":           result.Pricing,
		"matched":        result.Matched,
		"fallback_used":  result.FallbackUsed,
		"fallback_model": result.FallbackModel,
	})
}

func (h *pricingHandlers) handleList(w http.ResponseWriter, req *http.Request) {
	mode := req.URL.Query().Get("mode")
	ctx := req.Context()
	entries := h.svc.List(ctx, mode)
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  entries,
		"total": len(entries),
	})
}

func (h *pricingHandlers) handleEstimate(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid form"})
		return
	}
	modelID := req.FormValue("model")
	inputTokens, _ := strconv.ParseInt(req.FormValue("input_tokens"), 10, 64)
	outputTokens, _ := strconv.ParseInt(req.FormValue("output_tokens"), 10, 64)
	cacheCreationTokens, _ := strconv.ParseInt(req.FormValue("cache_creation_tokens"), 10, 64)
	cacheReadTokens, _ := strconv.ParseInt(req.FormValue("cache_read_tokens"), 10, 64)
	reasoningTokens, _ := strconv.ParseInt(req.FormValue("reasoning_tokens"), 10, 64)
	contextWindow, _ := strconv.ParseInt(req.FormValue("context_window"), 10, 64)
	pricingTier := req.FormValue("pricing_tier")

	if modelID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]any{"error": "model 参数不能为空"})
		return
	}

	ctx := req.Context()
	lookup := h.svc.LookupWithFallback(ctx, modelID)
	cost := h.svc.CalculateCostDetailed(ctx, pricing.CostInput{
		ModelID:             modelID,
		InputTokens:         inputTokens,
		OutputTokens:        outputTokens,
		CacheCreationTokens: cacheCreationTokens,
		CacheReadTokens:     cacheReadTokens,
		ReasoningTokens:     reasoningTokens,
		ContextWindow:       contextWindow,
		PricingTier:         pricingTier,
	})

	respondJSON(w, http.StatusOK, map[string]any{
		"model_id":              modelID,
		"input_tokens":          inputTokens,
		"output_tokens":         outputTokens,
		"cache_creation_tokens": cacheCreationTokens,
		"cache_read_tokens":     cacheReadTokens,
		"reasoning_tokens":      reasoningTokens,
		"context_window":        cost.ContextWindow,
		"pricing_tier":          cost.PricingTier,
		"estimated_usd":         cost.TotalUSD,
		"cost_breakdown":        cost,
		"pricing_rule_json":     cost.PricingRuleJSON,
		"pricing":               lookup.Pricing,
		"matched":               lookup.Matched,
		"fallback_used":         lookup.FallbackUsed,
		"fallback_model":        lookup.FallbackModel,
	})
}
