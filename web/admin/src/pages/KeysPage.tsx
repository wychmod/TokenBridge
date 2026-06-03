import { useEffect, useMemo, useState } from "react";
import { CalendarClock, Copy, Loader2, Plus, RotateCcw, ShieldAlert, X } from "lucide-react";
import clsx from "clsx";
import { useAdminStore } from "../store/admin-store";
import { keyStatusMap } from "../store/labels";
import type { LocalKeyRecord } from "../store/entities";
import { dateInputToExpiryISOString, expiryISOStringToDateInput, formatLocalKeyExpiryLabel } from "../utils/local-key-expiry";

const createEmptyKey = (): LocalKeyRecord => ({
  id: `key-${Date.now()}`,
  name: "新本地密钥",
  displayKey: `tb-${Math.random().toString(36).slice(2, 6)}****${Math.random().toString(36).slice(2, 6)}`,
  allowedModels: [],
  allowedProviders: [],
  monthlyBudget: 0,
  currentSpend: 0,
  tokenBudget: 0,
  currentTokens: 0,
  enabled: true,
  status: "active" as const,
  expires_at: null as string | null
});

const unique = (items: string[]) => Array.from(new Set(items.filter(Boolean)));
const budgetUsagePercent = (used: number, budget: number) => (budget > 0 ? Math.min(100, Math.round((used / budget) * 100)) : 0);
const budgetUsageLabel = (used: number, budget: number) => (budget > 0 ? `${budgetUsagePercent(used, budget)}%` : "无限制");
const monthlyBudgetLabel = (budget: number) => (budget > 0 ? `$${budget}` : "无限制");
const tokenBudgetLabel = (budget: number) => (budget > 0 ? budget.toLocaleString() : "无限制");
const parseBudgetNumber = (value: string) => Math.max(0, Number(value) || 0);
const parseTokenBudget = (value: string) => Math.max(0, Math.floor(Number(value) || 0));

