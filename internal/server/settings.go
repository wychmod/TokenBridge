package server

import (
	"net/http"
	"tokenbridge/internal/settings"
)

func (r *Router) handleGetSettings(w http.ResponseWriter, req *http.Request) {
	value, err := r.deps.Settings.Get(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": value})
}

func (r *Router) handleSaveSettings(w http.ResponseWriter, req *http.Request) {
	var payload settings.AppSettings
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	saved, err := r.deps.Settings.Save(req.Context(), payload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": saved})
}

func (r *Router) handleBackupSettings(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"data": r.deps.Settings.Backup(req.Context())})
}

func (r *Router) handleDistributionPlan(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"data": r.deps.Settings.DistributionPlan()})
}
