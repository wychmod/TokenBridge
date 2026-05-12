package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"tokenbridge/internal/auth"
)

type createKeyRequest struct {
	Name             string     `json:"name"`
	AllowedModels    []string   `json:"allowed_models"`
	AllowedProviders []string   `json:"allowed_providers"`
	MonthlyBudget    float64    `json:"monthly_budget"`
	TokenBudget      int64      `json:"token_budget"`
	Enabled          bool       `json:"enabled"`
	ExpiresAt        *time.Time `json:"expires_at"`
}

type extendKeyRequest struct {
	ExpiresAt *time.Time `json:"expires_at"`
}

func (r *Router) handleKeys(w http.ResponseWriter, req *http.Request) {
	items, err := r.deps.Keys.List(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) handleCreateKey(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeKeyPayload(w, req)
	if !ok {
		return
	}
	entity, rawKey, err := r.deps.Keys.Create(req.Context(), input)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"data": entity, "raw_key": rawKey})
}

func (r *Router) handleUpdateKey(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeUpdateKeyPayload(w, req)
	if !ok {
		return
	}
	updated, err := r.deps.Keys.Update(req.Context(), chi.URLParam(req, "id"), input)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (r *Router) handleRevokeKey(w http.ResponseWriter, req *http.Request) {
	if err := r.deps.Keys.Revoke(req.Context(), chi.URLParam(req, "id")); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (r *Router) handleRotateKey(w http.ResponseWriter, req *http.Request) {
	updated, rawKey, err := r.deps.Keys.Rotate(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": updated, "raw_key": rawKey})
}

func (r *Router) handleExtendKey(w http.ResponseWriter, req *http.Request) {
	var payload extendKeyRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updated, err := r.deps.Keys.Extend(req.Context(), chi.URLParam(req, "id"), payload.ExpiresAt)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func decodeKeyPayload(w http.ResponseWriter, req *http.Request) (auth.CreateKeyInput, bool) {
	var payload createKeyRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return auth.CreateKeyInput{}, false
	}
	return auth.CreateKeyInput{
		Name:             payload.Name,
		AllowedModels:    payload.AllowedModels,
		AllowedProviders: payload.AllowedProviders,
		MonthlyBudget:    payload.MonthlyBudget,
		TokenBudget:      payload.TokenBudget,
		Enabled:          payload.Enabled,
		ExpiresAt:        payload.ExpiresAt,
	}, true
}

func decodeUpdateKeyPayload(w http.ResponseWriter, req *http.Request) (auth.UpdateKeyInput, bool) {
	var payload createKeyRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return auth.UpdateKeyInput{}, false
	}
	return auth.UpdateKeyInput{
		Name:             payload.Name,
		AllowedModels:    payload.AllowedModels,
		AllowedProviders: payload.AllowedProviders,
		MonthlyBudget:    payload.MonthlyBudget,
		TokenBudget:      payload.TokenBudget,
		Enabled:          payload.Enabled,
		ExpiresAt:        payload.ExpiresAt,
	}, true
}
