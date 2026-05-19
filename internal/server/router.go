package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"tokenbridge/internal/admin"
	"tokenbridge/internal/aitoolusage"
	"tokenbridge/internal/auth"
	"tokenbridge/internal/config"
	"tokenbridge/internal/pricing"
	"tokenbridge/internal/provider"
	"tokenbridge/internal/requestlog"
	"tokenbridge/internal/routing"
	"tokenbridge/internal/settings"
	"tokenbridge/internal/usage"

	"tokenbridge/build/embed"
)

type Dependencies struct {
	Config      config.Config
	Logger      zerolog.Logger
	Providers   *provider.Service
	Keys        *auth.Service
	Routing     *routing.Service
	Usage       *usage.Service
	Pricing     *pricing.Service
	Settings    *settings.Service
	Admin       *admin.Service
	RequestLogs *requestlog.Service
	AIToolUsage *aitoolusage.Service
	DB          *gorm.DB
}

type Router struct {
	mux  chi.Router
	deps Dependencies
}

func NewRouter(deps Dependencies) *Router {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.Config.Security.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Api-Key", "anthropic-version"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router := &Router{mux: r, deps: deps}
	router.mount()
	return router
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) mount() {
	r.mux.Get("/health", r.handleHealth)
	r.mux.Get("/v1/models", r.handleModels)
	r.mux.Post("/v1/chat/completions", r.handleChatCompletions)
	r.mux.Post("/v1/messages", r.handleClaudeMessages)

	pricingH := newPricingHandlers(r.deps.Pricing)

	r.mux.Route("/admin/api", func(adminRouter chi.Router) {
		adminRouter.Get("/logs/export", r.handleAdminLogsExport)
		adminRouter.Get("/dashboard", r.handleAdminDashboard)
		adminRouter.Get("/analytics", r.handleAdminAnalytics)
		adminRouter.Get("/logs", r.handleAdminLogs)
		adminRouter.Get("/overview", r.handleAdminOverview)
		adminRouter.Get("/ai-tool-usage", r.handleAIToolUsage)
		adminRouter.Get("/ai-tool-usage/realtime", r.handleAIToolUsageRealtime)
		adminRouter.Post("/ai-tool-usage/scan", r.handleAIToolUsageScan)
		adminRouter.Get("/ai-tool-usage/export", r.handleAIToolUsageExport)

		adminRouter.Get("/providers", r.handleProviders)
		adminRouter.Get("/providers/types", r.handleProviderTypes)
		adminRouter.Post("/providers", r.handleCreateProvider)
		adminRouter.Post("/providers/test", r.handleTestProviderDraft)
		adminRouter.Post("/providers/discover-models", r.handleDiscoverModelsDraft)
		adminRouter.Put("/providers/{id}", r.handleUpdateProvider)
		adminRouter.Delete("/providers/{id}", r.handleDeleteProvider)
		adminRouter.Post("/providers/{id}/test", r.handleTestProvider)
		adminRouter.Post("/providers/{id}/discover-models", r.handleDiscoverModels)
		adminRouter.Post("/providers/reorder", r.handleReorderProviders)

		adminRouter.Get("/keys", r.handleKeys)
		adminRouter.Post("/keys", r.handleCreateKey)
		adminRouter.Put("/keys/{id}", r.handleUpdateKey)
		adminRouter.Post("/keys/{id}/revoke", r.handleRevokeKey)
		adminRouter.Post("/keys/{id}/rotate", r.handleRotateKey)
		adminRouter.Post("/keys/{id}/extend", r.handleExtendKey)

		adminRouter.Get("/routing", r.handleRoutingRules)
		adminRouter.Post("/routing", r.handleCreateRoutingRule)
		adminRouter.Put("/routing/{id}", r.handleUpdateRoutingRule)
		adminRouter.Delete("/routing/{id}", r.handleDeleteRoutingRule)
		adminRouter.Post("/routing/test", r.handleRoutingTest)
		adminRouter.Get("/aliases", r.handleModelAliases)
		adminRouter.Put("/aliases", r.handleUpsertModelAlias)

		adminRouter.Get("/settings", r.handleGetSettings)
		adminRouter.Put("/settings", r.handleSaveSettings)
		adminRouter.Post("/settings/backup", r.handleBackupSettings)
		adminRouter.Get("/distribution", r.handleDistributionPlan)

		// Model pricing endpoints
		adminRouter.Post("/pricing/refresh", pricingH.handleRefresh)
		adminRouter.Get("/pricing/status", pricingH.handleStatus)
		adminRouter.Get("/pricing/lookup", pricingH.handleLookup)
		adminRouter.Get("/pricing/list", pricingH.handleList)
		adminRouter.Get("/pricing/estimate", pricingH.handleEstimate)
	})

	adminAssets := http.FileServer(embed.AdminFS())
	r.mux.Handle("/assets/*", adminAssets)
	r.mux.Handle("/admin/assets/*", http.StripPrefix("/admin", adminAssets))

	// Serve embedded admin UI on /admin with SPA fallback.
	// API routes (/admin/api/*) are registered above and take priority over this static handler.
	r.mux.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	r.mux.MethodFunc("GET", "/admin/*", r.serveAdminUI)
}

func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// serveAdminUI serves embedded admin UI with SPA fallback.
// For non-file requests (like /admin/providers), it serves index.html
// so client-side React Router can handle the route.
func (r *Router) serveAdminUI(w http.ResponseWriter, req *http.Request) {
	// Strip /admin prefix from path
	path := chi.URLParam(req, "*")
	if path == "" {
		path = "/"
	}

	// Try to open the file from embed FS
	f, err := embed.AdminFS().Open(path)
	if err != nil {
		// File not found — serve index.html for SPA routing
		r.serveAdminIndex(w)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		r.serveAdminIndex(w)
		return
	}

	http.ServeContent(w, req, stat.Name(), stat.ModTime(), f)
}

// serveAdminIndex serves the index.html from the embedded admin assets.
func (r *Router) serveAdminIndex(w http.ResponseWriter) {
	f, err := embed.AdminFS().Open("/index.html")
	if err != nil {
		http.Error(w, "admin UI not available", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, &http.Request{URL: &url.URL{Path: "/admin/index.html"}}, "index.html", stat.ModTime(), f)
}
