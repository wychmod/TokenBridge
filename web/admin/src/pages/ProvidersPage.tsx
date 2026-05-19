import { useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Eye, EyeOff, Loader2, Plus, RefreshCcw, Trash2, Wifi, X } from "lucide-react";
import clsx from "clsx";
import { useAdminStore } from "../store/admin-store";
import { providerStatusMap } from "../store/labels";
import type { ProviderRecord } from "../store/entities";
import type { ProviderModelDiscoveryResult, ProviderTestResult } from "../store/admin-store";

const providerTypeOptions = [
  { value: "openai", label: "OpenAI", hint: "官方 Chat Completions", defaultBaseURL: "https://api.openai.com", modelPlaceholder: "gpt-4o, gpt-4o-mini, o3-mini" },
  { value: "openai-compatible", label: "OpenAI 兼容", hint: "DeepSeek / OpenRouter / 硅基流动等", defaultBaseURL: "https://api.deepseek.com", modelPlaceholder: "deepseek-chat, deepseek-reasoner" },
  { value: "anthropic", label: "Anthropic (Claude)", hint: "官方 Messages API", defaultBaseURL: "https://api.anthropic.com", modelPlaceholder: "claude-sonnet-4, claude-haiku-4" }
] as const;

type DrawerNotice = {
  tone: "success" | "warning";
  title: string;
  message: string;
};

const emptyProvider = (count: number): ProviderRecord => ({
  id: `prov-${Date.now()}`,
  name: "新供应商",
  type: "openai-compatible",
  baseURL: "https://api.deepseek.com",
  apiKey: "",
  hasApiKey: false,
  apiKeyMasked: "",
  enabled: true,
  status: "healthy" as const,
  priority: count + 1,
  models: [],
  rpm: 60,
  tpm: 120000
});

