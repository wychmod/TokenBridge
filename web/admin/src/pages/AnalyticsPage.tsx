import { useMemo, useState, useEffect } from "react";
import { Bar, BarChart, CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis, Cell } from "recharts";
import { Loader2, Zap } from "lucide-react";
import { useSearchParams } from "react-router-dom";

type BreakdownItem = { name: string; cost: number; requests: number; tokens: number };

type AnalyticsPayload = {
  summary: {
    total_requests: number;
    total_cost_usd: number;
    input_tokens: number;
    output_tokens: number;
    success_rate: number;
  };
  trend: Array<{ day: string; cost: number; requests: number; tokens: number }>;
  provider_breakdown: BreakdownItem[];
  model_breakdown: BreakdownItem[];
  key_breakdown: BreakdownItem[];
  log_stats: { total: number; failures: number; fallbacks: number; avg_latency_ms: number };
};

export function AnalyticsPage() {
  const [searchParams] = useSearchParams();
  const rangeParam = searchParams.get("range");
  const metricParam = searchParams.get("metric");
  const [range, setRange] = useState(normalizeAnalyticsRange(rangeParam));
  const [dimension, setDimension] = useState<"cost" | "requests">(normalizeAnalyticsMetric(metricParam));
  const [breakdownMode, setBreakdownMode] = useState<"provider" | "model" | "key">("provider");
  const [data, setData] = useState<AnalyticsPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setRange(normalizeAnalyticsRange(rangeParam));
    setDimension(normalizeAnalyticsMetric(metricParam));
  }, [rangeParam, metricParam]);

  useEffect(() => {
    setLoading(true);
    setError(null);
    const days = range === "7d" ? 7 : 30;
    fetch(`/admin/api/analytics?days=${days}`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const payload = await res.json();
        setData(payload.data ?? null);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "加载失败"))
      .finally(() => setLoading(false));
  }, [range]);

  const summary = useMemo(() => {
    if (!data) return { totalCost: 0, totalRequests: 0, totalTokens: 0, successRate: 0, failures: 0, fallbacks: 0, avgLatency: 0 };
    return {
      totalCost: data.summary.total_cost_usd,
      totalRequests: data.summary.total_requests,
      totalTokens: data.summary.input_tokens + data.summary.output_tokens,
      successRate: data.summary.success_rate,
      failures: data.log_stats.failures,
      fallbacks: data.log_stats.fallbacks,
      avgLatency: data.log_stats.avg_latency_ms
    };
  }, [data]);

  const activeBreakdown = useMemo(() => {
    if (!data) return [];
    if (breakdownMode === "model") return data.model_breakdown;
    if (breakdownMode === "key") return data.key_breakdown;
    return data.provider_breakdown;
  }, [data, breakdownMode]);

  const breakdownColor = breakdownMode === "provider" ? "var(--accent)" : breakdownMode === "model" ? "var(--info)" : "var(--warning)";

  if (loading) {
    return (
      <div className="flex-col gap-4">
        <div className="section-header">
          <div className="section-header-main">
            <span className="eyebrow">数据洞察</span>
            <h2 className="section-title">用量分析</h2>
          </div>
        </div>
        <div className="kpi-grid">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="kpi-card" style={{ minHeight: 100 }}>
              <div className="skeleton skeleton-text" style={{ width: "40%" }} />
              <div className="skeleton skeleton-title" style={{ width: "70%", marginTop: 8 }} />
              <div className="skeleton skeleton-text" style={{ width: "50%", marginTop: 8 }} />
            </div>
          ))}
        </div>
        <div className="panel" style={{ minHeight: 400 }}>
          <div className="skeleton skeleton-title" style={{ width: "25%" }} />
          <div className="skeleton" style={{ width: "100%", height: 320, marginTop: 16 }} />
        </div>
        <div className="panel" style={{ minHeight: 340 }}>
          <div className="skeleton skeleton-title" style={{ width: "30%" }} />
          <div className="skeleton" style={{ width: "100%", height: 260, marginTop: 16 }} />
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
      </div>
    );
  }

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">数据洞察</span>
          <h2 className="section-title">用量分析</h2>
          <p className="section-description">把请求、Token、费用和成功率转换成可解释的业务指标，方便研发排障和产品看趋势。</p>
        </div>
        <div className="section-actions">
          <div className="tabs">
            <button type="button" className={clsx("tab", range === "7d" && "active")} onClick={() => setRange("7d")}>7 天</button>
            <button type="button" className={clsx("tab", range === "30d" && "active")} onClick={() => setRange("30d")}>30 天</button>
          </div>
        </div>
      </div>

      {/* Summary KPIs */}
      <div className="kpi-grid">
        <div className="kpi-card">
          <span className="kpi-label">总费用</span>
          <span className="kpi-value">${summary.totalCost.toFixed(2)}</span>
          <span className="kpi-delta">美元</span>
        </div>
        <div className="kpi-card">
          <span className="kpi-label">总请求</span>
          <span className="kpi-value">{summary.totalRequests.toLocaleString()}</span>
          <span className="kpi-delta">次</span>
        </div>
        <div className="kpi-card">
          <span className="kpi-label">总令牌</span>
          <span className="kpi-value">{summary.totalTokens.toLocaleString()}</span>
          <span className="kpi-delta">tokens</span>
        </div>
        <div className="kpi-card">
          <span className="kpi-label">成功率</span>
          <span className="kpi-value" style={{ color: summary.successRate >= 0.95 ? "var(--accent)" : "var(--warning)" }}>
            {(summary.successRate * 100).toFixed(1)}%
          </span>
          <span className="kpi-delta">失败 {summary.failures} · 备用 {summary.fallbacks}</span>
        </div>
      </div>

      {/* Trend Chart */}
      <div className="panel">
        <div className="section-header" style={{ marginBottom: "var(--space-4)" }}>
          <div className="section-header-main">
            <span className="eyebrow">趋势变化</span>
            <h3 className="section-title">{dimension === "cost" ? "费用趋势" : "请求量趋势"}</h3>
          </div>
          <div className="section-actions">
            <div className="tabs">
              <button type="button" className={clsx("tab", dimension === "cost" && "active")} onClick={() => setDimension("cost")}>费用</button>
              <button type="button" className={clsx("tab", dimension === "requests" && "active")} onClick={() => setDimension("requests")}>请求</button>
            </div>
          </div>
        </div>
        <div className="chart-container chart-container-lg">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data?.trend ?? []}>
              <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
              <XAxis dataKey="day" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} />
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
              <Line
                type="monotone"
                dataKey={dimension}
                name={dimension === "cost" ? "费用 USD" : "请求数"}
                stroke="var(--accent)"
                strokeWidth={2}
                dot={{ r: 3, fill: "var(--accent)", strokeWidth: 0 }}
                activeDot={{ r: 5, stroke: "var(--accent)", strokeWidth: 2, fill: "var(--bg-surface)" }}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
        <p className="chart-note">切换“费用/请求”可区分成本异常与流量增长；tooltip 显示精确数值。</p>
      </div>

      {/* Breakdown */}
      <div className="panel">
        <div className="section-header" style={{ marginBottom: "var(--space-4)" }}>
          <div className="section-header-main">
            <span className="eyebrow">维度拆分</span>
            <h3 className="section-title">
              {breakdownMode === "provider" ? "供应商维度" : breakdownMode === "model" ? "模型维度" : "密钥维度"}
            </h3>
          </div>
          <div className="section-actions">
            <div className="tabs">
              <button type="button" className={clsx("tab", breakdownMode === "provider" && "active")} onClick={() => setBreakdownMode("provider")}>供应商</button>
              <button type="button" className={clsx("tab", breakdownMode === "model" && "active")} onClick={() => setBreakdownMode("model")}>模型</button>
              <button type="button" className={clsx("tab", breakdownMode === "key" && "active")} onClick={() => setBreakdownMode("key")}>密钥</button>
            </div>
          </div>
        </div>
        {activeBreakdown.length ? (
          <div className="chart-container">
            <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={activeBreakdown}
              layout="vertical"
              margin={{ left: 16, right: 16, top: 8, bottom: 8 }}
            >
              <CartesianGrid stroke="var(--border-subtle)" horizontal={false} />
              <XAxis type="number" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} />
              <YAxis
                dataKey="name"
                type="category"
                stroke="var(--text-secondary)"
                fontSize={12}
                tickLine={false}
                axisLine={false}
                width={100}
              />
              <Tooltip
                contentStyle={{
                  background: "var(--bg-surface)",
                  border: "1px solid var(--border-default)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "0.82rem",
                  boxShadow: "var(--shadow-md)"
                }}
                formatter={(value: number) => [`$${value.toFixed(2)}`, "费用"]}
              />
              <Legend verticalAlign="top" height={28} />
              <Bar dataKey="cost" name="费用 USD" radius={[0, 4, 4, 0]} barSize={20}>
                {activeBreakdown.map((_, i) => (
                  <Cell key={i} fill={breakdownColor} opacity={0.7 + (i % 3) * 0.15} />
                ))}
              </Bar>
            </BarChart>
            </ResponsiveContainer>
          </div>
        ) : (
          <div className="chart-empty">暂无可拆分数据。发起真实请求后会显示 Provider、模型和密钥维度排行。</div>
        )}
      </div>
    </div>
  );
}

function normalizeAnalyticsRange(value: string | null) {
  return value === "30d" ? "30d" : "7d";
}

function normalizeAnalyticsMetric(value: string | null): "cost" | "requests" {
  return value === "requests" ? "requests" : "cost";
}

function clsx(...classes: Array<string | false | undefined>) {
  return classes.filter(Boolean).join(" ");
}
