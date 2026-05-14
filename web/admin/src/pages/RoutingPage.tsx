import { useMemo, useState } from "react";
import { Activity, Plus, Route, Save, Trash2, X } from "lucide-react";
import clsx from "clsx";
import { useAdminStore } from "../store/admin-store";
import type { ModelAliasRecord, RoutingSimulation } from "../store/entities";

const createEmptyRule = () => ({
  id: `route-${Date.now()}`,
  modelPattern: "new-model-*",
  strategy: "优先转发 + 备用切换",
  providerChain: ["OpenAI 主线路"],
  fallbackChain: ["Azure 备用线路"],
  enabled: true
});

const createEmptyAlias = (): ModelAliasRecord => ({
  id: `alias-${Date.now()}`,
  alias: "gpt-fast",
  target: "OpenAI 主线路",
  fallbackChain: ["DeepSeek 节省线路"]
});

export function RoutingPage() {
  const { rules, aliases, saveRule, deleteRule, saveAlias, testRouting } = useAdminStore();

  const [viewMode, setViewMode] = useState<"rules" | "aliases">("rules");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [aliasDrawerOpen, setAliasDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<"edit" | "test">("edit");
  const [form, setForm] = useState<ReturnType<typeof createEmptyRule>>(createEmptyRule());
  const [aliasForm, setAliasForm] = useState<ModelAliasRecord>(createEmptyAlias());
  const [busyAction, setBusyAction] = useState<"save" | "delete" | "test" | "alias" | null>(null);
  const [simulation, setSimulation] = useState<RoutingSimulation>({
    model: "gpt-4o",
    key: "默认本地密钥",
    format: "openai",
    target: "等待模拟",
    fallback: "等待模拟",
    cost: "-",
    ttft: "-"
  });

  const enabledRules = useMemo(() => rules.filter((rule) => rule.enabled).length, [rules]);
  const fallbackRules = useMemo(() => rules.filter((rule) => rule.fallbackChain.length > 0).length, [rules]);

  const openDrawer = (rule?: typeof form, mode: "edit" | "test" = "edit") => {
    setForm(rule ?? createEmptyRule());
    setDrawerMode(mode);
    setDrawerOpen(true);
  };

  const openAliasDrawer = (alias?: ModelAliasRecord) => {
    setAliasForm(alias ?? createEmptyAlias());
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
          <h2 className="section-title">路由规则</h2>
          <p className="section-description">路由决定一个模型请求先走哪条 Provider Chain，失败时按哪个 Fallback Chain 继续尝试。默认只展示关键路径，复杂配置进入抽屉处理。</p>
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

      <div className="flex items-center gap-2">
        <span className="pill pill-neutral">规则 {rules.length}</span>
        <span className="pill pill-success">已启用 {enabledRules}</span>
        <span className="pill pill-warning">Fallback {fallbackRules}</span>
        <span className="pill pill-info">模型别名 {aliases.length}</span>
      </div>

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
          {rules.map((rule, index) => (
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
                <span className="list-row-meta">{rule.strategy}</span>
                <span className="list-row-sub">
                  主链路：{rule.providerChain.join(" → ") || "未配置"}
                  {rule.fallbackChain.length > 0 && ` · 备用：${rule.fallbackChain.join(" → ")}`}
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
          ))}

          {rules.length === 0 && (
            <div className="empty-state">
              <span className="empty-state-title">暂无路由规则</span>
              <span className="empty-state-desc">先创建一个模型匹配规则，把请求明确分发到主 Provider 和备用 Provider。</span>
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
                <span className="list-row-sub">备用：{alias.fallbackChain.join(" → ") || "未配置"}</span>
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
              <span className="empty-state-desc">用别名把业务侧模型名映射到真实上游模型，减少客户端改造成本。</span>
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
                <p className="section-description">{drawerMode === "edit" ? "配置模型匹配、主链路和备用链路。" : "用真实输入模拟一次路由决策。"}</p>
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
                  路由模拟
                </button>
              </div>
            </div>

            <div className="drawer-body">
              {drawerMode === "edit" ? (
                <div className="form-grid">
                  <div className="form-field span-2">
                    <label className="form-label">模型匹配规则</label>
                    <input className="form-control" value={form.modelPattern} onChange={(e) => setForm({ ...form, modelPattern: e.target.value })} placeholder="例如：gpt-4o* / claude-*" />
                    <span className="form-hint">支持精确模型名和通配模式。更具体的规则应放在更高优先级。</span>
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">分发策略</label>
                    <input className="form-control" value={form.strategy} onChange={(e) => setForm({ ...form, strategy: e.target.value })} placeholder="例如：优先转发 + 备用切换" />
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">主链路顺序</label>
                    <input
                      className="form-control"
                      value={form.providerChain.join(" → ")}
                      onChange={(e) => setForm({ ...form, providerChain: e.target.value.split(/[→,]/).map((s) => s.trim()).filter(Boolean) })}
                      placeholder="例如：OpenAI 主线路 → Azure 备用线路"
                    />
                    <span className="form-hint">主链路从左到右尝试，建议把稳定且成本可控的 Provider 放在前面。</span>
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">备用链路顺序</label>
                    <input
                      className="form-control"
                      value={form.fallbackChain.join(" → ")}
                      onChange={(e) => setForm({ ...form, fallbackChain: e.target.value.split(/[→,]/).map((s) => s.trim()).filter(Boolean) })}
                      placeholder="例如：Azure 备用线路 → OpenRouter 备用出口"
                    />
                    <span className="form-hint">当上游返回 429、5xx 或网络错误时，系统会按备用链路继续尝试。</span>
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label" style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
                      <input type="checkbox" checked={form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} />
                      <span>启用此规则</span>
                    </label>
                  </div>

                  <div className="detail-card span-2">
                    <strong>链路预览</strong>
                    <div className="route-chain mt-4">{renderChain(form.providerChain, "primary")}</div>
                    <div className="route-chain mt-4">{renderChain(form.fallbackChain, "warning")}</div>
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
                      <label className="form-label">本地密钥</label>
                      <input className="form-control" value={simulation.key} onChange={(e) => setSimulation({ ...simulation, key: e.target.value })} placeholder="例如：tb-..." />
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
                    <Route size={14} /> {busyAction === "test" ? "模拟中..." : "执行模拟"}
                  </button>

                  <div className="detail-grid">
                    <div className="detail-card">
                      <strong>目标线路</strong>
                      <span>{simulation.target}</span>
                    </div>
                    <div className="detail-card">
                      <strong>备用线路</strong>
                      <span>{simulation.fallback}</span>
                    </div>
                    <div className="detail-card">
                      <strong>预计费用</strong>
                      <span>{simulation.cost}</span>
                    </div>
                    <div className="detail-card">
                      <strong>首字返回时间</strong>
                      <span>{simulation.ttft}</span>
                    </div>
                  </div>

                  <div className="alert-card">
                    <strong>模拟结果怎么读</strong>
                    <p>目标线路是本次请求会先命中的 Provider；备用线路表示发生可重试错误时的后续尝试顺序。</p>
                  </div>
                </div>
              )}
            </div>

            {drawerMode === "edit" && (
              <div className="drawer-footer">
                <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={busyAction !== null}>
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
                <p className="section-description">把业务侧模型名映射到真实上游模型名。</p>
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
                  <label className="form-label">目标模型或 Provider</label>
                  <input className="form-control" value={aliasForm.target} onChange={(e) => setAliasForm({ ...aliasForm, target: e.target.value })} placeholder="例如：gpt-4o-mini" />
                </div>
                <div className="form-field span-2">
                  <label className="form-label">备用链路</label>
                  <input
                    className="form-control"
                    value={aliasForm.fallbackChain.join(" → ")}
                    onChange={(e) => setAliasForm({ ...aliasForm, fallbackChain: e.target.value.split(/[→,]/).map((s) => s.trim()).filter(Boolean) })}
                    placeholder="例如：DeepSeek 节省线路 → OpenRouter"
                  />
                </div>
              </div>
            </div>
            <div className="drawer-footer">
              <button type="button" className="btn btn-primary" onClick={() => void handleAliasSave()} disabled={busyAction !== null}>
                <Save size={14} /> 保存别名
              </button>
            </div>
          </aside>
        </>
      )}
    </div>
  );
}

function renderChain(items: string[], tone: "primary" | "warning") {
  if (!items.length) {
    return <span className="chain-step">未配置</span>;
  }
  return items.map((item, index) => (
    <span key={`${item}-${index}`} className={`chain-step ${tone}`}>
      {index + 1}. {item}
    </span>
  ));
}
