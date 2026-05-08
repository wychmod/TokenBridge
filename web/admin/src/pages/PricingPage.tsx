import { useEffect, useMemo, useState } from "react";
import { DollarSign, RefreshCw, Search, TrendingUp, Database, CheckCircle, AlertCircle, Loader2, Eye } from "lucide-react";
import { useAdminStore } from "../store/admin-store";

type ModelPricing = {
  model_id: string;
  litellm_provider: string;
  mode: string;
  max_input_tokens: number;
  max_output_tokens: number;
  input_cost_per_token: number;
  output_cost_per_token: number;
  cache_creation_cost_per_token: number;
  cache_read_cost_per_token: number;
  supports_vision: boolean;
  supports_function_calling: boolean;
  supports_prompt_caching: boolean;
  fetched_at: string;
};

type PricingStatus = {
  total_models: number;
  last_sync: string;
};

type LookupResult = {
  data: ModelPricing | null;
  matched: boolean;
  fallback_used: boolean;
  fallback_model: string;
};

export function PricingPage() {
  const { pushNotice } = useAdminStore();
  const [models, setModels] = useState<ModelPricing[]>([]);
  const [status, setStatus] = useState<PricingStatus | null>(null);
  const [search, setSearch] = useState("");
  const [modeFilter, setModeFilter] = useState<"all" | "chat" | "embedding" | "image_generation" | "audio_transcription">("all");
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [lookupModel, setLookupModel] = useState("");
  const [lookupResult, setLookupResult] = useState<LookupResult | null>(null);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<"browse" | "lookup" | "estimate">("browse");

  // Estimate state
  const [estModel, setEstModel] = useState("");
  const [estInput, setEstInput] = useState(1000);
  const [estOutput, setEstOutput] = useState(500);
  const [estResult, setEstResult] = useState<{
    model_id: string;
    input_tokens: number;
    output_tokens: number;
    estimated_usd: number;
    matched: boolean;
    fallback_used: boolean;
    fallback_model: string;
  } | null>(null);

  const loadStatus = async () => {
    try {
      const res = await fetch("/admin/api/pricing/status");
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setStatus(data);
    } catch {
      // silent
    }
  };

  const loadModels = async () => {
    setLoading(true);
    try {
      const qs = modeFilter !== "all" ? `?mode=${modeFilter}` : "";
      const res = await fetch(`/admin/api/pricing/list${qs}`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setModels(data.data ?? []);
    } catch (err) {
      pushNotice({ tone: "warning", title: "定价加载失败", message: err instanceof Error ? err.message : "请检查后端服务" });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadStatus();
    void loadModels();
  }, [modeFilter]);

  const handleRefresh = async () => {
    setSyncing(true);
    try {
      const res = await fetch("/admin/api/pricing/refresh", { method: "POST" });
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.message || data.error || `HTTP ${res.status}`);
      }
      const data = await res.json();
      await loadStatus();
      await loadModels();
      pushNotice({ tone: "success", title: "定价同步成功", message: `已同步 ${data.models_synced} 个模型` });
    } catch (err) {
      pushNotice({ tone: "warning", title: "定价同步失败", message: err instanceof Error ? err.message : "请检查网络或后端服务" });
    } finally {
      setSyncing(false);
    }
  };

  const handleLookup = async () => {
    if (!lookupModel.trim()) return;
    setLookupLoading(true);
    try {
      const res = await fetch(`/admin/api/pricing/lookup?model=${encodeURIComponent(lookupModel.trim())}`);
      const data = await res.json();
      setLookupResult(data);
    } catch {
      setLookupResult(null);
    } finally {
      setLookupLoading(false);
    }
  };

  const handleEstimate = async () => {
    if (!estModel.trim()) return;
    try {
      const params = new URLSearchParams({
        model: estModel.trim(),
        input_tokens: String(estInput),
        output_tokens: String(estOutput),
      });
      const res = await fetch(`/admin/api/pricing/estimate?${params}`);
      const data = await res.json();
      setEstResult(data);
    } catch {
      setEstResult(null);
    }
  };

  const filteredModels = useMemo(() => {
    if (!search.trim()) return models;
    const q = search.toLowerCase();
    return models.filter(
      (m) =>
        m.model_id.toLowerCase().includes(q) ||
        m.litellm_provider.toLowerCase().includes(q)
    );
  }, [models, search]);

  const fmtCost = (val: number) => {
    if (val === 0) return "—";
    const perM = val * 1_000_000;
    return perM >= 1 ? `$${perM.toFixed(2)}` : `$${perM.toFixed(4)}`;
  };

  const fmtUSD = (val: number) => {
    if (val === 0) return "$0.00";
    if (val < 0.01) return `$${val.toFixed(6)}`;
    return `$${val.toFixed(4)}`;
  };

  const fmtDate = (s: string) => {
    if (!s) return "从未同步";
    try {
      return new Date(s).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
    } catch {
      return s;
    }
  };

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">计费管理</span>
          <h2 className="section-title">模型定价</h2>
        </div>
        <div className="section-actions" style={{ gap: 8 }}>
          <button type="button" className="btn btn-ghost btn-sm" onClick={handleRefresh} disabled={syncing}>
            <RefreshCw size={14} className={syncing ? "spin" : ""} />
            {syncing ? "同步中..." : "刷新定价"}
          </button>
        </div>
      </div>

      {/* Status bar */}
      {status && (
        <div style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
          <div className="stat-pill">
            <Database size={13} />
            <span>{status.total_models.toLocaleString()} 模型</span>
          </div>
          <div className="stat-pill">
            <TrendingUp size={13} />
            <span>上次同步: {fmtDate(status.last_sync)}</span>
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="tabs" style={{ alignSelf: "flex-start" }}>
        <button type="button" className={`tab ${activeTab === "browse" ? "active" : ""}`} onClick={() => setActiveTab("browse")}>
          定价浏览
        </button>
        <button type="button" className={`tab ${activeTab === "lookup" ? "active" : ""}`} onClick={() => setActiveTab("lookup")}>
          模型查询
        </button>
        <button type="button" className={`tab ${activeTab === "estimate" ? "active" : ""}`} onClick={() => setActiveTab("estimate")}>
          费用估算
        </button>
      </div>

      {/* Tab: Browse */}
      {activeTab === "browse" && (
        <div className="panel flex-col gap-4">
          {/* Filters */}
          <div style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap" }}>
            <div style={{ position: "relative", flex: 1, minWidth: 200 }}>
              <Search size={14} style={{ position: "absolute", left: 10, top: "50%", transform: "translateY(-50%)", opacity: 0.4 }} />
              <input
                className="form-control"
                style={{ paddingLeft: 32 }}
                placeholder="搜索模型名或 Provider..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <select className="form-control" style={{ width: 160 }} value={modeFilter} onChange={(e) => setModeFilter(e.target.value as typeof modeFilter)}>
              <option value="all">全部类型</option>
              <option value="chat">对话</option>
              <option value="embedding">向量</option>
              <option value="image_generation">图像生成</option>
              <option value="audio_transcription">语音转录</option>
            </select>
          </div>

          {/* Table */}
          {loading ? (
            <div style={{ display: "flex", justifyContent: "center", padding: 40, opacity: 0.5 }}>
              <Loader2 size={20} className="spin" />
            </div>
          ) : (
            <div style={{ overflowX: "auto" }}>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>模型</th>
                    <th>Provider</th>
                    <th>类型</th>
                    <th style={{ textAlign: "right" }}>输入 ($/M)</th>
                    <th style={{ textAlign: "right" }}>输出 ($/M)</th>
                    <th style={{ textAlign: "center" }}>视觉</th>
                    <th style={{ textAlign: "center" }}>函数调用</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredModels.length === 0 ? (
                    <tr><td colSpan={7} style={{ textAlign: "center", padding: 24, opacity: 0.5 }}>暂无数据</td></tr>
                  ) : (
                    filteredModels.slice(0, 200).map((m) => (
                      <tr key={m.model_id}>
                        <td>
                          <code style={{ fontSize: 12, fontWeight: 500 }}>{m.model_id}</code>
                        </td>
                        <td style={{ fontSize: 12, opacity: 0.7 }}>{m.litellm_provider || "—"}</td>
                        <td>
                          <span className="badge">{m.mode || "—"}</span>
                        </td>
                        <td style={{ textAlign: "right", fontVariantNumeric: "tabular-nums" }}>{fmtCost(m.input_cost_per_token)}</td>
                        <td style={{ textAlign: "right", fontVariantNumeric: "tabular-nums" }}>{fmtCost(m.output_cost_per_token)}</td>
                        <td style={{ textAlign: "center" }}>{m.supports_vision ? <CheckCircle size={14} style={{ color: "var(--c-green-600)" }} /> : <span style={{ opacity: 0.2 }}>—</span>}</td>
                        <td style={{ textAlign: "center" }}>{m.supports_function_calling ? <CheckCircle size={14} style={{ color: "var(--c-green-600)" }} /> : <span style={{ opacity: 0.2 }}>—</span>}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
              {filteredModels.length > 200 && (
                <div style={{ textAlign: "center", padding: 12, fontSize: 12, opacity: 0.5 }}>
                  显示前 200 条，共 {filteredModels.length} 条
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Tab: Lookup */}
      {activeTab === "lookup" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">精确查询</span>
              <h3 className="section-title">模型查询</h3>
            </div>
          </div>

          <div style={{ display: "flex", gap: 8 }}>
            <input
              className="form-control"
              placeholder="输入模型 ID，如 gpt-4o、claude-3-5-sonnet..."
              value={lookupModel}
              onChange={(e) => setLookupModel(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && void handleLookup()}
            />
            <button type="button" className="btn btn-primary btn-sm" onClick={handleLookup} disabled={lookupLoading}>
              {lookupLoading ? <Loader2 size={14} className="spin" /> : <Eye size={14} />}
              查询
            </button>
          </div>

          {lookupResult && (
            <div className="panel" style={{ background: "var(--color-background-secondary)" }}>
              {lookupResult.data ? (
                <div className="flex-col gap-3">
                  {lookupResult.fallback_used && (
                    <div className="alert alert-warning" style={{ marginBottom: 8 }}>
                      <AlertCircle size={14} />
                      <span>未找到精确匹配，已使用 <code>{lookupResult.fallback_model}</code> 价格兜底</span>
                    </div>
                  )}
                  {lookupResult.matched && (
                    <div className="alert alert-success" style={{ marginBottom: 8 }}>
                      <CheckCircle size={14} />
                      <span>精确匹配成功</span>
                    </div>
                  )}
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(180px, 1fr))", gap: 12 }}>
                    <div className="stat-pill"><span>模型</span><strong>{lookupResult.data.model_id}</strong></div>
                    <div className="stat-pill"><span>Provider</span><strong>{lookupResult.data.litellm_provider}</strong></div>
                    <div className="stat-pill"><span>类型</span><strong>{lookupResult.data.mode}</strong></div>
                    <div className="stat-pill"><span>输入价格</span><strong>{fmtCost(lookupResult.data.input_cost_per_token)} /M</strong></div>
                    <div className="stat-pill"><span>输出价格</span><strong>{fmtCost(lookupResult.data.output_cost_per_token)} /M</strong></div>
                    {lookupResult.data.max_input_tokens > 0 && (
                      <div className="stat-pill"><span>最大输入</span><strong>{(lookupResult.data.max_input_tokens / 1000).toFixed(0)}K</strong></div>
                    )}
                    {lookupResult.data.max_output_tokens > 0 && (
                      <div className="stat-pill"><span>最大输出</span><strong>{(lookupResult.data.max_output_tokens / 1000).toFixed(0)}K</strong></div>
                    )}
                  </div>
                </div>
              ) : (
                <div style={{ textAlign: "center", padding: 24, opacity: 0.5 }}>
                  未找到定价信息
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Tab: Estimate */}
      {activeTab === "estimate" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">费用预估</span>
              <h3 className="section-title">请求费用估算</h3>
            </div>
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr auto", gap: 12, alignItems: "end" }}>
            <div className="form-field">
              <label className="form-label">模型</label>
              <input className="form-control" placeholder="gpt-4o" value={estModel} onChange={(e) => setEstModel(e.target.value)} />
            </div>
            <div className="form-field">
              <label className="form-label">输入 Tokens</label>
              <input className="form-control" type="number" value={estInput} onChange={(e) => setEstInput(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">输出 Tokens</label>
              <input className="form-control" type="number" value={estOutput} onChange={(e) => setEstOutput(Number(e.target.value))} />
            </div>
            <button type="button" className="btn btn-primary btn-sm" onClick={handleEstimate}>
              <DollarSign size={14} /> 估算
            </button>
          </div>

          {estResult && (
            <div className="panel" style={{ background: "var(--color-background-secondary)" }}>
              <div className="flex-col gap-3">
                {estResult.fallback_used && (
                  <div className="alert alert-warning">
                    <AlertCircle size={14} />
                    <span>未匹配到 <code>{estResult.model_id}</code>，使用 <code>{estResult.fallback_model}</code> 兜底</span>
                  </div>
                )}
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(160px, 1fr))", gap: 12 }}>
                  <div className="stat-pill">
                    <span>预估费用</span>
                    <strong style={{ fontSize: 18, color: "var(--c-coral-600)" }}>{fmtUSD(estResult.estimated_usd)}</strong>
                  </div>
                  <div className="stat-pill"><span>输入 Tokens</span><strong>{estResult.input_tokens.toLocaleString()}</strong></div>
                  <div className="stat-pill"><span>输出 Tokens</span><strong>{estResult.output_tokens.toLocaleString()}</strong></div>
                  <div className="stat-pill"><span>匹配状态</span><strong>{estResult.matched ? "精确匹配" : "兜底"}</strong></div>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
