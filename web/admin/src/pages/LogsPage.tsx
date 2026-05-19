import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, Copy, Download, Search, ChevronDown, ChevronUp, Zap } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import clsx from "clsx";

type LogItem = {
  id: string;
  path: string;
  provider_id: string;
  latency_ms: number;
  status_label: string;
  detail: string;
  trace_id: string;
  fallback_used: boolean;
  fallback_tried: string[];
  created_at: string;
  metadata: Record<string, unknown>;
  status_code: number;
};

type LogStats = {
  total: number;
  failures: number;
  fallbacks: number;
  avg_latency_ms: number;
};

export function LogsPage() {
  const [searchParams] = useSearchParams();
  const statusParam = searchParams.get("status");
  const fallbackParam = searchParams.get("only_fallback");
  const [query, setQuery] = useState("");
  const [traceQuery, setTraceQuery] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [onlyFallback, setOnlyFallback] = useState(fallbackParam === "true");
  const [statusFilter, setStatusFilter] = useState(normalizeLogStatus(statusParam));
  const [providerFilter, setProviderFilter] = useState("");
  const [apiFormatFilter, setApiFormatFilter] = useState("");
  const [logs, setLogs] = useState<LogItem[]>([]);
  const [stats, setStats] = useState<LogStats>({ total: 0, failures: 0, fallbacks: 0, avg_latency_ms: 0 });
  const [selectedId, setSelectedId] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [detailExpanded, setDetailExpanded] = useState(false);

  useEffect(() => {
    setOnlyFallback(fallbackParam === "true");
    setStatusFilter(normalizeLogStatus(statusParam));
  }, [fallbackParam, statusParam]);

  const params = useMemo(() => {
    const p = new URLSearchParams();
    if (query) p.set("query", query);
    if (traceQuery) p.set("trace", traceQuery);
    if (fromDate) p.set("from", fromDate);
    if (toDate) p.set("to", toDate);
    if (onlyFallback) p.set("only_fallback", "true");
    if (providerFilter) p.set("provider", providerFilter);
    if (apiFormatFilter) p.set("api_format", apiFormatFilter);
    if (statusFilter !== "all") p.set("status", statusFilter);
    p.set("limit", "100");
    return p;
  }, [query, traceQuery, fromDate, toDate, onlyFallback, providerFilter, apiFormatFilter, statusFilter]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    fetch(`/admin/api/logs?${params.toString()}`, { signal: controller.signal })
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const payload = await res.json();
        const items = (payload.data ?? []) as LogItem[];
        setLogs(items);
        setStats(payload.stats ?? { total: items.length, failures: 0, fallbacks: 0, avg_latency_ms: 0 });
        setSelectedId((current) => current || items[0]?.id || "");
      })
      .catch((err) => {
        if (err.name !== "AbortError") {
          setError(err instanceof Error ? err.message : "加载失败");
        }
      })
      .finally(() => setLoading(false));

    return () => controller.abort();
  }, [params.toString()]);

  const active = logs.find((item) => item.id === selectedId) ?? logs[0];

  const handleExport = () => {
    window.open(`/admin/api/logs/export?${params.toString()}`, "_blank");
  };

  const clearFilters = () => {
    setQuery("");
    setTraceQuery("");
    setFromDate("");
    setToDate("");
    setOnlyFallback(false);
    setStatusFilter("all");
    setProviderFilter("");
    setApiFormatFilter("");
  };

  const statusDot = (code: number) => {
    if (code >= 500) return "var(--danger)";
    if (code >= 400) return "var(--warning)";
    return "var(--accent)";
  };

  if (error && !loading) {
    return (
      <div className="empty-state" style={{ minHeight: 400 }}>
        <Zap size={32} style={{ color: "var(--danger)", opacity: 0.6 }} />
        <span className="empty-state-title" style={{ color: "var(--danger)" }}>日志加载失败</span>
        <span className="empty-state-desc">{error}，请检查后端服务是否正常运行。</span>
      </div>
    );
  }

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">运行记录</span>
          <h2 className="section-title">日志检索</h2>
          <p className="section-description">围绕 Trace ID、Provider、状态码、Fallback 和时间范围排查失败请求。左侧定位，右侧复现链路。</p>
        </div>
        <div className="section-actions">
          <button
            type="button"
            className={clsx("btn btn-sm", onlyFallback ? "btn-primary" : "btn-secondary")}
            onClick={() => setOnlyFallback((v) => !v)}
          >
            <AlertTriangle size={13} /> 备用切换
          </button>
          <button type="button" className="btn btn-secondary btn-sm" onClick={handleExport}>
            <Download size={13} /> 导出
          </button>
          <button type="button" className="btn btn-ghost btn-sm" onClick={clearFilters}>
            清空筛选
          </button>
        </div>
      </div>

      {/* Compact Filters */}
      <div className="panel panel-compact flex-col gap-3">
        <div className="toolbar-row">
          <div className="secret-input" style={{ flex: 1 }}>
            <Search size={16} style={{ color: "var(--text-tertiary)", flexShrink: 0 }} />
            <input
              className="form-control"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="按路径、fallback、错误信息搜索"
            />
          </div>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            onClick={() => setShowFilters((v) => !v)}
          >
            筛选 {showFilters ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </button>
        </div>

        {showFilters && (
          <div className="form-grid" style={{ marginTop: 4 }}>
            <input
              className="form-control"
              value={traceQuery}
              onChange={(e) => setTraceQuery(e.target.value)}
              placeholder="精确搜索 Trace ID"
            />
            <input className="form-control" type="date" value={fromDate} onChange={(e) => setFromDate(e.target.value)} />
            <input className="form-control" type="date" value={toDate} onChange={(e) => setToDate(e.target.value)} />
            <input
              className="form-control"
              value={providerFilter}
              onChange={(e) => setProviderFilter(e.target.value)}
              placeholder="筛选供应商"
            />
            <input
              className="form-control"
              value={apiFormatFilter}
              onChange={(e) => setApiFormatFilter(e.target.value)}
              placeholder="筛选接口格式"
            />
            <select className="form-control" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
              <option value="all">全部状态</option>
              <option value="failed">只看失败</option>
              <option value="success">只看成功</option>
            </select>
          </div>
        )}

        <div className="flex items-center gap-2" style={{ fontSize: "0.78rem", color: "var(--text-tertiary)" }}>
          <span>匹配 {logs.length} 条</span>
          <span>·</span>
          <span>备用 {stats.fallbacks}</span>
          <span>·</span>
          <span>失败 {stats.failures}</span>
          <span>·</span>
          <span>平均延迟 {stats.avg_latency_ms}ms</span>
          {loading && <span className="pill pill-neutral" style={{ marginLeft: "auto" }}>加载中…</span>}
        </div>
      </div>

      {/* Logs Table + Detail */}
      <div className="logs-grid" style={{ minHeight: 400 }}>
        {/* Log List */}
        <div className="flex-col gap-1">
          {loading && logs.length === 0 ? (
            <div className="flex-col gap-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="panel panel-compact flex items-center gap-3">
                  <div className="skeleton skeleton-circle" style={{ width: 10, height: 10, flexShrink: 0 }} />
                  <div className="flex-col gap-1" style={{ flex: 1 }}>
                    <div className="skeleton skeleton-text" style={{ width: "60%" }} />
                    <div className="skeleton skeleton-text" style={{ width: "40%" }} />
                  </div>
                  <div className="skeleton skeleton-text" style={{ width: 50 }} />
                </div>
              ))}
            </div>
          ) : logs.length ? (
            <div className="flex-col gap-1 list-animate" style={{ overflow: "auto" }}>
              {logs.map((log, index) => (
                <button
                  key={log.id}
                  type="button"
                  className="list-row"
                  aria-label={`${log.path}，状态 ${log.status_label}，延迟 ${log.latency_ms}ms`}
                  style={{
                    "--index": index,
                    padding: "var(--space-2) var(--space-3)",
                    borderColor: active?.id === log.id ? "var(--accent)" : undefined,
                    background: active?.id === log.id ? "var(--accent-dim)" : undefined
                  } as React.CSSProperties}
                  onClick={() => { setSelectedId(log.id); setDetailExpanded(false); }}
                >
                  <div className="flex items-center gap-3" style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        background: statusDot(log.status_code),
                        flexShrink: 0,
                        boxShadow: `0 0 6px ${statusDot(log.status_code)}40`
                      }}
                    />
                    <div className="list-row-main" style={{ gap: 2 }}>
                      <span className="list-row-title">{log.path}</span>
                      <span className="list-row-sub">
                        {log.provider_id} · {new Date(log.created_at).toLocaleTimeString()}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-3" style={{ flexShrink: 0 }}>
                    <span className="data-value" style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>
                      {log.latency_ms}ms
                    </span>
                    <span
                      className="pill"
                      style={{
                        background: log.fallback_used ? "var(--warning-dim)" : log.status_code >= 400 ? "var(--danger-dim)" : "var(--accent-dim)",
                        color: log.fallback_used ? "var(--warning)" : log.status_code >= 400 ? "var(--danger)" : "var(--accent)"
                      }}
                    >
                      {log.status_label}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          ) : (
            <div className="empty-state">
              <span className="empty-state-title">没有匹配到日志</span>
              <span className="empty-state-desc">换个筛选条件试试，或者清空筛选回到最近 100 条请求。</span>
              <div className="empty-state-actions">
                <button type="button" className="btn btn-secondary btn-sm" onClick={clearFilters}>清空筛选</button>
              </div>
            </div>
          )}
        </div>

        {/* Detail Panel */}
        <div className="panel" style={{ overflow: "auto" }}>
          {active ? (
            <div className="flex-col gap-4">
              <div>
                <span className="eyebrow">日志详情</span>
                <h3 className="section-title" style={{ marginTop: 4, wordBreak: "break-all" }}>{active.path}</h3>
              </div>

              <div className="flex-col gap-2">
                {[
                  { label: "时间", value: new Date(active.created_at).toLocaleString() },
                  { label: "供应商", value: active.provider_id },
                  { label: "延迟", value: `${active.latency_ms}ms` },
                  { label: "状态", value: `${active.status_label} (${active.status_code})` },
                  { label: "Trace", value: active.trace_id || "暂无" }
                ].map((item) => (
                  <div key={item.label} className="flex items-center justify-between gap-2">
                    <span style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>{item.label}</span>
                    <span style={{ fontSize: "0.82rem", color: "var(--text-primary)", fontFamily: "var(--font-mono)", textAlign: "right" }}>
                      {item.value}
                    </span>
                  </div>
                ))}
              </div>

              {active.trace_id ? (
                <button
                  type="button"
                  className="btn btn-secondary btn-sm"
                  style={{ alignSelf: "flex-start" }}
                  onClick={() => void navigator.clipboard?.writeText(active.trace_id)}
                >
                  <Copy size={14} /> 复制 Trace ID
                </button>
              ) : null}

              <div className="flex-col gap-2" style={{ padding: "var(--space-3) 0", borderTop: "1px solid var(--border-subtle)" }}>
                <strong style={{ fontSize: "0.82rem" }}>详细说明</strong>
                <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", lineHeight: 1.5 }}>
                  {active.detail}
                </p>
              </div>

              <div className="flex-col gap-2" style={{ padding: "var(--space-3) 0", borderTop: "1px solid var(--border-subtle)" }}>
                <strong style={{ fontSize: "0.82rem" }}>调用链路</strong>
                <div className="flex-col gap-1" style={{ marginTop: 4 }}>
                  <div className="flex items-center justify-between">
                    <span style={{ fontSize: "0.78rem", color: "var(--text-secondary)" }}>请求格式</span>
                    <span style={{ fontSize: "0.8rem", fontFamily: "var(--font-mono)" }}>{String(active.metadata?.apiFormat ?? "未知")}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span style={{ fontSize: "0.78rem", color: "var(--text-secondary)" }}>请求模型</span>
                    <span style={{ fontSize: "0.8rem", fontFamily: "var(--font-mono)" }}>{String(active.metadata?.requestedModel ?? "未知")}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span style={{ fontSize: "0.78rem", color: "var(--text-secondary)" }}>实际模型</span>
                    <span style={{ fontSize: "0.8rem", fontFamily: "var(--font-mono)" }}>{String(active.metadata?.actualModel ?? "未知")}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span style={{ fontSize: "0.78rem", color: "var(--text-secondary)" }}>备用尝试</span>
                    <span style={{ fontSize: "0.8rem", fontFamily: "var(--font-mono)" }}>
                      {active.fallback_tried?.length ? active.fallback_tried.join(" → ") : "无"}
                    </span>
                  </div>
                </div>
              </div>

              {/* Raw JSON */}
              <div>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => setDetailExpanded((v) => !v)}
                >
                  {detailExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />} 原始 Metadata JSON
                </button>
                {detailExpanded && (
                  <pre
                    style={{
                      marginTop: 8,
                      padding: "var(--space-3)",
                      background: "var(--bg-base)",
                      borderRadius: "var(--radius-sm)",
                      fontSize: "0.75rem",
                      overflow: "auto",
                      maxHeight: 300,
                      fontFamily: "var(--font-mono)",
                      color: "var(--text-secondary)"
                    }}
                  >
                    {JSON.stringify(active.metadata, null, 2)}
                  </pre>
                )}
              </div>
            </div>
          ) : (
            <div className="empty-state" style={{ padding: "var(--space-5)" }}>
              <span className="empty-state-title">暂无详情</span>
              <span className="empty-state-desc">先在左边选一条日志</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function normalizeLogStatus(value: string | null) {
  if (value === "failed" || value === "success") return value;
  return "all";
}