export function ProvidersPage() {
  const {
    providers,
    selectedProviderId,
    setSelectedProvider,
    saveProvider,
    deleteProvider,
    reorderProviders,
    testProvider,
    testProviderDraft,
    discoverProviderModelsDraft,
  } = useAdminStore();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [form, setForm] = useState<ProviderRecord>(emptyProvider(providers.length));
  const [showApiKey, setShowApiKey] = useState(false);
  const [busyAction, setBusyAction] = useState<"save" | "test" | "discover" | "delete" | null>(null);
  const [testingProviderId, setTestingProviderId] = useState<string | null>(null);
  const [drawerNotice, setDrawerNotice] = useState<DrawerNotice | null>(null);

  const active = useMemo(
    () => providers.find((item) => item.id === selectedProviderId),
    [providers, selectedProviderId]
  );

  useEffect(() => {
    if (active && !drawerOpen) {
      setForm({ ...active, apiKey: "", type: normalizeProviderType(active.type) });
    }
  }, [active, drawerOpen]);

  const openDrawer = (provider?: ProviderRecord) => {
    if (provider) {
      const normalized = { ...provider, apiKey: "", type: normalizeProviderType(provider.type) };
      setForm(normalized);
      setSelectedProvider(provider.id);
    } else {
      const next = emptyProvider(providers.length);
      setForm(next);
      setSelectedProvider(next.id);
    }
    setDrawerOpen(true);
    setShowApiKey(false);
    setDrawerNotice(null);
  };

  const closeDrawer = () => {
    setDrawerOpen(false);
    setShowApiKey(false);
    setBusyAction(null);
    setDrawerNotice(null);
  };

  const handleSave = async () => {
    setBusyAction("save");
    await saveProvider({ ...form, type: normalizeProviderType(form.type) });
    setBusyAction(null);
    closeDrawer();
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定删除此供应商？删除后相关路由和密钥可能无法再命中该上游线路。")) return;
    setBusyAction("delete");
    await deleteProvider(id);
    setBusyAction(null);
    if (form.id === id) closeDrawer();
  };

  const handleTest = async () => {
    setBusyAction("test");
    const result = await testProviderDraft({ ...form, type: normalizeProviderType(form.type) });
    setDrawerNotice(providerTestNotice(result));
    setBusyAction(null);
  };

  const handleSavedProviderTest = async (id: string) => {
    setTestingProviderId(id);
    await testProvider(id);
    setTestingProviderId(null);
  };

  const handleDiscover = async () => {
    setBusyAction("discover");
    const result = await discoverProviderModelsDraft({ ...form, type: normalizeProviderType(form.type) });
    setBusyAction(null);
    setDrawerNotice(providerModelDiscoveryNotice(result));
    if (result.models.length) setForm({ ...form, models: result.models });
  };

  const moveProvider = (id: string, direction: -1 | 1) => {
    const currentIndex = providers.findIndex((provider) => provider.id === id);
    const nextIndex = currentIndex + direction;
    if (currentIndex < 0 || nextIndex < 0 || nextIndex >= providers.length) return;
    const next = [...providers];
    [next[currentIndex], next[nextIndex]] = [next[nextIndex], next[currentIndex]];
    void reorderProviders(next.map((provider) => provider.id));
  };

  const handleTypeChange = (nextType: string) => {
    const normalizedCurrent = normalizeProviderType(form.type);
    const nextOption = providerTypeOptions.find((option) => option.value === nextType);
    setForm({
      ...form,
      type: nextType,
      baseURL: shouldAutoReplaceBaseURL(form.baseURL, normalizedCurrent)
        ? nextOption?.defaultBaseURL ?? form.baseURL
        : form.baseURL,
    });
  };

  const normalizedFormType = normalizeProviderType(form.type);
  const selectedType = providerTypeOptions.find((option) => option.value === normalizedFormType) ?? providerTypeOptions[1];
  const modelsText = form.models.join(", ");
  const savedTokenText = form.apiKeyMasked || form.api_key_masked || "";
  const hasSavedToken = form.hasApiKey === true || form.has_api_key === true || savedTokenText !== "";
  const hasUnsavedChanges = JSON.stringify(comparableProvider(form)) !== JSON.stringify(active ? comparableProvider(active) : {});

  return (
    <div className="flex-col gap-4">
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">模型接入</span>
          <h2 className="section-title">供应商管理</h2>
          <p className="section-description">供应商是 TokenBridge 连接上游模型服务的出口。先看健康状态，再调整优先级、模型列表和速率限制。</p>
        </div>
        <div className="section-actions">
          <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
            <Plus size={14} /> 新建供应商
          </button>
        </div>
      </div>

      <div className="flex items-center gap-2">
        <span className="pill pill-neutral">总数 {providers.length}</span>
        <span className="pill pill-success">在线 {providers.filter((p) => p.status === "healthy").length}</span>
        <span className="pill pill-warning">异常 {providers.filter((p) => p.status === "warning").length}</span>
        <span className="pill pill-neutral">已停用 {providers.filter((p) => p.status === "disabled" || p.enabled === false).length}</span>
      </div>

      <div className="flex-col gap-2 list-animate">
        {providers.map((provider, index) => (
          <div
            key={provider.id}
            className={clsx("list-row", selectedProviderId === provider.id && "active")}
            style={{ "--index": index } as React.CSSProperties}
            role="button"
            tabIndex={0}
            aria-label={`编辑 ${provider.name}，状态 ${providerStatusMap[provider.status] ?? provider.status}`}
            onClick={() => openDrawer(provider)}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                openDrawer(provider);
              }
            }}
          >
            <div className="list-row-main">
              <div className="flex items-center gap-2">
                <div
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: "50%",
                    background:
                      provider.status === "healthy"
                        ? "var(--accent)"
                        : provider.status === "warning"
                        ? "var(--warning)"
                        : "var(--text-tertiary)",
                    flexShrink: 0
                  }}
                />
                <span className="list-row-title">{provider.name}</span>
                <span
                  className="pill"
                  style={{
                    background:
                      provider.status === "healthy"
                        ? "var(--accent-dim)"
                        : provider.status === "warning"
                        ? "var(--warning-dim)"
                        : "var(--bg-elevated)",
                    color:
                      provider.status === "healthy"
                        ? "var(--accent)"
                        : provider.status === "warning"
                        ? "var(--warning)"
                        : "var(--text-tertiary)"
                  }}
                >
                  {providerStatusMap[provider.status] ?? provider.status}
                </span>
              </div>
              <span className="list-row-meta">
                {providerTypeLabel(provider.type)} · {provider.baseURL}
              </span>
              <span className="list-row-sub">
                {provider.models.length} 个模型 · {provider.rpm} RPM · {provider.tpm.toLocaleString()} TPM · 优先级 #{provider.priority}
              </span>
            </div>
            <div className="list-row-actions">
              <button type="button" className="btn btn-ghost btn-sm" disabled={index === 0} onClick={(e) => { e.stopPropagation(); moveProvider(provider.id, -1); }} title="上移" aria-label={`上移 ${provider.name}`}>
                <ArrowUp size={14} />
              </button>
              <button type="button" className="btn btn-ghost btn-sm" disabled={index === providers.length - 1} onClick={(e) => { e.stopPropagation(); moveProvider(provider.id, 1); }} title="下移" aria-label={`下移 ${provider.name}`}>
                <ArrowDown size={14} />
              </button>
              <button type="button" className="btn btn-secondary btn-sm" onClick={(e) => { e.stopPropagation(); openDrawer(provider); }}>
                编辑
              </button>
              <button type="button" className="btn btn-secondary btn-sm" onClick={(e) => { e.stopPropagation(); void handleSavedProviderTest(provider.id); }} disabled={testingProviderId === provider.id} aria-label={`测试 ${provider.name} 连接`}>
                {testingProviderId === provider.id ? <Loader2 size={14} className="spin" /> : <Wifi size={14} />}
                测试连接
              </button>
              <button type="button" className="btn btn-danger btn-sm" onClick={(e) => { e.stopPropagation(); void handleDelete(provider.id); }} aria-label={`删除 ${provider.name}`}>
                <Trash2 size={14} />
              </button>
            </div>
          </div>
        ))}

        {providers.length === 0 && (
          <div className="empty-state">
            <span className="empty-state-title">暂无供应商</span>
            <span className="empty-state-desc">添加第一个供应商后，应用就可以通过本地网关转发模型请求。</span>
            <div className="empty-state-actions">
              <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
                <Plus size={14} /> 新建供应商
              </button>
            </div>
          </div>
        )}
      </div>

      {drawerOpen && (
        <>
          <div className="drawer-overlay" onClick={closeDrawer} />
          <aside className="drawer drawer-wide" role="dialog" aria-modal="true" aria-label="供应商配置">
            <div className="drawer-header">
              <div>
                <h3 className="section-title">{form.name}</h3>
                <p style={{ fontSize: "0.82rem", color: "var(--text-secondary)", marginTop: 2 }}>
                  {hasUnsavedChanges ? "有未保存的变更" : "供应商详情与配置"}
                </p>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={closeDrawer} aria-label="关闭供应商配置">
                <X size={16} />
              </button>
            </div>

            {drawerNotice && (
              <div className={clsx("drawer-local-toast", drawerNotice.tone)} role="status">
                <strong>{drawerNotice.title}</strong>
                <span>{drawerNotice.message}</span>
              </div>
            )}

            <div className="drawer-body">
              <div className="form-grid">
                <div className="form-field span-2">
                  <label className="form-label">供应商名称</label>
                  <input className="form-control" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="例如：OpenAI 主线路" />
                </div>

                <div className="form-field span-2">
                  <label className="form-label">接入类型</label>
                  <div className="radio-card-grid">
                    {providerTypeOptions.map((option) => (
                      <label
                        key={option.value}
                        className={clsx("radio-card", normalizedFormType === option.value && "active")}
                      >
                        <input
                          type="radio"
                          name="provider-type"
                          value={option.value}
                          checked={normalizedFormType === option.value}
                          onChange={() => handleTypeChange(option.value)}
                          style={{ position: "absolute", opacity: 0 }}
                        />
                        <strong style={{ fontSize: "0.85rem" }}>{option.label}</strong>
                        <span style={{ fontSize: "0.75rem", color: "var(--text-tertiary)" }}>{option.hint}</span>
                      </label>
                    ))}
                  </div>
                </div>

                <div className="form-field span-2">
                  <label className="form-label">接口地址</label>
                  <input
                    className="form-control"
                    type="url"
                    value={form.baseURL}
                    onChange={(e) => setForm({ ...form, baseURL: e.target.value })}
                    placeholder={selectedType.defaultBaseURL}
                    spellCheck={false}
                  />
                  <span className="form-hint">填写官方或兼容供应商的基础地址即可，系统会按请求格式追加官方路径。</span>
                </div>

                <div className="form-field span-2">
                  <div className="form-label-row">
                    <label className="form-label">API Key / Token</label>
                    {hasSavedToken && <span className="secret-saved-label">已保存：{savedTokenText || "已保存 Token"}</span>}
                  </div>
                  <div className="secret-input">
                    <input
                      className="form-control"
                      type={showApiKey ? "text" : "password"}
                      value={form.apiKey ?? ""}
                      onChange={(e) => setForm({ ...form, apiKey: e.target.value })}
                      placeholder={hasSavedToken ? "留空不修改" : normalizedFormType === "anthropic" ? "sk-ant-..." : "sk-..."}
                      autoComplete="off"
                      spellCheck={false}
                    />
                    <button type="button" className="btn btn-ghost btn-icon" onClick={() => setShowApiKey((v) => !v)} aria-label={showApiKey ? "隐藏本次输入的 Token" : "显示本次输入的 Token"}>
                      {showApiKey ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                  <span className="form-hint">
                    {hasSavedToken ? "已保存 Token，留空不会修改；输入新 Token 后仅显示本次输入内容。" : "Token 不会明文回显，保存后只显示安全掩码。"}
                  </span>
                </div>

                <div className="form-field">
                  <label className="form-label">每分钟请求数 (RPM)</label>
                  <input className="form-control" type="number" min={0} step={1} value={form.rpm} onChange={(e) => setForm({ ...form, rpm: Number(e.target.value) })} />
                </div>

                <div className="form-field">
                  <label className="form-label">每分钟令牌数 (TPM)</label>
                  <input className="form-control" type="number" min={0} step={1000} value={form.tpm} onChange={(e) => setForm({ ...form, tpm: Number(e.target.value) })} />
                </div>

                <div className="form-field span-2">
                  <label className="form-label">支持模型</label>
                  <textarea
                    className="form-control"
                    value={modelsText}
                    onChange={(e) => setForm({ ...form, models: e.target.value.split(/[\n,]/).map((s) => s.trim()).filter(Boolean) })}
                    placeholder={`例如：${selectedType.modelPlaceholder}`}
                    rows={3}
                  />
                  <span className="form-hint">支持逗号或换行分隔。Claude 模型应走 Anthropic 类型；DeepSeek/OpenRouter 等走 OpenAI 兼容。</span>
                </div>

                <div className="form-field span-2">
                  <label className="form-label" style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
                    <input
                      type="checkbox"
                      checked={form.enabled !== false && form.status !== "disabled"}
                      onChange={(e) => setForm({ ...form, enabled: e.target.checked, status: e.target.checked ? "healthy" : "disabled" })}
                    />
                    <span>启用此供应商</span>
                  </label>
                  <span className="form-hint">停用后不会再作为主链路或备用链路参与真实请求。</span>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <span className="pill pill-neutral">状态 {providerStatusMap[form.status] ?? form.status}</span>
                <span className="pill pill-neutral">模型 {form.models.length}</span>
                <span className="pill pill-neutral">优先级 {form.priority}</span>
              </div>
            </div>

            <div className="drawer-footer">
              <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={busyAction !== null}>
                {busyAction === "save" ? <Loader2 size={14} className="spin" /> : null}
                保存配置
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => void handleTest()} disabled={busyAction !== null}>
                {busyAction === "test" ? <Loader2 size={14} className="spin" /> : <Wifi size={14} />} 测试连接
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => void handleDiscover()} disabled={busyAction !== null}>
                {busyAction === "discover" ? <Loader2 size={14} className="spin" /> : <RefreshCcw size={14} />} 发现模型
              </button>
              <button type="button" className="btn btn-danger" onClick={() => void handleDelete(form.id)} disabled={busyAction !== null}>
                {busyAction === "delete" ? <Loader2 size={14} className="spin" /> : <Trash2 size={14} />} 删除
              </button>
            </div>
          </aside>
        </>
      )}
    </div>
  );
}

