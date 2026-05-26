import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  CheckCircle,
  Database,
  DollarSign,
  Eye,
  Loader2,
  RefreshCw,
  Search,
  TrendingUp
} from "lucide-react";
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
  supports_reasoning: boolean;
  pricing_json: string;
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

const modeOptions = [
  { value: "all", label: "全部", shortLabel: "全部" },
  { value: "chat", label: "对话模型", shortLabel: "对话" },
  { value: "embedding", label: "向量模型", shortLabel: "向量" },
  { value: "image_generation", label: "图像生成", shortLabel: "图像" },
  { value: "audio_transcription", label: "语音转录", shortLabel: "语音" }
] as const;

type ModeFilter = (typeof modeOptions)[number]["value"];

const modeLabelMap: Record<string, string> = {
  chat: "对话",
  embedding: "向量",
  image_generation: "图像",
  audio_transcription: "语音"
};

export function PricingPage() {
  const { pushNotice } = useAdminStore();
  const [models, setModels] = useState<ModelPricing[]>([]);
  const [status, setStatus] = useState<PricingStatus | null>(null);
  const [search, setSearch] = useState("");
  const [modeFilter, setModeFilter] = useState<ModeFilter>("all");
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [lookupModel, setLookupModel] = useState("");
  const [lookupResult, setLookupResult] = useState<LookupResult | null>(null);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<"browse" | "lookup" | "estimate">("browse");

  const [estModel, setEstModel] = useState("");
  const [estInput, setEstInput] = useState(1000);
  const [estOutput, setEstOutput] = useState(500);
  const [estCacheCreation, setEstCacheCreation] = useState(0);
  const [estCacheRead, setEstCacheRead] = useState(0);
  const [estReasoning, setEstReasoning] = useState(0);
  const [estContextWindow, setEstContextWindow] = useState(0);
  const [estPricingTier, setEstPricingTier] = useState("");
  const [estResult, setEstResult] = useState<{
    model_id: string;
    input_tokens: number;
    output_tokens: number;
    cache_creation_tokens: number;
    cache_read_tokens: number;
    reasoning_tokens: number;
    context_window: number;
    pricing_tier: string;
    estimated_usd: number;
    cost_breakdown: {
      input_usd: number;
      output_usd: number;
      cache_creation_usd: number;
      cache_read_usd: number;
      reasoning_usd: number;
      total_usd: number;
    };
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
      // The model list remains usable when the status endpoint is unavailable.
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
        cache_creation_tokens: String(estCacheCreation),
        cache_read_tokens: String(estCacheRead),
        reasoning_tokens: String(estReasoning),
        context_window: String(estContextWindow),
        pricing_tier: estPricingTier.trim()
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
        m.litellm_provider.toLowerCase().includes(q) ||
        (m.mode ?? "").toLowerCase().includes(q)
    );
  }, [models, search]);

  const visibleModels = filteredModels.slice(0, 200);
  const providerCount = useMemo(() => new Set(models.map((model) => model.litellm_provider).filter(Boolean)).size, [models]);
  const visionCount = filteredModels.filter((model) => model.supports_vision).length;
  const functionCount = filteredModels.filter((model) => model.supports_function_calling).length;
  const reasoningCount = filteredModels.filter((model) => model.supports_reasoning).length;
  const cacheCount = filteredModels.filter((model) => hasCachePricing(model)).length;
  const activeModeLabel = modeOptions.find((option) => option.value === modeFilter)?.label ?? "全部";

  const fmtCost = (val: number) => {
    if (!val) return "—";
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

  const statCards = [
    {
      label: "价格库",
      value: (status?.total_models ?? models.length).toLocaleString(),
      detail: status?.last_sync ? `同步 ${fmtDate(status.last_sync)}` : "本地缓存状态",
      tone: "teal",
      icon: Database
    },
    {
      label: "当前列表",
      value: filteredModels.length.toLocaleString(),
      detail: activeModeLabel,
      tone: "blue",
      icon: TrendingUp
    },
    {
      label: "能力覆盖",
      value: `${visionCount}/${functionCount}/${reasoningCount}`,
      detail: "视觉 / 函数 / 推理",
      tone: "amber",
      icon: CheckCircle
    },
    {
      label: "缓存计价",
      value: cacheCount.toLocaleString(),
      detail: `${providerCount.toLocaleString()} 个 Provider`,
      tone: "violet",
      icon: DollarSign
    }
  ];

  return (
    <div className="pricing-page flex-col gap-4">
      <section className="pricing-hero">
        <div className="pricing-hero-copy">
          <span className="eyebrow">Billing Catalog</span>
          <h2>模型定价矩阵</h2>
          <p>统一查看输入、输出、上下文与能力标签，把“这个模型贵不贵、能不能缓存、是否适合当前请求”讲清楚。</p>
        </div>
        <div className="pricing-hero-actions">
          <button type="button" className="btn btn-primary btn-sm" onClick={handleRefresh} disabled={syncing}>
            <RefreshCw size={14} className={syncing ? "spin" : ""} />
            {syncing ? "同步中..." : "刷新定价"}
          </button>
        </div>
      </section>

      <section className="pricing-summary-grid" aria-label="定价概览">
        {statCards.map((card) => {
          const Icon = card.icon;
          return (
            <article key={card.label} className={`pricing-stat-card ${card.tone}`}>
              <span className="pricing-stat-icon"><Icon size={16} /></span>
              <div>
                <span>{card.label}</span>
                <strong>{card.value}</strong>
                <em>{card.detail}</em>
              </div>
            </article>
          );
        })}
      </section>

      <div className="pricing-tabs tabs" role="tablist" aria-label="定价工具">
        <button type="button" role="tab" aria-selected={activeTab === "browse"} className={`tab ${activeTab === "browse" ? "active" : ""}`} onClick={() => setActiveTab("browse")}>
          定价浏览
        </button>
        <button type="button" role="tab" aria-selected={activeTab === "lookup"} className={`tab ${activeTab === "lookup" ? "active" : ""}`} onClick={() => setActiveTab("lookup")}>
          模型查询
        </button>
        <button type="button" role="tab" aria-selected={activeTab === "estimate"} className={`tab ${activeTab === "estimate" ? "active" : ""}`} onClick={() => setActiveTab("estimate")}>
          费用估算
        </button>
      </div>

      {activeTab === "browse" && (
        <section className="pricing-workbench">
          <div className="pricing-toolbar">
            <label className="pricing-search-field">
              <Search size={15} />
              <input
                className="form-control"
                aria-label="搜索模型名、Provider 或类型"
                placeholder="搜索模型名、Provider 或类型..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </label>

            <div className="pricing-mode-filter" aria-label="模型类型筛选">
              {modeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className={modeFilter === option.value ? "active" : ""}
                  onClick={() => setModeFilter(option.value)}
                >
                  {option.shortLabel}
                </button>
              ))}
            </div>
          </div>

          {loading ? (
            <div className="pricing-loading">
              <Loader2 size={22} className="spin" />
              <span>正在读取价格库</span>
            </div>
          ) : (
            <div className="pricing-table-shell">
              <table className="pricing-table">
                <thead>
                  <tr>
                    <th scope="col">模型</th>
                    <th scope="col">Provider</th>
                    <th scope="col">类型</th>
                    <th scope="col">上下文</th>
                    <th scope="col" className="numeric">输入 /M</th>
                    <th scope="col" className="numeric">输出 /M</th>
                    <th scope="col">缓存</th>
                    <th scope="col">能力</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleModels.length === 0 ? (
                    <tr>
                      <td colSpan={8}>
                        <div className="pricing-empty">暂无匹配的定价数据。请清空搜索词或切换模型类型。</div>
                      </td>
                    </tr>
                  ) : (
                    visibleModels.map((model) => {
                      const modeClass = getModeClass(model.mode);
                      return (
                        <tr key={model.model_id}>
                          <td className="pricing-model-column">
                            <div className="pricing-model-cell">
                              <span className={`pricing-mode-dot ${modeClass}`} />
                              <div>
                                <code>{model.model_id}</code>
                                <span>更新 {fmtDate(model.fetched_at)}</span>
                              </div>
                            </div>
                          </td>
                          <td>
                            <span className="pricing-provider">{model.litellm_provider || "未标注"}</span>
                          </td>
                          <td>
                            <span className={`pricing-badge ${modeClass}`}>{modeLabel(model.mode)}</span>
                          </td>
                          <td className="pricing-context-cell">
                            <strong>{formatTokens(model.max_input_tokens)}</strong>
                            <span>{formatTokens(model.max_output_tokens)} 输出</span>
                            <span>{pricingTierSummary(model.pricing_json)}</span>
                          </td>
                          <td className="numeric price-cell">{fmtCost(model.input_cost_per_token)}</td>
                          <td className="numeric price-cell output">{fmtCost(model.output_cost_per_token)}</td>
                          <td className="pricing-cache-cell">
                            {hasCachePricing(model) ? (
                              <>
                                <span>创建 {fmtCost(model.cache_creation_cost_per_token)}</span>
                                <span>读取 {fmtCost(model.cache_read_cost_per_token)}</span>
                              </>
                            ) : (
                              <span className="pricing-muted">未标注</span>
                            )}
                          </td>
                          <td>
                            <div className="pricing-capability-list">
                              {model.supports_vision && <span className="pricing-capability positive"><Eye size={12} />视觉</span>}
                              {model.supports_function_calling && <span className="pricing-capability info"><CheckCircle size={12} />函数</span>}
                              {model.supports_prompt_caching && <span className="pricing-capability amber"><Database size={12} />缓存</span>}
                              {model.supports_reasoning && <span className="pricing-capability info"><CheckCircle size={12} />推理</span>}
                              {!model.supports_vision && !model.supports_function_calling && !model.supports_prompt_caching && !model.supports_reasoning && (
                                <span className="pricing-muted">基础</span>
                              )}
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>

              {filteredModels.length > 200 && (
                <div className="pricing-table-note">
                  已展示前 200 条，共 {filteredModels.length.toLocaleString()} 条。可继续搜索缩小范围。
                </div>
              )}
            </div>
          )}
        </section>
      )}

      {activeTab === "lookup" && (
        <section className="pricing-tool-panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">Precise Lookup</span>
              <h3 className="section-title">模型查询</h3>
              <p className="section-description">用于确认某个模型是否有精确价格，若没有则显示兜底价格来源。</p>
            </div>
          </div>

          <div className="pricing-action-row">
            <input
              className="form-control"
              aria-label="输入模型 ID 查询定价"
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
            <PricingLookupResult result={lookupResult} fmtCost={fmtCost} />
          )}
        </section>
      )}

      {activeTab === "estimate" && (
        <section className="pricing-tool-panel">
          <div className="section-header">
            <div className="section-header-main">
              <span className="eyebrow">Cost Estimate</span>
              <h3 className="section-title">请求费用估算</h3>
              <p className="section-description">输入模型和 Token 数，即可估算一次请求的大致费用。</p>
            </div>
          </div>

          <div className="pricing-estimate-form">
            <div className="form-field">
              <label className="form-label">模型</label>
              <input className="form-control" placeholder="gpt-4o" value={estModel} onChange={(e) => setEstModel(e.target.value)} />
            </div>
            <div className="form-field">
              <label className="form-label">输入 Tokens</label>
              <input className="form-control" type="number" min={0} value={estInput} onChange={(e) => setEstInput(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">输出 Tokens</label>
              <input className="form-control" type="number" min={0} value={estOutput} onChange={(e) => setEstOutput(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">Cache write</label>
              <input className="form-control" type="number" min={0} value={estCacheCreation} onChange={(e) => setEstCacheCreation(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">Cache read</label>
              <input className="form-control" type="number" min={0} value={estCacheRead} onChange={(e) => setEstCacheRead(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">Reasoning Tokens</label>
              <input className="form-control" type="number" min={0} value={estReasoning} onChange={(e) => setEstReasoning(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">Context window</label>
              <input className="form-control" type="number" min={0} placeholder="auto" value={estContextWindow} onChange={(e) => setEstContextWindow(Number(e.target.value))} />
            </div>
            <div className="form-field">
              <label className="form-label">Tier / effort</label>
              <input className="form-control" placeholder="priority / high" value={estPricingTier} onChange={(e) => setEstPricingTier(e.target.value)} />
            </div>
            <button type="button" className="btn btn-primary btn-sm" onClick={handleEstimate}>
              <DollarSign size={14} /> 估算
            </button>
          </div>

          {estResult && (
            <div className="pricing-result-card estimate">
              {estResult.fallback_used && (
                <div className="pricing-alert warning">
                  <AlertCircle size={14} />
                  <span>未匹配到 <code>{estResult.model_id}</code>，使用 <code>{estResult.fallback_model}</code> 兜底</span>
                </div>
              )}
              <div className="pricing-result-grid">
                <MetricTile label="预估费用" value={fmtUSD(estResult.estimated_usd)} tone="strong" />
                <MetricTile label="输入 Tokens" value={estResult.input_tokens.toLocaleString()} />
                <MetricTile label="输出 Tokens" value={estResult.output_tokens.toLocaleString()} />
                <MetricTile label="Cache read/write" value={`${estResult.cache_read_tokens.toLocaleString()} / ${estResult.cache_creation_tokens.toLocaleString()}`} />
                <MetricTile label="Reasoning" value={`${estResult.reasoning_tokens.toLocaleString()} tokens · ${fmtUSD(estResult.cost_breakdown?.reasoning_usd ?? 0)}`} />
                <MetricTile label="Context / tier" value={`${formatTokens(estResult.context_window)} / ${estResult.pricing_tier || "base"}`} />
                <MetricTile label="匹配状态" value={estResult.matched ? "精确匹配" : "兜底"} />
              </div>
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function PricingLookupResult({ result, fmtCost }: { result: LookupResult; fmtCost: (value: number) => string }) {
  if (!result.data) {
    return (
      <div className="pricing-result-card">
        <div className="pricing-empty">未找到定价信息</div>
      </div>
    );
  }

  return (
    <div className="pricing-result-card">
      {result.fallback_used && (
        <div className="pricing-alert warning">
          <AlertCircle size={14} />
          <span>未找到精确匹配，已使用 <code>{result.fallback_model}</code> 价格兜底</span>
        </div>
      )}
      {result.matched && (
        <div className="pricing-alert success">
          <CheckCircle size={14} />
          <span>精确匹配成功</span>
        </div>
      )}
      <div className="pricing-result-grid">
        <MetricTile label="模型" value={result.data.model_id} mono />
        <MetricTile label="Provider" value={result.data.litellm_provider || "未标注"} />
        <MetricTile label="类型" value={modeLabel(result.data.mode)} />
        <MetricTile label="输入价格" value={`${fmtCost(result.data.input_cost_per_token)} /M`} />
        <MetricTile label="输出价格" value={`${fmtCost(result.data.output_cost_per_token)} /M`} />
        {result.data.max_input_tokens > 0 && <MetricTile label="最大输入" value={formatTokens(result.data.max_input_tokens)} />}
        {result.data.max_output_tokens > 0 && <MetricTile label="最大输出" value={formatTokens(result.data.max_output_tokens)} />}
        <MetricTile label="推理支持" value={result.data.supports_reasoning ? "支持" : "未标注"} />
        <MetricTile label="分段价" value={pricingTierSummary(result.data.pricing_json)} />
      </div>
    </div>
  );
}

function MetricTile({ label, value, mono, tone }: { label: string; value: string; mono?: boolean; tone?: "strong" }) {
  return (
    <div className={`pricing-metric-tile ${tone ?? ""}`}>
      <span>{label}</span>
      <strong className={mono ? "mono" : ""}>{value}</strong>
    </div>
  );
}

function modeLabel(mode: string) {
  return modeLabelMap[mode] ?? (mode || "未标注");
}

function getModeClass(mode: string) {
  return (mode || "unknown").replace(/_/g, "-").replace(/[^a-z0-9-]/gi, "").toLowerCase() || "unknown";
}

function formatTokens(value: number) {
  if (!value) return "—";
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(value % 1_000_000 === 0 ? 0 : 1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(value % 1_000 === 0 ? 0 : 1)}K`;
  return value.toLocaleString();
}

function pricingTierSummary(raw: string) {
  if (!raw) return "base tier";
  try {
    const data = JSON.parse(raw) as Record<string, unknown>;
    const keys = Object.keys(data);
    const contextTiers = Array.from(new Set(keys.flatMap((key) => {
      const match = key.match(/above_(\d+k)_tokens/i);
      return match ? [match[1]] : [];
    })));
    const serviceTiers = Array.from(new Set(keys.flatMap((key) => {
      const match = key.match(/_(priority|flex|minimal|low|medium|high|xhigh|none)$/i);
      return match ? [match[1]] : [];
    })));
    const parts = [...contextTiers, ...serviceTiers];
    return parts.length ? parts.slice(0, 3).join(" / ") : "base tier";
  } catch {
    return "base tier";
  }
}

function hasCachePricing(model: ModelPricing) {
  return model.supports_prompt_caching || model.cache_creation_cost_per_token > 0 || model.cache_read_cost_per_token > 0;
}
