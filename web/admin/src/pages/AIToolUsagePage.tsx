import { Fragment, useEffect, useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from "recharts";
import {
  Bot,
  Coins,
  Download,
  FileJson,
  FileSpreadsheet,
  Loader2,
  Maximize2,
  Minimize2,
  RefreshCw,
  SlidersHorizontal,
  Target,
  X
} from "lucide-react";
import { api } from "../utils/api";

type Currency = "USD" | "CNY";
type WidgetSize = "compact" | "wide";

type Summary = {
  total_cost_usd: number;
  total_requests: number;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_tokens: number;
  cache_read_tokens: number;
  cache_hit_rate: number;
  pricing_fallbacks: number;
  scanned_sources: number;
  last_scan: string;
};

type Breakdown = {
  name: string;
  tool?: string;
  cost_usd: number;
  requests: number;
  tokens: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_hit_rate: number;
  pricing_fallbacks: number;
  last_seen?: string;
};

type DashboardPayload = {
  summary: Summary;
  trend: Array<{ day: string; cost_usd: number; requests: number; tokens: number; cache_read_tokens: number }>;
  heatmap: Array<{ day: string; hour: number; cost_usd: number; requests: number; tokens: number }>;
  model_rank: Breakdown[];
  project_spend: Breakdown[];
  tool_breakdown: Breakdown[];
  sources: Array<{ tool: string; path: string; records_found: number; records_created: number; last_scanned_at: string; error_message: string }>;
  recent: Array<{ id: string; tool: string; model: string; project_name: string; total_cost_usd: number; total_tokens: number; occurred_at: string }>;
};

const toolColors = ["#66c7b8", "#79afd9", "#d9ad63", "#df7c86", "#8abf72", "#b89ce8"];

export function AIToolUsagePage() {
  const [days, setDays] = useState(30);
  const [currency, setCurrency] = useState<Currency>("CNY");
  const [exchangeRate, setExchangeRate] = useState(7.2);
  const [metric, setMetric] = useState<"cost" | "requests" | "tokens">("cost");
  const [data, setData] = useState<DashboardPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [widgetOpen, setWidgetOpen] = useState(true);
  const [widgetOpacity, setWidgetOpacity] = useState(92);
  const [widgetSize, setWidgetSize] = useState<WidgetSize>("compact");

  const load = async () => {
    setLoading(true);
    setError(null);
    const result = await api.get<DashboardPayload>(`/ai-tool-usage?days=${days}`);
    if (!result.ok || !result.data) {
      setData(null);
      setError(result.error ?? "无法读取本地 AI Coding 工具用量");
      setLoading(false);
      return;
    }
    setData(result.data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
  }, [days]);

  const display = useMemo(() => {
    const rate = currency === "CNY" ? exchangeRate : 1;
    const prefix = currency === "CNY" ? "¥" : "$";
    return {
      rate,
      prefix,
      money: (usd: number) => `${prefix}${(usd * rate).toFixed(usd * rate >= 10 ? 2 : 4)}`
    };
  }, [currency, exchangeRate]);

  const trendData = useMemo(() => {
    return (data?.trend ?? []).map((item) => ({
      ...item,
      cost: item.cost_usd * display.rate
    }));
  }, [data, display.rate]);

  const modelChart = useMemo(() => {
    return (data?.model_rank ?? []).slice(0, 8).map((item) => ({
      ...item,
      cost: item.cost_usd * display.rate
    }));
  }, [data, display.rate]);

  const modelDistribution = useMemo(() => {
    return (data?.model_rank ?? []).slice(0, 6).map((item) => ({
      ...item,
      cost: item.cost_usd * display.rate
    }));
  }, [data, display.rate]);

  const heatmapRows = useMemo(() => {
    const rows: Array<{ day: string; cells: DashboardPayload["heatmap"] }> = [];
    const byDay = new Map<string, DashboardPayload["heatmap"]>();
    for (const item of data?.heatmap ?? []) {
      if (!byDay.has(item.day)) byDay.set(item.day, []);
      byDay.get(item.day)!.push(item);
    }
    for (const [day, cells] of byDay) {
      rows.push({ day, cells: cells.slice().sort((a, b) => a.hour - b.hour) });
    }
    return rows;
  }, [data]);

  const maxHeat = useMemo(() => {
    const values = (data?.heatmap ?? []).map((item) => (metric === "cost" ? item.cost_usd : metric === "requests" ? item.requests : item.tokens));
    return Math.max(1, ...values);
  }, [data, metric]);

  const scanNow = async () => {
    setScanning(true);
    const result = await api.post<{ records_created: number }>("/ai-tool-usage/scan");
    setScanning(false);
    if (!result.ok) {
      setError(result.error ?? "扫描失败");
      return;
    }
    await load();
  };

  const exportReport = (format: "csv" | "json" | "xlsx") => {
    const params = new URLSearchParams({
      format,
      days: String(days),
      exchange_rate: String(exchangeRate)
    });
    window.open(`/admin/api/ai-tool-usage/export?${params.toString()}`, "_blank");
  };

  if (loading) {
    return (
      <div className="ai-usage-page flex-col gap-4">
        <div className="ai-usage-hero skeleton" />
        <div className="kpi-grid">
          {Array.from({ length: 4 }).map((_, index) => (
            <div className="kpi-card" key={index}>
              <div className="skeleton skeleton-text" style={{ width: "45%" }} />
              <div className="skeleton skeleton-title" style={{ width: "70%" }} />
              <div className="skeleton skeleton-text" style={{ width: "55%" }} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (error && !data) {
    return (
      <div className="empty-state" style={{ minHeight: 420 }}>
        <Bot size={34} style={{ color: "var(--danger)" }} />
        <span className="empty-state-title">AI Coding 用量加载失败</span>
        <span className="empty-state-desc">{error}</span>
        <button type="button" className="btn btn-primary btn-sm" onClick={() => void load()}>
          <RefreshCw size={14} /> 重试
        </button>
      </div>
    );
  }

  const summary = data?.summary;

  return (
    <div className="ai-usage-page flex-col gap-4">
      <section className="ai-usage-hero">
        <div>
          <span className="eyebrow">Local AI Coding Spend</span>
          <h2>AI Coding 工具 Token 成本与用量</h2>
          <p>自动汇总 Kimi Code、Claude Code、Codex、Qoder、WorkBuddy、Hermes 的本机会话日志，所有解析、去重和成本估算仅保存在本地。</p>
        </div>
        <div className="ai-usage-toolbar">
          <div className="tabs">
            {[7, 30, 90].map((value) => (
              <button key={value} type="button" className={`tab ${days === value ? "active" : ""}`} onClick={() => setDays(value)}>
                {value} 天
              </button>
            ))}
          </div>
          <div className="tabs">
            <button type="button" className={`tab ${currency === "CNY" ? "active" : ""}`} onClick={() => setCurrency("CNY")}>CNY</button>
            <button type="button" className={`tab ${currency === "USD" ? "active" : ""}`} onClick={() => setCurrency("USD")}>USD</button>
          </div>
          <label className="ai-rate-field">
            <span>汇率</span>
            <input className="form-control" type="number" min="0" step="0.01" value={exchangeRate} onChange={(event) => setExchangeRate(Number(event.target.value) || 1)} />
          </label>
          <button type="button" className="btn btn-primary btn-sm" onClick={scanNow} disabled={scanning}>
            {scanning ? <Loader2 size={14} className="spin" /> : <RefreshCw size={14} />}
            扫描
          </button>
        </div>
      </section>

      <section className="kpi-grid">
        <article className="kpi-card ai-kpi">
          <span className="kpi-label">总成本</span>
          <span className="kpi-value">{display.money(summary?.total_cost_usd ?? 0)}</span>
          <span className="kpi-delta">{currency === "CNY" ? `$${(summary?.total_cost_usd ?? 0).toFixed(4)} USD` : "来自本地价格表"}</span>
        </article>
        <article className="kpi-card ai-kpi">
          <span className="kpi-label">总请求数</span>
          <span className="kpi-value">{(summary?.total_requests ?? 0).toLocaleString()}</span>
          <span className="kpi-delta">{(summary?.total_tokens ?? 0).toLocaleString()} tokens</span>
        </article>
        <article className="kpi-card ai-kpi">
          <span className="kpi-label">缓存命中率</span>
          <span className="kpi-value">{((summary?.cache_hit_rate ?? 0) * 100).toFixed(1)}%</span>
          <span className="kpi-delta">{(summary?.cache_read_tokens ?? 0).toLocaleString()} cache-read tokens</span>
        </article>
        <article className="kpi-card ai-kpi">
          <span className="kpi-label">日志源</span>
          <span className="kpi-value">{(summary?.scanned_sources ?? 0).toLocaleString()}</span>
          <span className="kpi-delta">fallback 计价 {summary?.pricing_fallbacks ?? 0} 条</span>
        </article>
      </section>

      <section className="ai-usage-grid">
        <article className="panel ai-chart-panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">趋势变化</span>
              <h3 className="section-title">成本、请求与 Token</h3>
            </div>
            <div className="tabs">
              <button type="button" className={`tab ${metric === "cost" ? "active" : ""}`} onClick={() => setMetric("cost")}>成本</button>
              <button type="button" className={`tab ${metric === "requests" ? "active" : ""}`} onClick={() => setMetric("requests")}>请求</button>
              <button type="button" className={`tab ${metric === "tokens" ? "active" : ""}`} onClick={() => setMetric("tokens")}>Token</button>
            </div>
          </div>
          <div className="chart-container chart-container-lg">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={trendData}>
                <defs>
                  <linearGradient id="aiUsageFill" x1="0" x2="0" y1="0" y2="1">
                    <stop offset="0%" stopColor="var(--accent)" stopOpacity={0.28} />
                    <stop offset="100%" stopColor="var(--accent)" stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="var(--border-subtle)" vertical={false} />
                <XAxis dataKey="day" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} />
                <YAxis stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} width={48} />
                <Tooltip content={<ChartTooltip money={display.money} metric={metric} />} />
                <Area type="monotone" dataKey={metric === "cost" ? "cost" : metric} stroke="var(--accent)" strokeWidth={2} fill="url(#aiUsageFill)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </article>

        <article className="panel ai-chart-panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">模型分布</span>
              <h3 className="section-title">成本占比与工具来源</h3>
            </div>
          </div>
          <div className="ai-pie-layout">
            <ResponsiveContainer width="100%" height={220}>
              <PieChart>
                <Pie data={modelDistribution} dataKey="cost" nameKey="name" innerRadius={54} outerRadius={86} paddingAngle={2}>
                  {modelDistribution.map((_, index) => (
                    <Cell key={index} fill={toolColors[index % toolColors.length]} />
                  ))}
                </Pie>
                <Tooltip formatter={(value: number) => display.money(value)} />
              </PieChart>
            </ResponsiveContainer>
            <div className="ai-tool-list">
              {modelDistribution.map((item, index) => (
                <div key={item.name} className="ai-rank-row">
                  <span className="ai-dot" style={{ background: toolColors[index % toolColors.length] }} />
                  <span className="truncate">{item.name}</span>
                  <strong>{display.money(item.cost_usd)}</strong>
                </div>
              ))}
            </div>
            <div className="ai-secondary-list">
              {(data?.tool_breakdown ?? []).slice(0, 4).map((item) => (
                <span key={item.name}>{item.name}: {display.money(item.cost_usd)}</span>
              ))}
            </div>
          </div>
        </article>
      </section>

      <section className="ai-usage-grid ai-usage-grid-bottom">
        <article className="panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">模型排行</span>
              <h3 className="section-title">成本最高的模型</h3>
            </div>
          </div>
          <div className="chart-container">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={modelChart} layout="vertical" margin={{ left: 12, right: 18, top: 4, bottom: 4 }}>
                <CartesianGrid stroke="var(--border-subtle)" horizontal={false} />
                <XAxis type="number" stroke="var(--text-tertiary)" fontSize={12} tickLine={false} axisLine={false} />
                <YAxis dataKey="name" type="category" width={120} stroke="var(--text-secondary)" fontSize={12} tickLine={false} axisLine={false} />
                <Tooltip formatter={(value: number) => display.money(value)} />
                <Bar dataKey="cost" radius={[0, 4, 4, 0]} barSize={18} fill="var(--info)" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </article>

        <article className="panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">项目消耗</span>
              <h3 className="section-title">按项目拆分</h3>
            </div>
          </div>
          <div className="ai-rank-list">
            {(data?.project_spend ?? []).slice(0, 8).map((item, index) => (
              <div key={`${item.name}-${index}`} className="ai-rank-item">
                <div>
                  <strong>{item.name}</strong>
                  <span>{item.tool || "mixed"} · {item.requests.toLocaleString()} requests · {(item.cache_hit_rate * 100).toFixed(1)}% cache</span>
                </div>
                <em>{display.money(item.cost_usd)}</em>
              </div>
            ))}
          </div>
        </article>
      </section>

      <section className="panel">
        <div className="section-header">
          <div className="section-header-main">
            <span className="eyebrow">热力图</span>
            <h3 className="section-title">小时级消耗密度</h3>
          </div>
          <div className="section-actions">
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => exportReport("csv")}><Download size={14} /> CSV</button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => exportReport("json")}><FileJson size={14} /> JSON</button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => exportReport("xlsx")}><FileSpreadsheet size={14} /> Excel</button>
          </div>
        </div>
        <div className="ai-heatmap" aria-label="AI Coding hourly usage heatmap">
          <span className="ai-heatmap-corner">Day</span>
          {Array.from({ length: 24 }).map((_, hour) => (
            <span key={`h-${hour}`} className="ai-heatmap-hour">{hour}</span>
          ))}
          {heatmapRows.map((row) => (
            <Fragment key={row.day}>
              <span className="ai-heatmap-day">{row.day}</span>
              {row.cells.map((item) => {
                const value = metric === "cost" ? item.cost_usd : metric === "requests" ? item.requests : item.tokens;
                return (
                  <span
                    key={`${item.day}-${item.hour}`}
                    className="ai-heatmap-cell"
                    style={{ opacity: Math.max(0.14, value / maxHeat) }}
                    title={`${item.day} ${item.hour}:00 · ${display.money(item.cost_usd)} · ${item.requests} requests`}
                  />
                );
              })}
            </Fragment>
          ))}
        </div>
      </section>

      <section className="panel panel-compact">
        <div className="section-header">
          <div className="section-header-main">
            <span className="eyebrow">本地来源</span>
            <h3 className="section-title">最近扫描的日志</h3>
          </div>
        </div>
        <div className="ai-source-list">
          {(data?.sources ?? []).slice(0, 8).map((source) => (
            <div className="ai-source-row" key={source.path}>
              <Target size={14} />
              <div className="truncate">
                <strong>{source.tool}</strong>
                <span>{source.path}</span>
              </div>
              <em>{source.records_created}/{source.records_found}</em>
            </div>
          ))}
        </div>
      </section>

      {widgetOpen ? (
        <aside className={`ai-floating-widget ${widgetSize}`} style={{ opacity: widgetOpacity / 100 }}>
          <div className="ai-floating-head">
            <span><Bot size={14} /> AI Spend</span>
            <div>
              <button type="button" onClick={() => setWidgetSize(widgetSize === "compact" ? "wide" : "compact")} aria-label="调整悬浮窗大小">
                {widgetSize === "compact" ? <Maximize2 size={13} /> : <Minimize2 size={13} />}
              </button>
              <button type="button" onClick={() => setWidgetOpen(false)} aria-label="关闭悬浮窗"><X size={13} /></button>
            </div>
          </div>
          <strong>{display.money(summary?.total_cost_usd ?? 0)}</strong>
          <span>{(summary?.total_requests ?? 0).toLocaleString()} requests · {((summary?.cache_hit_rate ?? 0) * 100).toFixed(1)}% cache</span>
          <label>
            <SlidersHorizontal size={12} />
            <input type="range" min={55} max={100} value={widgetOpacity} onChange={(event) => setWidgetOpacity(Number(event.target.value))} />
          </label>
        </aside>
      ) : (
        <button type="button" className="ai-widget-toggle" onClick={() => setWidgetOpen(true)} aria-label="打开 AI 成本悬浮窗">
          <Coins size={16} />
        </button>
      )}
    </div>
  );
}

function ChartTooltip({ active, payload, label, money, metric }: { active?: boolean; payload?: Array<{ value: number }>; label?: string; money: (value: number) => string; metric: string }) {
  if (!active || !payload?.length) return null;
  const value = payload[0].value;
  return (
    <div className="ai-chart-tooltip">
      <strong>{label}</strong>
      <span>{metric === "cost" ? money(value) : Number(value).toLocaleString()}</span>
    </div>
  );
}
