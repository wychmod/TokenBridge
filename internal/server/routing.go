package server

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"tokenbridge/internal/routing"
)

type routingRuleRequest struct {
	ModelPattern  string   `json:"model_pattern"`
	Strategy      string   `json:"strategy"`
	ProviderChain []string `json:"provider_chain"`
	FallbackChain []string `json:"fallback_chain"`
	Enabled       bool     `json:"enabled"`
}

type aliasRequest struct {
	Alias         string   `json:"alias"`
	Target        string   `json:"target"`
	FallbackChain []string `json:"fallback_chain"`
}

type routingTestRequest struct {
	Model     string `json:"model"`
	LocalKey  string `json:"local_key"`
	Format    string `json:"format"`
	Streaming bool   `json:"streaming"`
}

func (r *Router) handleRoutingRules(w http.ResponseWriter, req *http.Request) {
	items, err := r.deps.Routing.ListRules(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) handleCreateRoutingRule(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeRoutingRule(w, req)
	if !ok {
		return
	}
	created, err := r.deps.Routing.CreateRule(req.Context(), input)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"data": created})
}

func (r *Router) handleUpdateRoutingRule(w http.ResponseWriter, req *http.Request) {
	input, ok := decodeRoutingRule(w, req)
	if !ok {
		return
	}
	updated, err := r.deps.Routing.UpdateRule(req.Context(), chi.URLParam(req, "id"), input)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": updated})
}

func (r *Router) handleDeleteRoutingRule(w http.ResponseWriter, req *http.Request) {
	if err := r.deps.Routing.DeleteRule(req.Context(), chi.URLParam(req, "id")); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (r *Router) handleRoutingTest(w http.ResponseWriter, req *http.Request) {
	var payload routingTestRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := r.deps.Routing.Simulate(req.Context(), routing.TestInput(payload))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (r *Router) handleModelAliases(w http.ResponseWriter, req *http.Request) {
	items, err := r.deps.Routing.ListAliases(req.Context())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (r *Router) handleUpsertModelAlias(w http.ResponseWriter, req *http.Request) {
	var payload aliasRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := r.deps.Routing.UpsertAlias(req.Context(), routing.AliasInput(payload))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func decodeRoutingRule(w http.ResponseWriter, req *http.Request) (routing.RuleInput, bool) {
	var payload routingRuleRequest
	if err := decodeJSON(req, &payload); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return routing.RuleInput{}, false
	}
	return routing.RuleInput(payload), true
}
