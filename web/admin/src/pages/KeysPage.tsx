import { useEffect, useMemo, useState } from "react";
import { CalendarClock, Copy, RotateCcw, ShieldAlert, X } from "lucide-react";
import { useAdminStore } from "../store/admin-store";
import { keyStatusMap } from "../store/labels";

const createEmptyKey = () => ({
  id: `key-${Date.now()}`,
  name: "新本地密钥",
  displayKey: `tb-${Math.random().toString(36).slice(2, 6)}****${Math.random().toString(36).slice(2, 6)}`,
  allowedModels: [],
  allowedProviders: [],
  monthlyBudget: 100,
  currentSpend: 0,
  tokenBudget: 5000000,
  currentTokens: 0,
  enabled: true,
  status: "active" as const,
  expires_at: null as string | null
});

const unique = (items: string[]) => Array.from(new Set(items.filter(Boolean)));

export function KeysPage() {
  const { keys, providers, selectedKeyId, setSelectedKey, saveKey, rotateKey, revokeKey, extendKey, pushNotice } = useAdminStore();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [form, setForm] = useState<ReturnType<typeof createEmptyKey>>(createEmptyKey());
  const [expiresAt, setExpiresAt] = useState("");

  const active = useMemo(() => keys.find((item) => item.id === selectedKeyId) ?? keys[0], [keys, selectedKeyId]);

  const providerOptions = useMemo(
    () => providers.filter((p) => p.enabled !== false && p.status !== "disabled").map((p) => p.name),
    [providers]
  );
  const selectedProviders = form.allowedProviders.filter((p) => providerOptions.includes(p));
  const modelOptions = useMemo(
    () => unique(providers.filter((p) => selectedProviders.includes(p.name)).flatMap((p) => p.models)),
    [providers, selectedProviders]
  );
  const selectedModels = form.allowedModels.filter((m) => modelOptions.includes(m));

  useEffect(() => {
    if (active) {
      setForm(active);
      setExpiresAt(active.expires_at ? active.expires_at.slice(0, 10) : "");
    }
  }, [active]);

  const openDrawer = (key?: typeof form) => {
    if (key) {
      setForm(key);
      setSelectedKey(key.id);
    } else {
      const next = createEmptyKey();
      setForm(next);
      setSelectedKey(next.id);
    }
    setDrawerOpen(true);
  };

  const closeDrawer = () => setDrawerOpen(false);

  const spendUsage = Math.min(100, Math.round((form.currentSpend / Math.max(form.monthlyBudget, 1)) * 100));
  const tokenUsage = Math.min(100, Math.round((form.currentTokens / Math.max(form.tokenBudget, 1)) * 100));

  const handleSave = () => {
    if (selectedProviders.length === 0) {
      pushNotice({ tone: "warning", title: "请选择厂商", message: "本地 API 需要绑定至少一个允许厂商。" });
      return;
    }
    if (selectedModels.length === 0) {
      pushNotice({ tone: "warning", title: "请选择模型", message: "本地 API 需要绑定至少一个所选厂商支持的模型。" });
      return;
    }
    void saveKey({
      ...form,
      allowedProviders: selectedProviders,
      allowedModels: selectedModels,
      expires_at: expiresAt ? `${expiresAt}T23:59:59Z` : null
    });
    closeDrawer();
  };

  const handleRevoke = (id: string) => {
    if (!confirm("确定吊销此密钥？此操作不可恢复。")) return;
    void revokeKey(id);
    if (form.id === id) closeDrawer();
  };

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">访问控制</span>
          <h2 className="section-title">密钥管理</h2>
        </div>
        <div className="section-actions">
          <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
            新建密钥
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="flex items-center gap-2">
        <span className="pill pill-neutral">总数 {keys.length}</span>
        <span className="pill pill-success">启用中 {keys.filter((k) => k.status === "active").length}</span>
        <span className="pill pill-warning">预算告警 {keys.filter((k) => k.status === "warning").length}</span>
      </div>

      {/* List */}
      <div className="flex-col gap-2 list-animate">
        {keys.map((key, index) => {
          const spendPct = Math.min(100, Math.round((key.currentSpend / Math.max(key.monthlyBudget, 1)) * 100));
          return (
            <div
              key={key.id}
              className="list-row"
              style={{ "--index": index } as React.CSSProperties}
            >
              <div className="list-row-main">
                <div className="flex items-center gap-2">
                  <span className="list-row-title">{key.name}</span>
                  <span
                    className="pill"
                    style={{
                      background:
                        key.status === "active"
                          ? "var(--accent-dim)"
                          : key.status === "warning"
                          ? "var(--warning-dim)"
                          : "var(--danger-dim)",
                      color:
                        key.status === "active"
                          ? "var(--accent)"
                          : key.status === "warning"
                          ? "var(--warning)"
                          : "var(--danger)"
                    }}
                  >
                    {keyStatusMap[key.status] ?? key.status}
                  </span>
                </div>
                <span className="list-row-meta" style={{ fontFamily: "var(--font-mono)" }}>
                  {key.displayKey}
                </span>
                <span className="list-row-sub">
                  {key.allowedModels.length} 个模型 · 已用 ${key.currentSpend.toFixed(1)} / ${key.monthlyBudget}
                  {key.expires_at ? ` · 到期 ${new Date(key.expires_at).toLocaleDateString()}` : " · 长期有效"}
                </span>
                {/* Inline budget bar */}
                <div className="progress-bar" style={{ maxWidth: 240, marginTop: 4 }}>
                  <div
                    className="progress-bar-fill"
                    style={{
                      width: `${spendPct}%`,
                      background: spendPct >= 90 ? "var(--danger)" : spendPct >= 70 ? "var(--warning)" : "var(--accent)"
                    }}
                  />
                </div>
              </div>
              <div className="list-row-actions">
                <button
                  type="button"
                  className="btn btn-secondary btn-sm"
                  onClick={(e) => { e.stopPropagation(); openDrawer(key); }}
                >
                  编辑
                </button>
                <button
                  type="button"
                  className="btn btn-danger btn-sm"
                  onClick={(e) => { e.stopPropagation(); void handleRevoke(key.id); }}
                >
                  <ShieldAlert size={14} />
                </button>
              </div>
            </div>
          );
        })}

        {keys.length === 0 && (
          <div className="empty-state">
            <span style={{ fontSize: "1.25rem", color: "var(--text-tertiary)" }}>○</span>
            <span className="empty-state-title">暂无密钥</span>
            <span className="empty-state-desc">点击右上角"新建密钥"创建第一个本地 API Key</span>
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
                <h3 className="section-title">{form.name}</h3>
                <p style={{ fontSize: "0.82rem", color: "var(--text-secondary)", marginTop: 2, fontFamily: "var(--font-mono)" }}>
                  {form.displayKey}
                </p>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={closeDrawer}>
                <X size={16} />
              </button>
            </div>

            <div className="drawer-body">
              <div className="form-grid">
                <div className="form-field span-2">
                  <label className="form-label">密钥名称</label>
                  <input
                    className="form-control"
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    placeholder="例如：Codex 专用密钥"
                  />
                </div>

                <div className="form-field span-2">
                  <label className="form-label">允许厂商</label>
                  <select
                    className="form-control"
                    multiple
                    value={selectedProviders}
                    disabled={providerOptions.length === 0}
                    onChange={(e) => {
                      const nextProviders = Array.from(e.currentTarget.selectedOptions, (o) => o.value);
                      const nextModelOptions = unique(providers.filter((p) => nextProviders.includes(p.name)).flatMap((p) => p.models));
                      setForm({
                        ...form,
                        allowedProviders: nextProviders,
                        allowedModels: form.allowedModels.filter((m) => nextModelOptions.includes(m))
                      });
                    }}
                    size={4}
                  >
                    {providerOptions.map((p) => (
                      <option key={p} value={p}>{p}</option>
                    ))}
                  </select>
                  <span className="form-hint">{providerOptions.length ? "按住 Ctrl 可选择多个厂商" : "请先到厂商接入页面新增厂商"}</span>
                </div>

                <div className="form-field span-2">
                  <label className="form-label">允许模型</label>
                  <select
                    className="form-control"
                    multiple
                    value={selectedModels}
                    disabled={modelOptions.length === 0}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        allowedModels: Array.from(e.currentTarget.selectedOptions, (o) => o.value)
                      })
                    }
                    size={4}
                  >
                    {modelOptions.map((m) => (
                      <option key={m} value={m}>{m}</option>
                    ))}
                  </select>
                  <span className="form-hint">{modelOptions.length ? "模型来自已选厂商的模型配置" : "请先选择带有模型配置的厂商"}</span>
                </div>

                <div className="form-field">
                  <label className="form-label">月预算（美元）</label>
                  <input
                    className="form-control"
                    type="number"
                    value={form.monthlyBudget}
                    onChange={(e) => setForm({ ...form, monthlyBudget: Number(e.target.value) })}
                  />
                </div>

                <div className="form-field">
                  <label className="form-label">令牌预算</label>
                  <input
                    className="form-control"
                    type="number"
                    value={form.tokenBudget}
                    onChange={(e) => setForm({ ...form, tokenBudget: Number(e.target.value) })}
                  />
                </div>

                <div className="form-field">
                  <label className="form-label">到期日期</label>
                  <input
                    className="form-control"
                    type="date"
                    value={expiresAt}
                    onChange={(e) => setExpiresAt(e.target.value)}
                  />
                </div>

                <div className="form-field">
                  <label className="form-label">当前有效期</label>
                  <input
                    className="form-control"
                    value={form.expires_at ? new Date(form.expires_at).toLocaleString() : "长期有效"}
                    readOnly
                  />
                </div>
              </div>

              {/* Usage bars */}
              <div className="flex-col gap-3">
                <div className="usage-row">
                  <div className="usage-header">
                    <span className="usage-label">本月预算使用率</span>
                    <span className="usage-value">{spendUsage}%</span>
                  </div>
                  <div className="progress-bar">
                    <div
                      className="progress-bar-fill"
                      style={{
                        width: `${spendUsage}%`,
                        background: spendUsage >= 90 ? "var(--danger)" : spendUsage >= 70 ? "var(--warning)" : "var(--accent)"
                      }}
                    />
                  </div>
                </div>
                <div className="usage-row">
                  <div className="usage-header">
                    <span className="usage-label">令牌配额使用率</span>
                    <span className="usage-value">{tokenUsage}%</span>
                  </div>
                  <div className="progress-bar">
                    <div
                      className="progress-bar-fill"
                      style={{
                        width: `${tokenUsage}%`,
                        background: tokenUsage >= 90 ? "var(--danger)" : tokenUsage >= 70 ? "var(--warning)" : "var(--info)"
                      }}
                    />
                  </div>
                </div>
              </div>
            </div>

            <div className="drawer-footer">
              <button type="button" className="btn btn-primary" onClick={handleSave}>
                保存变更
              </button>
              <button
                type="button"
                className="btn btn-secondary"
                onClick={() => {
                  void navigator.clipboard?.writeText(form.displayKey);
                  pushNotice({ tone: "success", title: "密钥已复制", message: `${form.name} 的展示密钥已复制到剪贴板。` });
                }}
              >
                <Copy size={14} /> 复制密钥
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => void rotateKey(form.id)}>
                <RotateCcw size={14} /> 轮换密钥
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => void extendKey(form.id, expiresAt ? `${expiresAt}T23:59:59Z` : null)}>
                <CalendarClock size={14} /> 保存有效期
              </button>
              <button type="button" className="btn btn-danger" onClick={() => void handleRevoke(form.id)}>
                <ShieldAlert size={14} /> 吊销
              </button>
            </div>
          </aside>
        </>
      )}
    </div>
  );
}
