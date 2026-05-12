package server

import (
	"net/http"
	"strconv"
	"time"

	"tokenbridge/internal/requestlog"
)

func (r *Router) handleAdminLogs(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("query")
	traceID := req.URL.Query().Get("trace")
	onlyFallback := req.URL.Query().Get("only_fallback") == "true"
	provider := req.URL.Query().Get("provider")
	apiFormat := req.URL.Query().Get("api_format")
	status := req.URL.Query().Get("status")
	from := req.URL.Query().Get("from")
	to := req.URL.Query().Get("to")
	limit := 50
	if raw := req.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	logs, err := r.deps.RequestLogs.ListWithQuery(req.Context(), requestlog.Query{
		Text:         query,
		TraceID:      traceID,
		OnlyFallback: onlyFallback,
		Provider:     provider,
		APIFormat:    apiFormat,
		Status:       status,
		From:         from,
		To:           to,
		Limit:        limit,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	stats, _ := r.deps.RequestLogs.Stats(req.Context())
	respondJSON(w, http.StatusOK, map[string]any{"data": logs, "stats": stats})
}

func (r *Router) handleAdminLogsExport(w http.ResponseWriter, req *http.Request) {
	query := req.URL.Query().Get("query")
	traceID := req.URL.Query().Get("trace")
	onlyFallback := req.URL.Query().Get("only_fallback") == "true"
	provider := req.URL.Query().Get("provider")
	apiFormat := req.URL.Query().Get("api_format")
	status := req.URL.Query().Get("status")
	from := req.URL.Query().Get("from")
	to := req.URL.Query().Get("to")

	csvData, err := r.deps.RequestLogs.ExportCSV(req.Context(), requestlog.Query{
		Text:         query,
		TraceID:      traceID,
		OnlyFallback: onlyFallback,
		Provider:     provider,
		APIFormat:    apiFormat,
		Status:       status,
		From:         from,
		To:           to,
		Limit:        1000,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	filename := "tokenbridge-logs-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(csvData)
}
