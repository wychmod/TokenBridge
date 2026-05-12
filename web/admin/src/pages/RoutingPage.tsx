import { useEffect, useMemo, useState } from "react";
import { Activity, Route, Trash2, X } from "lucide-react";
import { useAdminStore } from "../store/admin-store";
import type { RoutingSimulation } from "../store/entities";

const createEmptyRule = () => ({
  id: `route-${Date.now()}`,
  modelPattern: "new-model-*",
  strategy: "优先转发 + 备用切换",
  providerChain: ["OpenAI 主线路"],
  fallbackChain: ["Azure 备用线路"],
  enabled: true
});

export function RoutingPage() {
  const { rules, saveRule, deleteRule, testRouting, pushNotice } = useAdminStore();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<"edit" | "test">("edit");
  const [form, setForm] = useState<ReturnType<typeof createEmptyRule>>(createEmptyRule());
  const [simulation, setSimulation] = useState<RoutingSimulation>({
    model: "gpt-4o",
    key: "默认本地密钥",
    format: "openai",
    target: "等待模拟",
    fallback: "等待模拟",
    cost: "-",
    ttft: "-"
  });

  const openDrawer = (rule?: typeof form, mode: "edit" | "test" = "edit") => {
    if (rule) {
      setForm(rule);
    } else {
      setForm(createEmptyRule());
    }
    setDrawerMode(mode);
    setDrawerOpen(true);
  };

  const closeDrawer = () => setDrawerOpen(false);

  const handleSave = () => {
    void saveRule(form);
    closeDrawer();
  };

  const handleDelete = (id: string) => {
    if (!confirm("确定删除此路由规则？")) return;
    void deleteRule(id);
    if (form.id === id) closeDrawer();
  };

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">调度策略</span>
          <h2 className="section-title">路由规则</h2>
        </div>
        <div className="section-actions">
          <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
            新建规则
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="flex items-center gap-2">
        <span className="pill pill-neutral">总数 {rules.length}</span>
        <span className="pill pill-success">已启用 {rules.filter((r) => r.enabled).length}</span>
        <span className="pill pill-neutral">已配置备用 {rules.filter((r) => r.fallbackChain.length > 0).length}</span>
      </div>

      {/* List */}
      <div className="flex-col gap-2 list-animate">
        {rules.map((rule, index) => (
          <div
            key={rule.id}
            className="list-row"
            style={{ "--index": index } as React.CSSProperties}
          >
            <div className="list-row-main">
              <div className="flex items-center gap-2">
                <span className="list-row-title">{rule.modelPattern}</span>
                <span
                  className="pill"
                  style={{
                    background: rule.enabled ? "var(--accent-dim)" : "var(--bg-elevated)",
                    color: rule.enabled ? "var(--accent)" : "var(--text-tertiary)"
                  }}
                >
                  {rule.enabled ? "已启用" : "已停用"}
                </span>
              </div>
              <span className="list-row-meta">{rule.strategy}</span>
              <span className="list-row-sub">
                主链路：{rule.providerChain.join(" → ") || "未配置"}
                {rule.fallbackChain.length > 0 && ` · 备用：${rule.fallbackChain.join(" → ")}`}
              </span>
            </div>
            <div className="list-row-actions">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={(e) => { e.stopPropagation(); openDrawer(rule, "test"); }}
              >
                <Activity size={14} /> 测试
              </button>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={(e) => { e.stopPropagation(); openDrawer(rule, "edit"); }}
              >
                编辑
              </button>
              <button
                type="button"
                className="btn btn-danger btn-sm"
                onClick={(e) => { e.stopPropagation(); handleDelete(rule.id); }}
              >
                <Trash2 size={14} />
              </button>
            </div>
          </div>
        ))}

        {rules.length === 0 && (
          <div className="empty-state">
            <span style={{ fontSize: "1.25rem", color: "var(--text-tertiary)" }}>○</span>
            <span className="empty-state-title">暂无路由规则</span>
            <span className="empty-state-desc">点击右上角"新建规则"创建第一个调度策略</span>
          </div>
        )}
      </div>

      {/* Drawer */}
      {drawerOpen && (
        <>
          <div className="drawer-overlay" onClick={closeDrawer} />
          <aside className="drawer">
            <div className="drawer-header">
              <div>
                <h3 className="section-title">{form.modelPattern}</h3>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={closeDrawer}>
                <X size={16} />
              </button>
            </div>

            {/* Tabs */}
            <div style={{ padding: "0 var(--space-5)", paddingBottom: "var(--space-3)" }}>
              <div className="tabs">
                <button
                  type="button"
                  className={clsx("tab", drawerMode === "edit" && "active")}
                  onClick={() => setDrawerMode("edit")}
                >
                  编辑规则
                </button>
                <button
                  type="button"
                  className={clsx("tab", drawerMode === "test" && "active")}
                  onClick={() => setDrawerMode("test")}
                >
                  路由测试
                </button>
              </div>
            </div>

            <div className="drawer-body">
              {drawerMode === "edit" ? (
                <div className="form-grid">
                  <div className="form-field span-2">
                    <label className="form-label">模型匹配规则</label>
                    <input
                      className="form-control"
                      value={form.modelPattern}
                      onChange={(e) => setForm({ ...form, modelPattern: e.target.value })}
                      placeholder="例如：gpt-4o* / claude-*"
                    />
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">分发策略</label>
                    <input
                      className="form-control"
                      value={form.strategy}
                      onChange={(e) => setForm({ ...form, strategy: e.target.value })}
                      placeholder="例如：优先转发 + 备用切换"
                    />
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">主链路顺序</label>
                    <input
                      className="form-control"
                      value={form.providerChain.join(" → ")}
                      onChange={(e) =>
                        setForm({
                          ...form,
                          providerChain: e.target.value.split(/[→,]/).map((s) => s.trim()).filter(Boolean)
                        })
                      }
                      placeholder="例如：OpenAI 主线路 → Azure 备用线路"
                    />
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label">备用链路顺序</label>
                    <input
                      className="form-control"
                      value={form.fallbackChain.join(" → ")}
                      onChange={(e) =>
                        setForm({
                          ...form,
                          fallbackChain: e.target.value.split(/[→,]/).map((s) => s.trim()).filter(Boolean)
                        })
                      }
                      placeholder="例如：Azure 备用线路 → OpenRouter 备用出口"
                    />
                  </div>

                  <div className="form-field span-2">
                    <label className="form-label" style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
                      <input
                        type="checkbox"
                        checked={form.enabled}
                        onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                      />
                      <span>启用此规则</span>
                    </label>
                  </div>

                  <div className="flex-col gap-1 span-2" style={{ padding: "var(--space-3)", background: "var(--bg-base)", borderRadius: "var(--radius-sm)" }}>
                    <strong style={{ fontSize: "0.85rem", color: "var(--text-primary)" }}>链路预览</strong>
                    <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 4 }}>
                      主链路：{form.providerChain.join(" → ") || "未配置"}
                    </p>
                    <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>
                      备用链路：{form.fallbackChain.join(" → ") || "未配置"}
                    </p>
                  </div>
                </div>
              ) : (
                <div className="flex-col gap-4">
                  <div className="form-grid">
                    <div className="form-field">
                      <label className="form-label">测试模型</label>
                      <input
                        className="form-control"
                        value={simulation.model}
                        onChange={(e) => setSimulation({ ...simulation, model: e.target.value })}
                        placeholder="例如：gpt-4o"
                      />
                    </div>
                    <div className="form-field">
                      <label className="form-label">本地密钥</label>
                      <input
                        className="form-control"
                        value={simulation.key}
                        onChange={(e) => setSimulation({ ...simulation, key: e.target.value })}
                        placeholder="例如：tb-..."
                      />
                    </div>
                    <div className="form-field">
                      <label className="form-label">请求格式</label>
                      <input
                        className="form-control"
                        value={simulation.format}
                        onChange={(e) => setSimulation({ ...simulation, format: e.target.value })}
                        placeholder="openai / claude"
                      />
                    </div>
                  </div>

                  <button
                    type="button"
                    className="btn btn-primary"
                    style={{ alignSelf: "flex-start" }}
                    onClick={async () => {
                      const result = await testRouting({
                        model: simulation.model,
                        localKey: simulation.key,
                        format: simulation.format
                      });
                      if (result) setSimulation(result);
                    }}
                  >
                    <Route size={14} /> 执行模拟
                  </button>

                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-3)", padding: "var(--space-4)", background: "var(--bg-base)", borderRadius: "var(--radius-sm)" }}>
                    <div>
                      <span className="eyebrow">目标线路</span>
                      <p style={{ fontSize: "0.9rem", fontWeight: 600, marginTop: 4 }}>{simulation.target}</p>
                    </div>
                    <div>
                      <span className="eyebrow">备用线路</span>
                      <p style={{ fontSize: "0.9rem", fontWeight: 600, marginTop: 4 }}>{simulation.fallback}</p>
                    </div>
                    <div>
                      <span className="eyebrow">预计费用</span>
                      <p style={{ fontSize: "0.9rem", fontWeight: 600, marginTop: 4 }}>{simulation.cost}</p>
                    </div>
                    <div>
                      <span className="eyebrow">首字返回时间</span>
                      <p style={{ fontSize: "0.9rem", fontWeight: 600, marginTop: 4 }}>{simulation.ttft}</p>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {drawerMode === "edit" && (
              <div className="drawer-footer">
                <button type="button" className="btn btn-primary" onClick={handleSave}>
                  保存规则
                </button>
                <button type="button" className="btn btn-danger" onClick={() => void handleDelete(form.id)}>
                  <Trash2 size={14} /> 删除规则
                </button>
              </div>
            )}
          </aside>
        </>
      )}
    </div>
  );
}
