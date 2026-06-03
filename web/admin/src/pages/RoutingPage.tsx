import { useMemo, useState } from "react";
import { Activity, ArrowDown, ArrowUp, Plus, Route as RouteIcon, Save, Trash2, X } from "lucide-react";
import clsx from "clsx";
import { useAdminStore } from "../store/admin-store";
import type { ModelAliasRecord, ProviderRecord, RoutingRuleRecord, RoutingSimulation } from "../store/entities";
import {
  addProviderToChain,
  moveProviderInChain,
  normalizeProviderRefs,
  providerLabel,
  providerValue,
  removeProviderFromChain
} from "../utils/routing-ui";

type RoutingRuleForm = RoutingRuleRecord;

const SIMULATION_SCOPE = "仅模拟路由决策，未发送上游请求。";

function createEmptyRule(providers: ProviderRecord[] = []): RoutingRuleForm {
  const primary = firstUsableProvider(providers);
  const fallback = providers.find((provider) => provider.id !== primary?.id && isUsableProvider(provider));
  return {
    id: `route-${Date.now()}`,
    modelPattern: firstModelPattern(providers),
    strategy: "priority",
    providerChain: primary ? [primary.id] : [],
    fallbackChain: fallback ? [fallback.id] : [],
    enabled: true
  };
}

function createEmptyAlias(providers: ProviderRecord[] = []): ModelAliasRecord {
  return {
    id: `alias-${Date.now()}`,
    alias: "gpt-fast",
    target: firstModel(providers) ?? "gpt-4o-mini",
    fallbackChain: []
  };
}

function createInitialSimulation(model = "gpt-4o"): RoutingSimulation {
  return {
    model,
    requestedModel: model,
    key: "未校验 Local Key",
    format: "openai",
    target: "等待模拟",
    fallback: "等待模拟",
    cost: "未实际请求",
    ttft: "未实际请求",
    formatCompatible: true,
    scope: SIMULATION_SCOPE
  };
}

