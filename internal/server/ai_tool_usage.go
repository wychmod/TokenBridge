package server

import (
	"net/http"
	"strconv"
	"time"
)

func (r *Router) handleAIToolUsage(w http.ResponseWriter, req *http.Request) {
	days := queryInt(req, "days", 30)
	data, err := r.deps.AIToolUsage.Dashboard(req.Context(), days)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) handleAIToolUsageRealtime(w http.ResponseWriter, req *http.Request) {
	data, err := r.deps.AIToolUsage.RealtimeSnapshot(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (r *Router) handleAIToolUsageScan(w http.ResponseWriter, req *http.Request) {
	result, err := r.deps.AIToolUsage.Scan(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) handleAIToolUsageExport(w http.ResponseWriter, req *http.Request) {
	format := req.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	days := queryInt(req, "days", 30)
	exchangeRate := queryFloat(req, "exchange_rate", 7.2)
	body, contentType, filename, err := r.deps.AIToolUsage.Export(req.Context(), format, days, exchangeRate)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("X-Generated-At", time.Now().Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func queryInt(req *http.Request, key string, fallback int) int {
	if raw := req.URL.Query().Get(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			return parsed
		}
	}
	return fallback
}

func queryFloat(req *http.Request, key string, fallback float64) float64 {
	if raw := req.URL.Query().Get(key); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			return parsed
		}
	}
	return fallback
}
