package server

import (
	"net/http"
	"strconv"

	"tokenbridge/internal/admin"
	"tokenbridge/internal/aitoolusage"
)

type dashboardResponse struct {
	admin.DashboardData
	AIToolUsage *aitoolusage.Summary `json:"ai_tool_usage,omitempty"`
}

func (r *Router) handleAdminOverview(w http.ResponseWriter, req *http.Request) {
	overview, err := r.deps.Admin.Overview(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": overview})
}

func (r *Router) handleAdminDashboard(w http.ResponseWriter, req *http.Request) {
	data, err := r.deps.Admin.Dashboard(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	response := dashboardResponse{DashboardData: data}
	if r.deps.AIToolUsage != nil {
		aiToolUsage, err := r.deps.AIToolUsage.Dashboard(req.Context(), 30)
		if err != nil {
			r.deps.Logger.Warn().Err(err).Msg("admin dashboard: ai tool usage summary unavailable")
		} else {
			response.AIToolUsage = &aiToolUsage.Summary
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": response})
}

func (r *Router) handleAdminAnalytics(w http.ResponseWriter, req *http.Request) {
	days := 7
	if raw := req.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}
	data, err := r.deps.Admin.Analytics(req.Context(), days)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": data})
}
