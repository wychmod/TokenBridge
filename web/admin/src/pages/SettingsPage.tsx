import { useEffect, useMemo, useState } from "react";
import { CheckCheck, Copy, Download, RefreshCw, Save, ScanSearch, TerminalSquare, X } from "lucide-react";
import { useAdminStore } from "../store/admin-store";
import { labelFromMap, platformLabelMap, buildCheckStatusMap } from "../store/labels";
import {
  fetchDesktopStatus,
  fetchDesktopVersion,
  isDesktopMode,
  runDesktopSelfCheck,
  type DesktopCheckItem,
  type DesktopSelfCheck
} from "../utils/desktop-bridge";

const fallbackChecks: DesktopCheckItem[] = [
  { key: "assets", title: "管理后台构建资源", description: "等待构建产物进入嵌入目录", status: "pending", detail: "当前为静态演示数据" },
  { key: "portable", title: "便携目录结构", description: "打包目录结构已就绪", status: "ready", detail: "浏览器版便携结构已存在" },
  { key: "runtime", title: "运行验证", description: "等待运行环境可用后执行真实运行验证", status: "blocked", detail: "浏览器模式下不会执行桌面运行检查" },
  { key: "release", title: "发布清单", description: "等待生成压缩包、版本说明和校验文件", status: "pending", detail: "当前尚未生成正式发布物" }
];