function normalizeProviderType(type: string): string {
  const value = type.trim().toLowerCase();
  if (["openai", "openai官方", "openai 官方"].includes(value)) return "openai";
  if (["openai-compatible", "openai compatible", "openai 兼容", "openai兼容", "deepseek", "deepseek 兼容", "deepseek兼容"].includes(value)) return "openai-compatible";
  if (["anthropic", "claude", "anthropic compatible", "anthropic 兼容", "anthropic兼容", "anthropic official", "anthropic 官方", "anthropic官方"].includes(value)) return "anthropic";
  return type;
}

function providerTypeLabel(type: string): string {
  const normalized = normalizeProviderType(type);
  return providerTypeOptions.find((option) => option.value === normalized)?.label ?? type;
}

function shouldAutoReplaceBaseURL(baseURL: string, currentType: string): boolean {
  const value = baseURL.trim();
  if (!value || value === "https://") return true;
  return providerTypeOptions.some((option) => value === option.defaultBaseURL || value === `${option.defaultBaseURL}/`);
}

function providerTestNotice(result: ProviderTestResult): DrawerNotice {
  const healthy = result.status === "healthy";
  const latency = Number.isFinite(result.latency_ms) ? result.latency_ms : 0;
  return {
    tone: healthy ? "success" : "warning",
    title: healthy ? "连接测试通过" : "连接测试失败",
    message: `${result.message || "测试请求已返回。"}，延迟 ${latency} 毫秒。`
  };
}

function providerModelDiscoveryNotice(result: ProviderModelDiscoveryResult): DrawerNotice {
  if (!result.ok) {
    return {
      tone: "warning",
      title: "模型发现失败",
      message: result.error ?? "无法读取模型列表。"
    };
  }
  if (result.models.length === 0) {
    return {
      tone: "warning",
      title: "未发现模型",
      message: "供应商返回了空模型列表，请检查接口地址或权限。"
    };
  }
  return {
    tone: "success",
    title: "模型发现完成",
    message: `共发现 ${result.models.length} 个模型，已填入支持模型。`
  };
}

function comparableProvider(record: ProviderRecord) {
  return {
    name: record.name,
    type: normalizeProviderType(record.type),
    baseURL: record.baseURL,
    organization_id: record.organization_id ?? "",
    enabled: record.enabled,
    status: record.status,
    priority: record.priority,
    models: record.models,
    rpm: record.rpm,
    tpm: record.tpm,
    apiKeyChanged: Boolean(record.apiKey?.trim())
  };
}
