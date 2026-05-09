package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"localgateway/internal/provider"
)

type createProviderRequest struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	BaseURL        string   `json:"base_url"`
	APIKey         string   `json:"api_key"`
	OrganizationID string   `json:"organization_id"`
	Enabled        bool     `json:"enabled"`
	Priority       int      `json:"priority"`
	Status         string   `json:"status"`
	Models         []string `json:"models"`
	RateLimitRPM   int      `json:"rate_limit_rpm"`
	RateLimitTPM   int      `json:"rate_limit_tpm"`
}

type reorderProvidersRequest struct {
	IDs []string `json:"ids"`
}

func (r *Router) handleProviders(w http.ResponseWriter, req *http.Request) {
	items, err := r.deps.Providers.List(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) handleCreateProvider(w http.ResponseWriter, req *http.Request) {
	payload, ok := decodeProviderPayload(w, req)
	if !ok {
		return
	}
	// Validate type early for better HTTP status
	if err := provider.ValidateType(payload.Type); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	created, err := r.deps.Providers.Create(req.Context(), payload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (r *Router) handleUpdateProvider(w http.ResponseWriter, req *http.Request) {
	payload, ok := decodeProviderPayload(w, req)
	if !ok {
		return
	}
	// Validate type early for better HTTP status
	if err := provider.ValidateType(payload.Type); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updated, err := r.deps.Providers.Update(req.Context(), chi.URLParam(req, "id"), payload)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (r *Router) handleDeleteProvider(w http.ResponseWriter, req *http.Request) {
	if err := r.deps.Providers.Delete(req.Context(), chi.URLParam(req, "id")); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (r *Router) handleTestProvider(w http.ResponseWriter, req *http.Request) {
	result, err := r.deps.Providers.TestConnection(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) handleDiscoverModels(w http.ResponseWriter, req *http.Request) {
	models, err := r.deps.Providers.DiscoverModels(req.Context(), chi.URLParam(req, "id"))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": models})
}

func (r *Router) handleReorderProviders(w http.ResponseWriter, req *http.Request) {
	var payload reorderProvidersRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := r.deps.Providers.Reorder(req.Context(), payload.IDs); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (r *Router) handleProviderTypes(w http.ResponseWriter, req *http.Request) {
	typeInfo := make([]map[string]any, 0, len(provider.KnownProviderTypes))
	for _, t := range provider.KnownProviderTypes {
		entry := map[string]any{
			"value": t,
			"label": providerTypeLabel(t),
		}
		if defaults, ok := provider.GetDefaults(t); ok && defaults.BaseURL != "" {
			entry["default_base_url"] = defaults.BaseURL
		}
		typeInfo = append(typeInfo, entry)
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": typeInfo})
}

func providerTypeLabel(t string) string {
	switch t {
	case provider.TypeOpenAI:
		return "OpenAI"
	case provider.TypeOpenAICompatible:
		return "OpenAI 兼容"
	case provider.TypeAnthropic:
		return "Anthropic (Claude)"
	default:
		return t
	}
}

func decodeProviderPayload(w http.ResponseWriter, req *http.Request) (provider.ProviderInput, bool) {
	var payload createProviderRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return provider.ProviderInput{}, false
	}
	return provider.ProviderInput{
		Name:           payload.Name,
		Type:           payload.Type,
		BaseURL:        payload.BaseURL,
		APIKey:         payload.APIKey,
		OrganizationID: payload.OrganizationID,
		Enabled:        payload.Enabled,
		Priority:       payload.Priority,
		Status:         payload.Status,
		Models:         payload.Models,
		RateLimitRPM:   payload.RateLimitRPM,
		RateLimitTPM:   payload.RateLimitTPM,
	}, true
}