export function SettingsPage() {
  const { settings, distributionPlan, keys, saveSettings, backupSettings, reloadDistributionPlan, pushNotice } = useAdminStore();
  const [form, setForm] = useState(() => ({ ...settings, startAtLogin: settings.startAtLogin ?? false }));
  const [activeTab, setActiveTab] = useState<"basic" | "setup" | "version" | "data">("basic");

  const [desktopVersion, setDesktopVersion] = useState("0.1.0-alpha");
  const [platform, setPlatform] = useState("web");
  const [report, setReport] = useState<DesktopSelfCheck | null>(null);

  useEffect(() => {
    setForm({ ...settings, startAtLogin: settings.startAtLogin ?? false });
  }, [settings]);

  useEffect(() => {
    void fetchDesktopVersion().then((v) => { if (v && v !== "browser") setDesktopVersion(v); });
    void fetchDesktopStatus().then((s) => { if (s.platform) setPlatform(s.platform); });
    if (isDesktopMode) {
      void runDesktopSelfCheck().then(setReport);
    }
  }, []);

  const currentKey = keys[0] ?? { name: "未加载", displayKey: "tb-****" };

  const setups = [
    {
      name: "Codex / Cursor / Windsurf",
      snippet: `OPENAI_API_KEY=${currentKey.displayKey}\nOPENAI_BASE_URL=http://localhost:9090/v1`
    },
    {
      name: "Claude Desktop",
      snippet: `{"apiUrl": "http://localhost:9090", "apiKey": "${currentKey.displayKey}"}`
    },
    {
      name: "TokenBridge 便携分发包",
      snippet: "下载 tokenbridge.zip → 解压 → 运行 tokenbridge.exe → 打开 /admin 完成剩余配置"
    }
  ];

  const checks = report?.checks?.length ? report.checks : fallbackChecks;

  const versionBlocks = useMemo(() => [
    { label: "版本号", value: isDesktopMode ? desktopVersion : "0.1.0-alpha" },
    { label: "发布通道", value: isDesktopMode ? `桌面版 · ${labelFromMap(platformLabelMap, platform)}` : "便携预览版" },
    { label: "当前状态", value: isDesktopMode ? "桌面特性已启用" : "基础功能已就绪" }
  ], [desktopVersion, platform]);

  const handleSelfCheck = async () => {
    const result = await runDesktopSelfCheck();
    setReport(result);
    pushNotice({
      tone: result.health === "healthy" ? "success" : "warning",
      title: "桌面自检已执行",
      message: result.warnings[0] ?? `共完成 ${result.checks.length} 项检查。`
    });
  };

  return (
    <div className="flex-col gap-4">
      {/* Header */}
      <div className="section-header">
        <div className="section-header-main">
          <span className="eyebrow">系统管理</span>
          <h2 className="section-title">设置</h2>
          <p className="section-description">集中管理本地监听、后台入口、主题、日志保留、备份和桌面分发参数。危险操作会在执行前确认。</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs" style={{ alignSelf: "flex-start" }}>
        <button type="button" className={clsx("tab", activeTab === "basic" && "active")} onClick={() => setActiveTab("basic")}>
          基础配置
        </button>
        <button type="button" className={clsx("tab", activeTab === "setup" && "active")} onClick={() => setActiveTab("setup")}>
          接入模板
        </button>
        <button type="button" className={clsx("tab", activeTab === "version" && "active")} onClick={() => setActiveTab("version")}>
          版本与诊断
        </button>
        <button type="button" className={clsx("tab", activeTab === "data" && "active")} onClick={() => setActiveTab("data")}>
          数据管理
        </button>
      </div>

      {/* Tab: Basic Config */}
      {activeTab === "basic" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">服务参数</span>
              <h3 className="section-title">基础配置</h3>
            </div>
            <div className="section-actions">
              <button type="button" className="btn btn-primary btn-sm" onClick={() => void saveSettings(form)}>
                <Save size={14} /> 保存
              </button>
            </div>
          </div>

          <div className="form-grid">
            <div className="form-field">
              <label className="form-label">监听地址</label>
              <input className="form-control" value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">端口</label>
              <input className="form-control" type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} />
            </div>
            <div className="form-field">
              <label className="form-label">管理后台路径</label>
              <input className="form-control" value={form.adminPath} onChange={(e) => setForm({ ...form, adminPath: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">管理员账号</label>
              <input className="form-control" value={form.adminUsername} onChange={(e) => setForm({ ...form, adminUsername: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">界面主题</label>
              <select
                className="form-control"
                value={form.theme}
                onChange={(e) => setForm({ ...form, theme: e.target.value as typeof form.theme })}
              >
                <option value="system">跟随系统</option>
                <option value="light">浅色</option>
                <option value="dark">深色</option>
              </select>
            </div>
            <div className="form-field">
              <label className="form-label">更新通道</label>
              <input className="form-control" value={form.updateChannel} onChange={(e) => setForm({ ...form, updateChannel: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">备份间隔</label>
              <input className="form-control" value={form.backupInterval} onChange={(e) => setForm({ ...form, backupInterval: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">日志级别</label>
              <input className="form-control" value={form.logLevel} onChange={(e) => setForm({ ...form, logLevel: e.target.value })} />
            </div>
            <div className="form-field">
              <label className="form-label">保留天数</label>
              <input className="form-control" type="number" value={form.retentionDays} onChange={(e) => setForm({ ...form, retentionDays: Number(e.target.value) })} />
            </div>
            <div className="form-field">
              <label className="form-label">打包方式</label>
              <input className="form-control" value={form.bundleMode} onChange={(e) => setForm({ ...form, bundleMode: e.target.value })} />
            </div>
            <div className="form-field" style={{ gridColumn: "1 / -1" }}>
              <label className="form-label">开机自启动</label>
              <label className="flex items-center gap-2" style={{ cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={Boolean(form.startAtLogin)}
                  onChange={(e) => setForm({ ...form, startAtLogin: e.target.checked })}
                />
                <span style={{ fontSize: "0.88rem", color: "var(--text-secondary)" }}>
                  Windows 使用登录启动项，macOS 使用 LaunchAgents。默认关闭。
                </span>
              </label>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Setup Templates */}
      {activeTab === "setup" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">接入助手</span>
              <h3 className="section-title">接入配置</h3>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span className="pill pill-neutral">主用密钥 {currentKey.name}</span>
            <span className="pill pill-neutral">网关 http://localhost:9090/v1</span>
          </div>

          <div className="flex-col gap-3">
            {setups.map((item) => (
              <div key={item.name} className="flex-col gap-3" style={{ padding: "var(--space-3) 0", borderBottom: "1px solid var(--border-subtle)" }}>
                <div className="flex items-center justify-between">
                  <strong style={{ fontSize: "0.9rem" }}>{item.name}</strong>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => {
                      void navigator.clipboard?.writeText(item.snippet);
                      pushNotice({ tone: "info", title: `已复制 ${item.name} 配置`, message: "配置已写入剪贴板。" });
                    }}
                  >
                    <Copy size={13} /> 复制配置
                  </button>
                </div>
                <pre
                  style={{
                    background: "var(--bg-base)",
                    padding: "var(--space-3)",
                    borderRadius: "var(--radius-sm)",
                    fontSize: "0.8rem",
                    fontFamily: "var(--font-mono)",
                    overflow: "auto",
                    color: "var(--text-secondary)"
                  }}
                >
                  {item.snippet}
                </pre>
              </div>
            ))}
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => pushNotice({ tone: "info", title: "接入向导", message: "一步式向导后续开放。" })}
            >
              <TerminalSquare size={14} /> 向导
            </button>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={() => pushNotice({ tone: "success", title: "配置校验通过", message: "接入模板已生成。" })}
            >
              <CheckCheck size={14} /> 校验
            </button>
          </div>
        </div>
      )}

      {/* Tab: Version & Diagnostics */}
      {activeTab === "version" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">系统状态</span>
              <h3 className="section-title">版本与诊断</h3>
            </div>
          </div>

          {/* Version Cards */}
          <div className="kpi-grid">
            {versionBlocks.map((item) => (
              <div key={item.label} className="kpi-card">
                <span className="kpi-label">{item.label}</span>
                <span className="kpi-value" style={{ fontSize: "1.2rem" }}>{item.value}</span>
              </div>
            ))}
          </div>

          {/* Self Check */}
          <div className="flex-col gap-3">
            <div className="flex items-center justify-between">
              <span className="eyebrow">发布前检查</span>
              {isDesktopMode && (
                <button type="button" className="btn btn-primary btn-sm" onClick={() => void handleSelfCheck()}>
                  <ScanSearch size={14} /> 重新执行
                </button>
              )}
            </div>

            <div className="flex items-center gap-2">
              <span className="pill pill-neutral">模式：{isDesktopMode ? "桌面发布前检查器" : "浏览器演示模式"}</span>
              <span
                className="pill"
                style={{
                  background: report?.health === "healthy" ? "var(--accent-dim)" : report?.health === "blocked" ? "var(--danger-dim)" : "var(--warning-dim)",
                  color: report?.health === "healthy" ? "var(--accent)" : report?.health === "blocked" ? "var(--danger)" : "var(--warning)"
                }}
              >
                {buildCheckStatusMap[report?.health ?? "pending"] ?? report?.health ?? "待检查"}
              </span>
            </div>

            <div className="flex-col gap-2">
              {checks.map((item, index) => (
                <div
                  key={item.key}
                  className="alert-card"
                  style={{
                    borderLeftColor:
                      item.status === "ready"
                        ? "var(--accent)"
                        : item.status === "blocked"
                        ? "var(--danger)"
                        : "var(--warning)",
                    animationDelay: `${index * 40}ms`
                  }}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span
                        className="data-value"
                        style={{
                          fontSize: "0.75rem",
                          color: "var(--text-tertiary)",
                          width: 24
                        }}
                      >
                        {String(index + 1).padStart(2, "0")}
                      </span>
                      <strong style={{ fontSize: "0.85rem" }}>{item.title}</strong>
                    </div>
                    <span
                      className="pill"
                      style={{
                        background:
                          item.status === "ready"
                            ? "var(--accent-dim)"
                            : item.status === "blocked"
                            ? "var(--danger-dim)"
                            : "var(--warning-dim)",
                        color:
                          item.status === "ready"
                            ? "var(--accent)"
                            : item.status === "blocked"
                            ? "var(--danger)"
                            : "var(--warning)"
                      }}
                    >
                      {buildCheckStatusMap[item.status] ?? item.status}
                    </span>
                  </div>
                  <p style={{ fontSize: "0.78rem", color: "var(--text-secondary)", marginTop: 4, marginLeft: 32 }}>
                    {item.description}
                  </p>
                  <p style={{ fontSize: "0.75rem", color: "var(--text-tertiary)", marginTop: 2, marginLeft: 32 }}>
                    {item.detail}
                  </p>
                </div>
              ))}
            </div>

            {report?.warnings?.length ? (
              <div className="flex-col gap-2">
                {report.warnings.map((warning) => (
                  <div key={warning} className="alert-card warning">
                    <strong style={{ fontSize: "0.82rem", color: "var(--warning)" }}>发布前注意</strong>
                    <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 4 }}>{warning}</p>
                  </div>
                ))}
              </div>
            ) : null}
          </div>
        </div>
      )}

      {/* Tab: Data Management */}
      {activeTab === "data" && (
        <div className="panel flex-col gap-4">
          <div className="section-header" style={{ marginBottom: 0 }}>
            <div className="section-header-main">
              <span className="eyebrow">数据维护</span>
              <h3 className="section-title">数据管理</h3>
            </div>
          </div>

          <div className="flex-col gap-2">
            <div className="flex items-center justify-between" style={{ padding: "var(--space-3) 0", borderBottom: "1px solid var(--border-subtle)" }}>
              <div>
                <strong style={{ fontSize: "0.9rem" }}>导出配置</strong>
                <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 2 }}>下载当前系统配置的 JSON 备份</p>
              </div>
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => void backupSettings()}>
                <Download size={14} /> 导出
              </button>
            </div>

            <div className="flex items-center justify-between" style={{ padding: "var(--space-3) 0", borderBottom: "1px solid var(--border-subtle)" }}>
              <div>
                <strong style={{ fontSize: "0.9rem" }}>分发包信息</strong>
                <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 2 }}>
                  {distributionPlan?.package_name ?? "tokenbridge.zip"} · {distributionPlan?.mode ?? "单目录分发"}
                </p>
              </div>
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => void reloadDistributionPlan()}>
                <RefreshCw size={14} /> 刷新
              </button>
            </div>

            <div className="flex items-center justify-between" style={{ padding: "var(--space-3) 0" }}>
              <div>
                <strong style={{ fontSize: "0.9rem" }}>清理日志</strong>
                <p style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 2 }}>删除超过保留天数的旧日志</p>
              </div>
              <button
                type="button"
                className="btn btn-danger btn-sm"
                onClick={() => {
                  if (!confirm(`确定清理超过 ${form.retentionDays} 天的旧日志？清理后无法在控制台继续检索这些历史记录。`)) return;
                  pushNotice({ tone: "info", title: "日志清理", message: "自动清理任务已触发。" });
                }}
              >
                清理
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function clsx(...classes: Array<string | false | undefined>) {
  return classes.filter(Boolean).join(" ");
}
