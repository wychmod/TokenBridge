import { Area, AreaChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, CheckCircle2, Loader2, Zap } from "lucide-react";
import { isDesktopMode } from "../utils/desktop-bridge";
import { api } from "../utils/api";

type DashboardPayload = {
  overview: {
    providers: number;
    keys: number;
    rules: number;
    usage: {
      total_requests: number;
      total_cost_usd: number;
      input_tokens: number;
      output_tokens: number;
      success_rate: number;
    };
  };
  trend: Array<{ day: string; cost: number; requests: number; tokens: number }>;
  failure_trend: Array<{ day: string; failures: number; fallbacks: number }>;
  provider_health: Array<{ status: string; latency_ms: number; message: string }>;
  recent_logs: Array<{ id: string; path: string; provider_id: string; latency_ms: number; status_label: string; status_code: number }>;
  log_stats: { total: number; failures: number; fallbacks: number; avg_latency_ms: number };
};

export function DashboardPage() {
  const [data, setData] = useState<DashboardPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api.get<DashboardPayload>("/dashboard")
      .then((result) => {
        if (!result.ok || !result.data) {
          throw new Error(result.error ?? `加载失败`);
        }
        setData(result.data);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => setLoading(false));
  }, []);

  const kpis = useMemo(() => {
    if (!data) return [];
    const healthyProviders = data.provider_health?.filter((provider) => provider.status === "healthy").length ?? 0;
    return [
      {
        label: "总请求",
        value: data.overview.usage.total_requests.toLocaleString(),
        delta: "所有接入工具的网关调用",
        healthy: data.overview.usage.success_rate >= 0.95
      },
      {
        label: "成功率",
        value: `${(data.overview.usage.success_rate * 100).toFixed(1)}%`,
        delta: data.overview.usage.success_rate >= 0.95 ? "当前链路稳定" : "建议查看失败日志",
        healthy: data.overview.usage.success_rate >= 0.95
      },
      {
        label: "累计费用（7日）",
        value: `$${data.overview.usage.total_cost_usd.toFixed(2)}`,
        delta: `${(data.overview.usage.input_tokens + data.overview.usage.output_tokens).toLocaleString()} tokens`,
        healthy: true
      },
      {
        label: "失败请求",
        value: String(data.log_stats.failures),
        delta: `备用切换 ${data.log_stats.fallbacks} 次`,
        healthy: data.log_stats.failures === 0
      },
      {
        label: "Fallback",
        value: String(data.log_stats.fallbacks),
        delta: data.log_stats.fallbacks > 0 ? "主链路曾切到备用线路" : "未触发备用切换",
        healthy: data.log_stats.fallbacks === 0
      },
      {
        label: "平均延迟",
        value: `${data.log_stats.avg_latency_ms}ms`,
        delta: `${healthyProviders}/${data.overview.providers} 家 Provider 健康`,
        healthy: data.log_stats.avg_latency_ms < 1000
      }
    ];
  }, [data]);

  const alerts = useMemo(() => {
    const items: Array<{ level: "success" | "warning" | "danger"; title: string; message: string }> = [];
    if (!data) return items;

    if (data.log_stats.failures > 0) {
      items.push({
        level: "warning",
        title: "近期存在失败请求",
        message: `过去 7 天内共有 ${data.log_stats.failures} 次请求失败，${data.log_stats.fallbacks} 次触发备用切换。`
      });
    }

    const unhealthyProviders = data.provider_health?.filter((p) => p.status !== "healthy");
    if (unhealthyProviders?.length) {
      items.push({
        level: "danger",
        title: `${unhealthyProviders.length} 家厂商异常`,
        message: unhealthyProviders.map((p) => p.message).join("；")
      });
    }

    const recentFailures = data.recent_logs?.filter((log) => log.status_code >= 400);
    if (recentFailures?.length > 0) {
      items.push({
        level: "warning",
        title: "最近请求存在异常",
        message: `最近 ${recentFailures.length} 条请求返回非 2xx 状态码。`
      });
    }

    if (items.length === 0) {
      items.push({
        level: "success",
        title: "系统运行平稳",
        message: "过去 7 天内未发现需要关注的异常。"
      });
    }

    return items;
  }, [data]);

  const trafficDots = useMemo(() => {
    if (!data?.recent_logs?.length) return [];
    return data.recent_logs.slice(0, 8).map((log) => ({
      id: log.id,
      color: log.status_code >= 500 ? "var(--danger)" : log.status_code >= 400 ? "var(--warning)" : "var(--accent)",
      path: log.path,
      provider: log.provider_id,
      latency: log.latency_ms
    }));
  }, [data]);

  if (loading) {
    return (
      <div className="flex-col gap-5">
        <section className="kpi-grid">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="kpi-card" style={{ minHeight: 100 }}>
              <div className="skeleton skeleton-text" style={{ width: "40%" }} />
              <div className="skeleton skeleton-title" style={{ width: "70%", marginTop: 8 }} />
              <div className="skeleton skeleton-text" style={{ width: "50%", marginTop: 8 }} />
            </div>
          ))}
        </section>
        <div className="dashboard-grid">
          <div className="panel" style={{ minHeight: 340 }}>
            <div className="skeleton skeleton-title" style={{ width: "30%" }} />
            <div className="skeleton" style={{ width: "100%", height: 260, marginTop: 16 }} />
          </div>
          <div className="panel" style={{ minHeight: 340 }}>
            <div className="skeleton skeleton-title" style={{ width: "40%" }} />
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="panel panel-compact" style={{ marginTop: 12 }}>
                <div className="skeleton skeleton-text" style={{ width: "60%" }} />
                <div className="skeleton skeleton-text" style={{ width: "80%", marginTop: 8 }} />
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="empty-state" style={{ minHeight: 400 }}>
        <Zap size={32} style={{ color: "var(--danger)", opacity: 0.6 }} />
        <span className="empty-state-title" style={{ color: "var(--danger)" }}>数据加载失败</span>
        <span className="empty-state-desc">{error}，请检查后端服务是否正常运行。</span>
        <button type="button" className="btn btn-primary btn-sm" onClick={() => window.location.reload()}>重新加载</button>
      </div>
    );
  }

  return (
    <div className="flex-col gap-5">
      {/* KPI Row */}
      <section className="kpi-grid">
        {kpis.length > 0 ? (
          kpis.map((kpi) => (
            <article key={kpi.label} className="kpi-card">
              <span className="kpi-label">{kpi.label}</span>
              <span className="kpi-value" style={{ color: kpi.healthy ? "var(--text-primary)" : "var(--warning)" }}>
                {kpi.value}
              </span>
              <span className="kpi-delta">{kpi.delta}</span>
            </article>
          ))
        ) : (
          <article className="kpi-card" style={{ gridColumn: "span 4" }}>
            <span className="kpi-label">系统初始化中</span>
            <span className="kpi-value">--</span>
            <span className="kpi-delta">发起真实请求后将显示统计数据</span>
          </article>
        )}
      </section>

      {/* Charts + Alerts */}
      <section className="dashboard-grid">
        {/* Cost Trend */}
        <article className="panel">
          <div className="section-header" style={{ marginBottom: "var(--space-4)" }}>
            <div className="section-header-main">
              <span className="eyebrow">用量监控</span>
              <h2 className="section-title">请求与费用趋势</h2>
              <p className="section-description">同时查看请求量和成本，判断“用得多”还是“单次变贵”。</p>
            </div>
          </div>
          <div className="chart-container">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={data?.trend ?? []}>
                <defs>
                  <linearGradient id="costFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stopColor="var(--accent)" stopOpacity={0.25} />
                    <stop offset="100%" stopColor="var(--accent)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
                <XAxis dataKey="day" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} />
                <YAxis yAxisId="cost" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} width={44} />
                <YAxis yAxisId="requests" orientation="right" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} width={44} />
                <Tooltip
                  contentStyle={{
                    background: "var(--bg-surface)",
                    border: "1px solid var(--border-default)",
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.82rem",
                    boxShadow: "var(--shadow-md)"
                  }}
                />
                <Legend verticalAlign="top" height={28} />
                <Area
                  yAxisId="cost"
                  type="monotone"
                  dataKey="cost"
                  name="费用 USD"
                  stroke="var(--accent)"
                  fill="url(#costFill)"
                  strokeWidth={2}
                />
                <Area
                  yAxisId="requests"
                  type="monotone"
                  dataKey="requests"
                  name="请求数"
                  stroke="var(--info)"
                  fill="transparent"
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
          <p className="chart-note">图表提供 tooltip、图例和双轴单位；成本按美元展示，请求按次数展示。</p>
        </article>

        {/* Alerts */}
        <article className="panel" style={{ display: "flex", flexDirection: "column", gap: "var(--space-3)" }}>
          <div>
            <span className="eyebrow">异常监控</span>
            <h2 className="section-title" style={{ marginTop: 4 }}>最近 Alert</h2>
          </div>
          <div className="flex-col gap-2" style={{ flex: 1, overflow: "auto" }}>
            {alerts.map((alert, i) => (
              <div
                key={i}
                className={`alert-card ${alert.level}`}
                style={{ animationDelay: `${i * 50}ms` }}
              >
                <div className="flex items-center gap-2" style={{ marginBottom: 4 }}>
                  {alert.level === "success" ? (
                    <CheckCircle2 size={14} style={{ color: "var(--accent)" }} />
                  ) : alert.level === "warning" ? (
                    <AlertTriangle size={14} style={{ color: "var(--warning)" }} />
                  ) : (
                    <Zap size={14} style={{ color: "var(--danger)" }} />
                  )}
                  <strong style={{ fontSize: "0.85rem", color: "var(--text-primary)" }}>{alert.title}</strong>
                </div>
                <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", lineHeight: 1.4 }}>{alert.message}</p>
              </div>
            ))}
          </div>
        </article>
      </section>

      <section className="page-summary-grid">
        {(data?.provider_health ?? []).map((provider, index) => (
          <article key={`${provider.message}-${index}`} className="summary-card">
            <div className="flex items-center gap-2">
              <span className={`status-dot ${providerTone(provider.status)}`} />
              <strong>Provider #{index + 1}</strong>
              <span className={`pill pill-${providerTone(provider.status)}`}>{providerStatusText(provider.status)}</span>
            </div>
            <span>{provider.latency_ms ? `${provider.latency_ms}ms` : "暂无延迟"} · {provider.message || "未返回健康说明"}</span>
          </article>
        ))}
      </section>

      {/* Mini Traffic Bar */}
      <section className="panel panel-compact">
        <div className="flex items-center justify-between" style={{ marginBottom: "var(--space-3)" }}>
          <div>
            <span className="eyebrow">实时链路</span>
            <span style={{ fontSize: "0.82rem", color: "var(--text-secondary)", marginLeft: 8 }}>
              最近 {trafficDots.length} 条请求
            </span>
          </div>
        </div>
        {trafficDots.length > 0 ? (
          <div className="flex items-center gap-3">
            {trafficDots.map((dot) => (
              <div
                key={dot.id}
                className="flex-col items-center gap-1"
                style={{ cursor: "pointer" }}
                title={`${dot.path}\n厂商: ${dot.provider}\n延迟: ${dot.latency}ms`}
                aria-label={`${dot.path}，厂商 ${dot.provider}，延迟 ${dot.latency}ms`}
              >
                <div
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    background: dot.color,
                    transition: "transform 150ms ease"
                  }}
                />
                <span className="data-value" style={{ fontSize: "0.65rem", color: "var(--text-tertiary)" }}>
                  {dot.latency}ms
                </span>
              </div>
            ))}
          </div>
        ) : (
          <span style={{ fontSize: "0.82rem", color: "var(--text-tertiary)" }}>暂无最近请求数据</span>
        )}
      </section>
    </div>
  );
}

function providerTone(status: string): "success" | "warning" | "danger" | "neutral" {
  if (status === "healthy") return "success";
  if (status === "warning") return "warning";
  if (status === "disabled" || status === "blocked") return "danger";
  return "neutral";
}

function providerStatusText(status: string) {
  if (status === "healthy") return "正常";
  if (status === "warning") return "警告";
  if (status === "disabled") return "停用";
  if (status === "blocked") return "阻塞";
  return status || "未知";
}
