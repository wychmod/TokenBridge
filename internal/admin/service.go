package admin

import (
	"context"

	"localgateway/internal/auth"
	"localgateway/internal/pricing"
	"localgateway/internal/provider"
	"localgateway/internal/requestlog"
	"localgateway/internal/routing"
	"localgateway/internal/settings"
	"localgateway/internal/usage"
)

type Overview struct {
	Providers     int                  `json:"providers"`
	Keys          int                  `json:"keys"`
	Usage         usage.Summary        `json:"usage"`
	Settings      settings.AppSettings `json:"settings"`
	Rules         int                  `json:"rules"`
	PricingModels int                  `json:"pricing_models"`
}

type DashboardData struct {
	Overview          Overview                     `json:"overview"`
	Trend             []usage.TrendPoint           `json:"trend"`
	FailureTrend      []requestlog.TrendPoint      `json:"failure_trend"`
	ProviderBreakdown []usage.ProviderBreakdown    `json:"provider_breakdown"`
	ProviderHealth    []provider.HealthCheckResult `json:"provider_health"`
	RecentLogs        []requestlog.Item            `json:"recent_logs"`
	HotModels         []usage.ModelBreakdown       `json:"hot_models"`
	ProviderTop       []usage.ProviderBreakdown    `json:"provider_top"`
	LogStats          requestlog.Stats             `json:"log_stats"`
	Alerts            []map[string]string          `json:"alerts"`
}

type AnalyticsData struct {
	Summary           usage.Summary             `json:"summary"`
	Trend             []usage.TrendPoint        `json:"trend"`
	ProviderBreakdown []usage.ProviderBreakdown `json:"provider_breakdown"`
	ModelBreakdown    []usage.ModelBreakdown    `json:"model_breakdown"`
	KeyBreakdown      []usage.KeyBreakdown      `json:"key_breakdown"`
	LogStats          requestlog.Stats          `json:"log_stats"`
}

type Service struct {
	providers   *provider.Service
	keys        *auth.Service
	usage       *usage.Service
	pricing     *pricing.Service
	routing     *routing.Service
	settings    *settings.Service
	requestLogs *requestlog.Service
}

func NewService(providers *provider.Service, keys *auth.Service, usageService *usage.Service, pricingService *pricing.Service, routingService *routing.Service, settingsService *settings.Service, requestLogs *requestlog.Service) *Service {
	return &Service{providers: providers, keys: keys, usage: usageService, pricing: pricingService, routing: routingService, settings: settingsService, requestLogs: requestLogs}
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	providers, err := s.providers.List(ctx)
	if err != nil {
		return Overview{}, err
	}
	keys, err := s.keys.List(ctx)
	if err != nil {
		return Overview{}, err
	}
	usageSummary, err := s.usage.Summary(ctx)
	if err != nil {
		return Overview{}, err
	}
	rules, err := s.routing.ListRules(ctx)
	if err != nil {
		return Overview{}, err
	}
	appSettings, err := s.settings.Get(ctx)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Providers: len(providers), Keys: len(keys), Usage: usageSummary, Settings: appSettings, Rules: len(rules), PricingModels: s.pricing.Count(ctx)}, nil
}

func (s *Service) Dashboard(ctx context.Context) (DashboardData, error) {
	overview, err := s.Overview(ctx)
	if err != nil {
		return DashboardData{}, err
	}
	trend, err := s.usage.Trend(ctx, 7)
	if err != nil {
		return DashboardData{}, err
	}
	failureTrend, err := s.requestLogs.Trend(ctx, 7)
	if err != nil {
		return DashboardData{}, err
	}
	breakdown, err := s.usage.ProviderBreakdown(ctx)
	if err != nil {
		return DashboardData{}, err
	}
	hotModels, err := s.usage.ModelBreakdown(ctx)
	if err != nil {
		return DashboardData{}, err
	}
	recentLogs, err := s.requestLogs.List(ctx, "", false, 10)
	if err != nil {
		return DashboardData{}, err
	}
	logStats, err := s.requestLogs.Stats(ctx)
	if err != nil {
		return DashboardData{}, err
	}
	providerHealth := make([]provider.HealthCheckResult, 0)
	providers, err := s.providers.List(ctx)
	if err == nil {
		for _, item := range providers {
			providerHealth = append(providerHealth, provider.HealthCheckResult{Status: item.Status, LatencyMS: 0, Models: nil, Message: item.Name})
		}
	}
	alerts := []map[string]string{}
	for _, item := range recentLogs {
		if item.FallbackUsed {
			alerts = append(alerts, map[string]string{"level": "warning", "title": "发生备用切换", "description": item.Detail})
		}
		if item.ErrorMessage != "" {
			alerts = append(alerts, map[string]string{"level": "critical", "title": "请求失败", "description": item.ErrorMessage})
		}
	}
	providerTop := breakdown
	if len(providerTop) > 5 {
		providerTop = providerTop[:5]
	}
	if len(hotModels) > 5 {
		hotModels = hotModels[:5]
	}
	return DashboardData{Overview: overview, Trend: trend, FailureTrend: failureTrend, ProviderBreakdown: breakdown, ProviderHealth: providerHealth, RecentLogs: recentLogs, HotModels: hotModels, ProviderTop: providerTop, LogStats: logStats, Alerts: alerts}, nil
}

func (s *Service) Analytics(ctx context.Context, days int) (AnalyticsData, error) {
	summary, err := s.usage.Summary(ctx)
	if err != nil {
		return AnalyticsData{}, err
	}
	trend, err := s.usage.Trend(ctx, days)
	if err != nil {
		return AnalyticsData{}, err
	}
	breakdown, err := s.usage.ProviderBreakdown(ctx)
	if err != nil {
		return AnalyticsData{}, err
	}
	modelBreakdown, err := s.usage.ModelBreakdown(ctx)
	if err != nil {
		return AnalyticsData{}, err
	}
	keyBreakdown, err := s.usage.KeyBreakdown(ctx)
	if err != nil {
		return AnalyticsData{}, err
	}
	logStats, err := s.requestLogs.Stats(ctx)
	if err != nil {
		return AnalyticsData{}, err
	}
	return AnalyticsData{Summary: summary, Trend: trend, ProviderBreakdown: breakdown, ModelBreakdown: modelBreakdown, KeyBreakdown: keyBreakdown, LogStats: logStats}, nil
}