export function KeysPage() {
  const { keys, providers, selectedKeyId, setSelectedKey, saveKey, rotateKey, revokeKey, extendKey, pushNotice } = useAdminStore();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [form, setForm] = useState<LocalKeyRecord>(createEmptyKey());
  const [expiresAt, setExpiresAt] = useState("");
  const [busyAction, setBusyAction] = useState<"save" | "rotate" | "extend" | "revoke" | null>(null);
  const [revokeCandidate, setRevokeCandidate] = useState<LocalKeyRecord | null>(null);

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
      setExpiresAt(expiryISOStringToDateInput(active.expires_at));
    }
  }, [active]);

  const openDrawer = (key?: LocalKeyRecord) => {
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

  const closeDrawer = () => {
    setDrawerOpen(false);
    setBusyAction(null);
  };

  const spendUsage = budgetUsagePercent(form.currentSpend, form.monthlyBudget);
  const tokenUsage = budgetUsagePercent(form.currentTokens, form.tokenBudget);
  const expiryLabel = formatLocalKeyExpiryLabel(dateInputToExpiryISOString(expiresAt));

  const handleSave = async () => {
    if (selectedProviders.length === 0) {
      pushNotice({ tone: "warning", title: "请选择供应商", message: "本地 API 需要绑定至少一个允许供应商。" });
      return;
    }
    if (selectedModels.length === 0) {
      pushNotice({ tone: "warning", title: "请选择模型", message: "本地 API 需要绑定至少一个所选供应商支持的模型。" });
      return;
    }
    setBusyAction("save");
    await saveKey({
      ...form,
      allowedProviders: selectedProviders,
      allowedModels: selectedModels,
      expires_at: dateInputToExpiryISOString(expiresAt)
    });
    setBusyAction(null);
    closeDrawer();
  };

  const requestRevoke = (key: LocalKeyRecord) => {
    setRevokeCandidate(key);
  };

  const handleRevoke = async () => {
    if (!revokeCandidate) return;
    const revokedID = revokeCandidate.id;
    setBusyAction("revoke");
    await revokeKey(revokedID);
    setBusyAction(null);
    setRevokeCandidate(null);
    if (form.id === revokedID) closeDrawer();
  };

  const handleRotate = async () => {
    setBusyAction("rotate");
    await rotateKey(form.id);
    setBusyAction(null);
  };

  const handleExtend = async () => {
    setBusyAction("extend");
    await extendKey(form.id, dateInputToExpiryISOString(expiresAt));
    setBusyAction(null);
  };

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">访问控制</span>
          <h2 className="section-title">密钥管理</h2>
          <p className="section-description">Local Key 是给本地工具使用的访问凭证，用来隔离上游 Provider Key，并控制模型、预算和有效期。</p>
        </div>
        <div className="section-actions">
          <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
            <Plus size={14} /> 新建密钥
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
          const spendPct = budgetUsagePercent(key.currentSpend, key.monthlyBudget);
          return (
            <div
              key={key.id}
              className={clsx("list-row", key.id === selectedKeyId && "active")}
              style={{ "--index": index } as React.CSSProperties}
              role="button"
              tabIndex={0}
              aria-label={`编辑 ${key.name}，状态 ${keyStatusMap[key.status] ?? key.status}`}
              onClick={() => openDrawer(key)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  openDrawer(key);
                }
              }}
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
                  {key.allowedModels.length} 个模型 · 已用 ${key.currentSpend.toFixed(1)} / {monthlyBudgetLabel(key.monthlyBudget)}
                  {" · "}{formatLocalKeyExpiryLabel(key.expires_at)}
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
                  onClick={(e) => { e.stopPropagation(); requestRevoke(key); }}
                  aria-label={`吊销 ${key.name}`}
                >
                  <ShieldAlert size={14} />
                </button>
              </div>
            </div>
          );
        })}

        {keys.length === 0 && (
          <div className="empty-state">
            <span className="empty-state-title">暂无密钥</span>
            <span className="empty-state-desc">创建 Local Key 后，把它复制到 Codex、Cursor 或 Claude Desktop 等本地工具里。</span>
            <div className="empty-state-actions">
              <button type="button" className="btn btn-primary btn-sm" onClick={() => openDrawer()}>
                <Plus size={14} /> 新建密钥
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Drawer */}
      {drawerOpen && (
        <>
          <div className="drawer-overlay" onClick={closeDrawer} />
          <aside className="drawer drawer-wide" role="dialog" aria-modal="true" aria-label="Local Key 配置">
            <div className="drawer-header">
              <div>
                <h3 className="section-title">{form.name}</h3>
                <p style={{ fontSize: "0.82rem", color: "var(--text-secondary)", marginTop: 2, fontFamily: "var(--font-mono)" }}>
                  {form.displayKey}
                </p>
              </div>
              <button type="button" className="btn btn-ghost btn-icon" onClick={closeDrawer} aria-label="关闭 Local Key 配置">
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
                  <label className="form-label">允许供应商</label>
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
                  <span className="form-hint">{providerOptions.length ? "按住 Ctrl 可选择多个供应商" : "请先到供应商接入页面新增供应商"}</span>
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
                  <span className="form-hint">{modelOptions.length ? "模型来自已选供应商的模型配置" : "请先选择带有模型配置的供应商"}</span>
                </div>

                <div className="form-field">
                  <label className="form-label">月预算（美元）</label>
                  <input
                    className="form-control"
                    type="number"
                    min={0}
                    value={form.monthlyBudget}
                    onChange={(e) => setForm({ ...form, monthlyBudget: parseBudgetNumber(e.target.value) })}
                  />
                  <span className="form-hint">填 0 表示无限制，不设置月消费上限。</span>
                </div>

                <div className="form-field">
                  <label className="form-label">令牌预算</label>
                  <input
                    className="form-control"
                    type="number"
                    min={0}
                    value={form.tokenBudget}
                    onChange={(e) => setForm({ ...form, tokenBudget: parseTokenBudget(e.target.value) })}
                  />
                  <span className="form-hint">填 0 表示无限制，不设置令牌上限。</span>
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
                  <label className="form-label">有效期</label>
                  <input
                    className="form-control"
                    value={expiryLabel}
                    readOnly
                  />
                </div>
              </div>

              {/* Usage bars */}
              <div className="flex-col gap-3">
                <div className="usage-row">
                  <div className="usage-header">
                    <span className="usage-label">本月预算使用率（{monthlyBudgetLabel(form.monthlyBudget)}）</span>
                    <span className="usage-value">{budgetUsageLabel(form.currentSpend, form.monthlyBudget)}</span>
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
                    <span className="usage-label">令牌配额使用率（{tokenBudgetLabel(form.tokenBudget)}）</span>
                    <span className="usage-value">{budgetUsageLabel(form.currentTokens, form.tokenBudget)}</span>
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

              <div className="detail-card danger-zone">
                <strong>安全提示</strong>
                <span>轮换会生成新凭证，吊销会让旧凭证立即失效。执行前请确认对应工具已经切换到新的 Local Key。</span>
              </div>
            </div>

            <div className="drawer-footer">
              <button type="button" className="btn btn-primary" onClick={() => void handleSave()} disabled={busyAction !== null}>
                {busyAction === "save" ? <Loader2 size={14} className="spin" /> : null}
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
              <button type="button" className="btn btn-secondary" onClick={() => void handleRotate()} disabled={busyAction !== null}>
                {busyAction === "rotate" ? <Loader2 size={14} className="spin" /> : <RotateCcw size={14} />} 轮换密钥
              </button>
              <button type="button" className="btn btn-secondary" onClick={() => void handleExtend()} disabled={busyAction !== null}>
                {busyAction === "extend" ? <Loader2 size={14} className="spin" /> : <CalendarClock size={14} />} 保存有效期
              </button>
              <button type="button" className="btn btn-danger" onClick={() => requestRevoke(form)} disabled={busyAction !== null}>
                {busyAction === "revoke" ? <Loader2 size={14} className="spin" /> : <ShieldAlert size={14} />} 吊销
              </button>
            </div>
          </aside>
        </>
      )}

      {revokeCandidate && (
        <div className="confirm-overlay" role="presentation" onClick={() => busyAction !== "revoke" && setRevokeCandidate(null)}>
          <section
            className="confirm-dialog danger"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="revoke-local-key-title"
            aria-describedby="revoke-local-key-desc"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="confirm-dialog-icon" aria-hidden="true">
              <ShieldAlert size={22} />
            </div>
            <div className="confirm-dialog-body">
              <span className="eyebrow danger-text">高风险操作</span>
              <h3 id="revoke-local-key-title" className="confirm-dialog-title">吊销 Local Key</h3>
              <p id="revoke-local-key-desc" className="confirm-dialog-copy">
                确认吊销 <strong>{revokeCandidate.name}</strong>？吊销后使用它的本地工具会立即鉴权失败，此操作不可恢复。
              </p>
              <div className="confirm-dialog-meta">
                <span>凭证</span>
                <code>{revokeCandidate.displayKey}</code>
              </div>
            </div>
            <div className="confirm-dialog-actions">
              <button type="button" className="btn btn-secondary" onClick={() => setRevokeCandidate(null)} disabled={busyAction === "revoke"}>
                取消
              </button>
              <button type="button" className="btn btn-danger" onClick={() => void handleRevoke()} disabled={busyAction === "revoke"}>
                {busyAction === "revoke" ? <Loader2 size={14} className="spin" /> : <ShieldAlert size={14} />} 确认吊销
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