export function RoutingPage() {
  const { providers, rules, aliases, saveRule, deleteRule, saveAlias, testRouting } = useAdminStore();
  const providerOptions = useMemo(() => providers, [providers]);

  const [viewMode, setViewMode] = useState<"rules" | "aliases">("rules");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [aliasDrawerOpen, setAliasDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<"edit" | "test">("edit");
  const [form, setForm] = useState<RoutingRuleForm>(() => createEmptyRule(providerOptions));
  const [aliasForm, setAliasForm] = useState<ModelAliasRecord>(() => createEmptyAlias(providerOptions));
  const [busyAction, setBusyAction] = useState<"save" | "delete" | "test" | "alias" | null>(null);
  const [simulation, setSimulation] = useState<RoutingSimulation>(() => createInitialSimulation());

  const enabledRules = useMemo(() => rules.filter((rule) => rule.enabled).length, [rules]);
  const fallbackRules = useMemo(() => rules.filter((rule) => rule.fallbackChain.length > 0).length, [rules]);
  const hasProviders = providerOptions.length > 0;
  const formHasUnknownProviders = hasUnknownProviderRefs([...form.providerChain, ...form.fallbackChain], providerOptions);
  const aliasHasUnknownProviders = hasUnknownProviderRefs(aliasForm.fallbackChain, providerOptions);

  const openDrawer = (rule?: RoutingRuleRecord, mode: "edit" | "test" = "edit") => {
    const next = normalizeRuleForm(rule ?? createEmptyRule(providerOptions), providerOptions);
    setForm(next);
    setSimulation(createInitialSimulation(sampleModelFromPattern(next.modelPattern)));
    setDrawerMode(mode);
    setDrawerOpen(true);
  };

  const openAliasDrawer = (alias?: ModelAliasRecord) => {
    const next = alias ?? createEmptyAlias(providerOptions);
    setAliasForm({ ...next, fallbackChain: normalizeProviderRefs(next.fallbackChain, providerOptions) });
    setAliasDrawerOpen(true);
  };

  const closeDrawer = () => {
    setDrawerOpen(false);
    setBusyAction(null);
  };

  const handleSave = async () => {
    setBusyAction("save");
    await saveRule(form);
    setBusyAction(null);
    closeDrawer();
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定删除此路由规则？删除后匹配该模型的请求会回退到默认 Provider 解析逻辑。")) return;
    setBusyAction("delete");
    await deleteRule(id);
    setBusyAction(null);
    if (form.id === id) closeDrawer();
  };

  const handleAliasSave = async () => {
    setBusyAction("alias");
    await saveAlias(aliasForm);
    setBusyAction(null);
    setAliasDrawerOpen(false);
  };

  const runSimulation = async () => {
    setBusyAction("test");
    const result = await testRouting({
      model: simulation.model,
      localKey: simulation.key,
      format: simulation.format
    });
    setBusyAction(null);
    if (result) setSimulation(result);
  };

  return (
    <div className="flex-col gap-4">
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">调度策略</span>
          <h2 className="section-title">路由策略</h2>
          <p className="section-description">请求会先解析模型别名，再用解析后的模型名匹配路由规则，最后按主链路和备用链路选择 Provider。</p>
        </div>
        <div className="section-actions">
          <button type="button" className="btn btn-secondary btn-sm" onClick={() => openAliasDrawer()}>
            <Plus size={14} /> 新建别名
          </button>
          <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
            <Plus size={14} /> 新建规则
          </button>
        </div>
      </div>

      <div className="routing-flow" aria-label="路由决策顺序">
        <FlowStep index="1" title="模型别名" detail="改写模型名" />
        <FlowStep index="2" title="路由规则" detail="匹配解析后模型" />
        <FlowStep index="3" title="主链路" detail="选择首个可用 Provider" />
        <FlowStep index="4" title="备用链路" detail="限流或故障后尝试" />
      </div>

      <div className="flex items-center gap-2" style={{ flexWrap: "wrap" }}>
        <span className="pill pill-neutral">规则 {rules.length}</span>
        <span className="pill pill-success">已启用 {enabledRules}</span>
        <span className="pill pill-warning">Fallback {fallbackRules}</span>
        <span className="pill pill-info">模型别名 {aliases.length}</span>
        <span className={clsx("pill", hasProviders ? "pill-success" : "pill-warning")}>Provider {providerOptions.length}</span>
      </div>

      {!hasProviders && (
        <div className="alert-card">
          <strong>还没有可选择的 Provider</strong>
          <p>主链路和备用链路只能从已配置的 Provider 中选择。先在供应商页面添加 Provider 后再创建规则。</p>
        </div>
      )}

      <div className="tabs" style={{ alignSelf: "flex-start" }}>
        <button type="button" className={clsx("tab", viewMode === "rules" && "active")} onClick={() => setViewMode("rules")}>
          路由规则
        </button>
        <button type="button" className={clsx("tab", viewMode === "aliases" && "active")} onClick={() => setViewMode("aliases")}>
          模型别名
        </button>
      </div>

      {viewMode === "rules" ? (
        <div className="flex-col gap-2 list-animate">
          {rules.map((rule, index) => {
            const normalizedRule = normalizeRuleForm(rule, providerOptions);
            return (
              <div
                key={rule.id}
                className="list-row"
                style={{ "--index": index } as React.CSSProperties}
                role="button"
                tabIndex={0}
                aria-label={`编辑路由规则 ${rule.modelPattern}`}
                onClick={() => openDrawer(rule, "edit")}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    openDrawer(rule, "edit");
                  }
                }}
              >
                <div className="list-row-main">
                  <div className="flex items-center gap-2">
                    <span className="term">{rule.modelPattern}</span>
                    <span className={clsx("pill", rule.enabled ? "pill-success" : "pill-neutral")}>{rule.enabled ? "已启用" : "已停用"}</span>
                  </div>
                  <span className="list-row-meta">匹配解析后的模型 · {rule.strategy || "priority"}</span>
                  <span className="list-row-sub">
                    主链路：{formatProviderChain(normalizedRule.providerChain, providerOptions) || "按全局优先级"}
                    {normalizedRule.fallbackChain.length > 0 && ` · 备用：${formatProviderChain(normalizedRule.fallbackChain, providerOptions)}`}
                  </span>
                </div>
                <div className="list-row-actions">
                  <button type="button" className="btn btn-secondary btn-sm" onClick={(e) => { e.stopPropagation(); openDrawer(rule, "test"); }}>
                    <Activity size={14} /> 模拟
                  </button>
                  <button type="button" className="btn btn-secondary btn-sm" onClick={(e) => { e.stopPropagation(); openDrawer(rule, "edit"); }}>
                    编辑
                  </button>
                  <button type="button" className="btn btn-danger btn-sm" onClick={(e) => { e.stopPropagation(); void handleDelete(rule.id); }} aria-label={`删除路由规则 ${rule.modelPattern}`}>
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            );
          })}

          {rules.length === 0 && (
            <div className="empty-state">
              <span className="empty-state-title">暂无路由规则</span>
              <span className="empty-state-desc">创建规则后，匹配到的模型请求会按指定 Provider 链路转发。</span>
              <div className="empty-state-actions">
                <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
                  <Plus size={14} /> 新建规则
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        <div className="flex-col gap-2 list-animate">
          {aliases.map((alias, index) => (
            <div
              key={`${alias.alias}-${index}`}
              className="list-row"
              style={{ "--index": index } as React.CSSProperties}
              role="button"
              tabIndex={0}
              aria-label={`编辑模型别名 ${alias.alias}`}
              onClick={() => openAliasDrawer(alias)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  openAliasDrawer(alias);
                }
              }}
            >
              <div className="list-row-main">
                <div className="flex items-center gap-2">
                  <span className="term">{alias.alias}</span>
                  <span className="pill pill-info">Alias</span>
                </div>
                <span className="list-row-meta">目标模型：{alias.target}</span>
                <span className="list-row-sub">备用链路：{formatProviderChain(normalizeProviderRefs(alias.fallbackChain, providerOptions), providerOptions) || "未配置"}</span>
              </div>
              <div className="list-row-actions">
                <button type="button" className="btn btn-secondary btn-sm" onClick={(e) => { e.stopPropagation(); openAliasDrawer(alias); }}>
                  编辑
                </button>
              </div>
            </div>
          ))}

          {aliases.length === 0 && (
            <div className="empty-state">
              <span className="empty-state-title">暂无模型别名</span>
              <span className="empty-state-desc">别名只负责把业务侧模型名改写成真实模型名，Provider 仍由路由规则决定。</span>
              <div className="empty-state-actions">
                <button type="button" className="btn btn-primary btn-sm" onClick={() => openAliasDrawer()}>
                  <Plus size={14} /> 新建别名
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {drawerOpen && (
        <>
          <div className="drawer-overlay" onClick={closeDrawer} />
          <aside className="drawer drawer-wide" role="dialog" aria-modal="true" aria-label="路由规则配置">
            <div className="drawer-header">
              <div>
                <h3 className="section-title">{form.modelPattern}</h3>
                <p className="section-description">{drawerMode === "edit" ? "规则匹配解析后的模型名；Provider 链路只能从已配置供应商中选择。" : "模拟当前配置的路由决策，不实际请求上游。"}</p>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={closeDrawer} aria-label="关闭路由规则配置">
                <X size={16} />
              </button>
            </div>

            <div style={{ padding: "0 var(--space-5)", paddingBottom: "var(--space-3)" }}>
              <div className="tabs">
                <button type="button" className={clsx("tab", drawerMode === "edit" && "active")} onClick={() => setDrawerMode("edit")}>
                  编辑规则
                </button>
                <button type="button" className={clsx("tab", drawerMode === "test" && "active")} onClick={() => setDrawerMode("test")}>
                  决策模拟
                </button>
              </div>
            </div>

            <div className="drawer-body">
              {drawerMode === "edit" ? (
                <div className="form-grid">
                  <div className="form-field span-2">
                    <label className="form-label">模型匹配规则</label>
                    <input className="form-control" value={form.modelPattern} onChange={(e) => setForm({ ...form, modelPattern: e.target.value })} placeholder="例如：gpt-4o* / claude-*" />
                    <span className="form-hint">匹配的是别名解析后的模型名；支持精确模型名和末尾 * 通配。</span>
                  </div>

                  <div className="form-field">
                    <label className="form-label">策略标识</label>
                    <select className="form-control" value={form.strategy} onChange={(e) => setForm({ ...form, strategy: e.target.value })}>
                      <option value="priority">priority：按链路顺序</option>
                    </select>
                  </div>

                  <div className="form-field">
                    <label className="form-label" style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
                      <input type="checkbox" checked={form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} />
                      <span>启用此规则</span>
                    </label>
                  </div>

                  <ProviderChainEditor
                    className="span-2"
                    label="主链路顺序"
                    hint="第一个可用且支持该模型的 Provider 会被选中。"
                    prefix="主"
                    tone="primary"
                    chain={form.providerChain}
                    providers={providerOptions}
                    onChange={(providerChain) => setForm({ ...form, providerChain })}
                  />

                  <ProviderChainEditor
                    className="span-2"
                    label="备用链路顺序"
                    hint="仅在 429、5xx 或网络错误时继续尝试。"
                    prefix="备"
                    tone="warning"
                    chain={form.fallbackChain}
                    providers={providerOptions}
                    onChange={(fallbackChain) => setForm({ ...form, fallbackChain })}
                  />

                  <div className="detail-card span-2">
                    <strong>链路预览</strong>
                    <RoutePreview title="主链路" prefix="主" tone="primary" chain={form.providerChain} providers={providerOptions} />
                    <RoutePreview title="备用链路" prefix="备" tone="warning" chain={form.fallbackChain} providers={providerOptions} />
                    {formHasUnknownProviders && <span className="form-hint danger-text">存在未匹配到的 Provider，请移除后重新选择。</span>}
                  </div>
                </div>
              ) : (
                <div className="flex-col gap-4">
                  <div className="form-grid">
                    <div className="form-field">
                      <label className="form-label">测试模型</label>
                      <input className="form-control" value={simulation.model} onChange={(e) => setSimulation({ ...simulation, model: e.target.value })} placeholder="例如：gpt-4o" />
                    </div>
                    <div className="form-field">
                      <label className="form-label">请求格式</label>
                      <select className="form-control" value={simulation.format} onChange={(e) => setSimulation({ ...simulation, format: e.target.value })}>
                        <option value="openai">OpenAI Chat Completions</option>
                        <option value="claude">Anthropic Messages</option>
                      </select>
                    </div>
                  </div>

                  <button type="button" className="btn btn-primary" style={{ alignSelf: "flex-start" }} onClick={() => void runSimulation()} disabled={busyAction === "test"}>
                    <RouteIcon size={14} /> {busyAction === "test" ? "模拟中..." : "执行决策模拟"}
                  </button>

                  <div className="detail-grid">
                    <div className="detail-card">
                      <strong>请求模型</strong>
                      <span>{simulation.requestedModel ?? simulation.model}</span>
                    </div>
                    <div className="detail-card">
                      <strong>解析后模型</strong>
                      <span>{simulation.model}</span>
                    </div>
                    <div className="detail-card">
                      <strong>命中 Provider</strong>
                      <span>{simulation.target}</span>
                    </div>
                    <div className="detail-card">
                      <strong>备用链路</strong>
                      <span>{simulation.fallback}</span>
                    </div>
                  </div>

                  <div className={clsx("alert-card", simulation.formatCompatible === false && "warning")}>
                    <strong>
                      {simulation.formatCompatible === false ? "请求格式与 Provider 不兼容" : "模拟范围"}
                    </strong>
                    <p>{simulation.formatCompatible === false ? simulation.formatWarning : simulation.scope ?? SIMULATION_SCOPE}</p>
                  </div>
                </div>
              )}
            </div>

            {drawerMode === "edit" && (
              <div className="drawer-footer">
                <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={busyAction !== null || !form.modelPattern.trim() || formHasUnknownProviders}>
                  <Save size={14} /> 保存规则
                </button>
                <button type="button" className="btn btn-danger" onClick={() => void handleDelete(form.id)} disabled={busyAction !== null}>
                  <Trash2 size={14} /> 删除规则
                </button>
              </div>
            )}
          </aside>
        </>
      )}

      {aliasDrawerOpen && (
        <>
          <div className="drawer-overlay" onClick={() => setAliasDrawerOpen(false)} />
          <aside className="drawer" role="dialog" aria-modal="true" aria-label="模型别名配置">
            <div className="drawer-header">
              <div>
                <h3 className="section-title">{aliasForm.alias}</h3>
                <p className="section-description">别名只改写模型名；改写后的目标模型会继续进入路由规则匹配。</p>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={() => setAliasDrawerOpen(false)} aria-label="关闭模型别名配置">
                <X size={16} />
              </button>
            </div>
            <div className="drawer-body">
              <div className="form-grid">
                <div className="form-field span-2">
                  <label className="form-label">别名</label>
                  <input className="form-control" value={aliasForm.alias} onChange={(e) => setAliasForm({ ...aliasForm, alias: e.target.value })} placeholder="例如：gpt-fast" />
                </div>
                <div className="form-field span-2">
                  <label className="form-label">目标模型</label>
                  <input className="form-control" value={aliasForm.target} onChange={(e) => setAliasForm({ ...aliasForm, target: e.target.value })} placeholder="例如：gpt-4o-mini" />
                  <span className="form-hint">这里填写真实模型名，不是 Provider。Provider 由路由规则或默认优先级决定。</span>
                </div>
                <ProviderChainEditor
                  className="span-2"
                  label="别名备用链路"
                  hint="当目标模型没有命中路由规则的备用链路时使用。"
                  prefix="备"
                  tone="warning"
                  chain={aliasForm.fallbackChain}
                  providers={providerOptions}
                  onChange={(fallbackChain) => setAliasForm({ ...aliasForm, fallbackChain })}
                />
                {aliasHasUnknownProviders && <span className="form-hint danger-text span-2">存在未匹配到的 Provider，请移除后重新选择。</span>}
              </div>
            </div>
            <div className="drawer-footer">
              <button type="button" className="btn btn-primary" onClick={() => void handleAliasSave()} disabled={busyAction !== null || !aliasForm.alias.trim() || !aliasForm.target.trim() || aliasHasUnknownProviders}>
                <Save size={14} /> 保存别名
              </button>
            </div>
          </aside>
        </>
      )}
    </div>
  );
}

function FlowStep({ index, title, detail }: { index: string; title: string; detail: string }) {
  return (
    <div className="routing-flow-step">
      <span className="routing-flow-index">{index}</span>
      <strong>{title}</strong>
      <span>{detail}</span>
    </div>
  );
}

function ProviderChainEditor({
  className,
  label,
  hint,
  prefix,
  tone,
  chain,
  providers,
  onChange
}: {
  className?: string;
  label: string;
  hint: string;
  prefix: string;
  tone: "primary" | "warning";
  chain: string[];
  providers: ProviderRecord[];
  onChange: (next: string[]) => void;
}) {
  const [selectedProvider, setSelectedProvider] = useState("");
  const usableProviders = providers.filter(isUsableProvider);

  const addSelectedProvider = () => {
    onChange(addProviderToChain(chain, selectedProvider));
    setSelectedProvider("");
  };

  return (
    <div className={clsx("form-field provider-chain-field", className)}>
      <label className="form-label">{label}</label>
      <span className="form-hint">{hint}</span>
      <div className="provider-chain-picker">
        <select className="form-control" value={selectedProvider} onChange={(event) => setSelectedProvider(event.target.value)} disabled={usableProviders.length === 0}>
          <option value="">{usableProviders.length === 0 ? "暂无可用 Provider" : "选择 Provider"}</option>
          {usableProviders.map((provider) => (
            <option key={provider.id} value={provider.id}>
              {provider.name}
            </option>
          ))}
        </select>
        <button type="button" className="btn btn-secondary btn-sm" onClick={addSelectedProvider} disabled={!selectedProvider || chain.includes(selectedProvider)}>
          <Plus size={14} /> 添加
        </button>
      </div>

      <div className="provider-chain-list">
        {chain.length === 0 && <span className="chain-empty">未指定，按默认 Provider 优先级解析</span>}
        {chain.map((providerRef, index) => {
          const provider = findProvider(providerRef, providers);
          return (
            <div key={`${providerRef}-${index}`} className={clsx("provider-chain-row", !provider && "invalid")}>
              <span className={clsx("chain-order", tone)}>{prefix}{index + 1}</span>
              <div className="provider-chain-row-main">
                <strong>{provider ? provider.name : providerRef}</strong>
                <span>{provider ? providerMeta(provider) : "未找到匹配 Provider"}</span>
              </div>
              <div className="provider-chain-actions">
                <button type="button" className="btn btn-ghost btn-icon" onClick={() => onChange(moveProviderInChain(chain, index, -1))} disabled={index === 0} aria-label={`上移 ${providerLabel(providerRef, providers)}`}>
                  <ArrowUp size={14} />
                </button>
                <button type="button" className="btn btn-ghost btn-icon" onClick={() => onChange(moveProviderInChain(chain, index, 1))} disabled={index === chain.length - 1} aria-label={`下移 ${providerLabel(providerRef, providers)}`}>
                  <ArrowDown size={14} />
                </button>
                <button type="button" className="btn btn-ghost btn-icon" onClick={() => onChange(removeProviderFromChain(chain, index))} aria-label={`移除 ${providerLabel(providerRef, providers)}`}>
                  <X size={14} />
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function RoutePreview({ title, prefix, tone, chain, providers }: { title: string; prefix: string; tone: "primary" | "warning"; chain: string[]; providers: ProviderRecord[] }) {
  return (
    <div className="route-preview-lane">
      <span className="route-preview-title">{title}</span>
      <div className="route-chain">
        {chain.length === 0 ? (
          <span className="chain-step">未指定</span>
        ) : (
          chain.map((item, index) => (
            <span key={`${item}-${index}`} className={`chain-step ${tone}`}>
              {prefix}{index + 1} · {providerLabel(item, providers)}
            </span>
          ))
        )}
      </div>
    </div>
  );
}

function normalizeRuleForm(rule: RoutingRuleRecord, providers: ProviderRecord[]): RoutingRuleForm {
  return {
    ...rule,
    modelPattern: rule.modelPattern || rule.model_pattern || "*",
    strategy: rule.strategy || "priority",
    providerChain: normalizeProviderRefs(rule.providerChain, providers).filter(Boolean),
    fallbackChain: normalizeProviderRefs(rule.fallbackChain, providers).filter(Boolean),
    enabled: rule.enabled ?? true
  };
}

function formatProviderChain(chain: string[], providers: ProviderRecord[]): string {
  return chain.map((provider) => providerLabel(provider, providers)).join(" -> ");
}

function hasUnknownProviderRefs(refs: string[], providers: ProviderRecord[]): boolean {
  return refs.some((ref) => !findProvider(ref, providers));
}

function findProvider(ref: string, providers: ProviderRecord[]): ProviderRecord | undefined {
  const value = providerValue(ref, providers);
  return providers.find((provider) => provider.id === value || provider.name === value);
}

function isUsableProvider(provider: ProviderRecord): boolean {
  return provider.enabled !== false && provider.status !== "disabled" && provider.status !== "deleted";
}

function firstUsableProvider(providers: ProviderRecord[]): ProviderRecord | undefined {
  return providers.find(isUsableProvider) ?? providers[0];
}

function firstModel(providers: ProviderRecord[]): string | undefined {
  return providers.flatMap((provider) => provider.models).find(Boolean);
}

function firstModelPattern(providers: ProviderRecord[]): string {
  const model = firstModel(providers);
  if (!model) return "new-model-*";
  const family = model.split("-").slice(0, 2).join("-");
  return family ? `${family}*` : model;
}

function sampleModelFromPattern(pattern: string): string {
  const trimmed = pattern.trim();
  if (!trimmed || trimmed === "*") return "gpt-4o";
  return trimmed.endsWith("*") ? `${trimmed.slice(0, -1)}mini` : trimmed;
}

function providerMeta(provider: ProviderRecord): string {
  const models = provider.models.length > 0 ? `${provider.models.length} 个模型` : "未限定模型";
  return `${provider.type} · 优先级 ${provider.priority || "-"} · ${models}`;
}
