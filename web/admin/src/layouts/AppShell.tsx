import { PropsWithChildren, useEffect, useMemo, useState } from "react";
import {
  Activity,
  BarChart3,
  Bell,
  Bot,
  DollarSign,
  FileCheck2,
  Hash,
  KeyRound,
  LayoutDashboard,
  LockKeyhole,
  Maximize2,
  Minimize2,
  Minus,
  Moon,
  Network,
  PackageCheck,
  Rocket,
  Route,
  ScrollText,
  Settings,
  ShieldCheck,
  Sparkles,
  Sun,
  SunMoon,
  WandSparkles,
  X
} from "lucide-react";
import { NavLink, useLocation, useNavigate } from "react-router-dom";
import clsx from "clsx";
import { useUIStore } from "../store/ui-store";
import { useAdminStore } from "../store/admin-store";
import {
  fetchDesktopStatus,
  fetchDesktopVersion,
  fetchRuntimeSummary,
  fetchWindowState,
  hideDesktopToTray,
  isDesktopMode,
  minimiseDesktopWindow,
  onDesktopDomReady,
  onDesktopNotice,
  onDesktopRestoreRoute,
  onDesktopSelfCheck,
  onDesktopStatus,
  onDesktopWindowHidden,
  onDesktopWindowShown,
  openDesktopAdminInBrowser,
  persistWindowState,
  showDesktopWindow,
  toggleDesktopMaximise,
  type DesktopRuntimeSummary,
  type DesktopStatus,
  type DesktopWindowState
} from "../utils/desktop-bridge";

const navItems = [
  { to: "/dashboard", label: "总览", icon: LayoutDashboard, group: "运行", description: "网关是否正常、花费多少、哪里需要处理" },
  { to: "/providers", label: "Provider 接入", icon: Network, group: "配置", description: "上游厂商、模型、优先级和连接健康" },
  { to: "/keys", label: "Local Keys", icon: KeyRound, group: "配置", description: "本地密钥、权限、预算、轮换和吊销" },
  { to: "/routing", label: "路由策略", icon: Route, group: "配置", description: "模型别名、主链路、Fallback 和路由模拟" },
  { to: "/analytics", label: "调用分析", icon: BarChart3, group: "观测", description: "请求量、成功率、成本和模型分布趋势" },
  { to: "/ai-tool-usage", label: "AI 工具用量", icon: Bot, group: "观测", description: "本地 AI Coding 工具成本与日志来源" },
  { to: "/logs", label: "请求日志", icon: ScrollText, group: "观测", description: "Trace、状态码、Fallback、错误原因和导出" },
  { to: "/pricing", label: "模型定价", icon: DollarSign, group: "观测", description: "模型价格、上下文、能力标签和费用估算" },
  { to: "/quick-setup", label: "接入助手", icon: Rocket, group: "交付", description: "复制工具配置并验证本地网关地址" },
  { to: "/bootstrap", label: "启用引导", icon: WandSparkles, group: "交付", description: "首次运行边界、安全基线和下一步" },
  { to: "/security", label: "安全中心", icon: LockKeyhole, group: "交付", description: "管理员入口、本地监听和敏感数据边界" },
  { to: "/release-status", label: "发布状态", icon: PackageCheck, group: "交付", description: "便携包、嵌入资源和桌面交付状态" },
  { to: "/version", label: "版本信息", icon: Hash, group: "交付", description: "运行形态、版本、安全状态和平台基线" },
  { to: "/build-checks", label: "构建检查", icon: FileCheck2, group: "交付", description: "发布前资源、数据库、端口和打包检查" },
  { to: "/bootstrap/success", label: "完成", icon: ShieldCheck, group: "交付", description: "初始化完成状态", hidden: true },
  { to: "/settings", label: "系统设置", icon: Settings, group: "系统", description: "监听、主题、备份、日志保留和分发参数" }
];

const navGroups = ["运行", "配置", "观测", "交付", "系统"] as const;

const themeMeta = {
  light: { label: "浅色", icon: Sun },
  dark: { label: "深色", icon: Moon },
  system: { label: "跟随系统", icon: SunMoon }
} as const;

const fallbackWindowState: DesktopWindowState = {
  width: 1360,
  height: 860,
  x: 120,
  y: 80,
  maximised: false,
  lastRoute: "/dashboard",
  hiddenToTray: false
};

const fallbackDesktopStatus: DesktopStatus = {
  version: "browser",
  platform: "web",
  serverAddr: "",
  adminUrl: "",
  windowTitle: "TokenBridge",
  desktopMode: false,
  notifications: false,
  customChrome: false,
  trayEnabled: false,
  hideToTrayEnabled: false,
  stateRestore: false,
  windowState: fallbackWindowState,
  runtime: { providers: 0, keys: 0, rules: 0, health: "browser" },
  configSummary: { host: "127.0.0.1", port: 0, adminPath: "/admin", theme: "system", bundleMode: "browser", updateChannel: "stable" }
};

const NOTICE_AUTO_DISMISS_MS = 20_000;

export function AppShell({ children }: PropsWithChildren) {
  const { theme, setTheme } = useUIStore();
  const { notices, dismissNotice, providers, pushNotice, hydrate } = useAdminStore();
  const location = useLocation();
  const navigate = useNavigate();
  const [desktopStatus, setDesktopStatus] = useState<DesktopStatus>(fallbackDesktopStatus);
  const [desktopVersion, setDesktopVersion] = useState("browser");
  const [desktopMaximised, setDesktopMaximised] = useState(false);
  const [runtimeSummary, setRuntimeSummary] = useState<DesktopRuntimeSummary>(fallbackDesktopStatus.runtime);
  const [windowState, setWindowState] = useState<DesktopWindowState>(fallbackWindowState);

  useEffect(() => {
    const root = document.documentElement;
    const isDark = theme === "dark" || (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    root.dataset.theme = isDark ? "dark" : "light";
    root.dataset.desktop = isDesktopMode ? "true" : "false";
  }, [theme]);

  useEffect(() => {
    if (isDesktopMode) {
      persistWindowState({ ...windowState, lastRoute: location.pathname || "/dashboard" });
    }
  }, [location.pathname, windowState]);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  useEffect(() => {
    if (notices.length === 0) return;
    const timers = notices.map((notice) => {
      const elapsed = Date.now() - notice.createdAt;
      const delay = Math.max(0, NOTICE_AUTO_DISMISS_MS - elapsed);
      return window.setTimeout(() => dismissNotice(notice.id), delay);
    });
    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [notices, dismissNotice]);

  useEffect(() => {
    void fetchDesktopStatus().then((status) => {
      setDesktopStatus(status);
      setRuntimeSummary(status.runtime);
      setWindowState(status.windowState);
      setDesktopMaximised(status.windowState.maximised);
    });
    void fetchDesktopVersion().then(setDesktopVersion);
    void fetchRuntimeSummary().then(setRuntimeSummary);
    void fetchWindowState().then((state) => {
      setWindowState(state);
      setDesktopMaximised(state.maximised);
    });

    const offReady = onDesktopStatus((payload) => {
      setDesktopStatus(payload);
      setRuntimeSummary(payload.runtime);
      setWindowState(payload.windowState);
      pushNotice({ tone: "success", title: "桌面服务已就绪", message: `桌面后端已启动，监听地址 ${payload.serverAddr || "本地内嵌"}。` });
    });
    const offDom = onDesktopDomReady((payload) => setDesktopStatus(payload));
    const offNotice = onDesktopNotice((payload) => {
      pushNotice({ tone: "info", title: payload.title, message: payload.message });
    });
    const offCheck = onDesktopSelfCheck((payload) => {
      pushNotice({ tone: payload.health === "healthy" ? "success" : "warning", title: "桌面自检完成", message: payload.warnings[0] ?? "所有桌面能力检查通过。" });
    });
    const offRoute = onDesktopRestoreRoute((route) => {
      if (route && route !== location.pathname) navigate(route);
    });
    const offHidden = onDesktopWindowHidden((state) => {
      setWindowState(state);
      pushNotice({ tone: "info", title: "已隐藏到托盘", message: "主窗口已隐藏，可从托盘菜单恢复。" });
    });
    const offShown = onDesktopWindowShown((state) => {
      setWindowState(state);
      pushNotice({ tone: "success", title: "窗口已恢复", message: "主窗口已从托盘恢复显示。" });
    });

    const handler = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && event.key.toLowerCase() === "l") {
        event.preventDefault();
        navigate("/logs");
        pushNotice({ tone: "info", title: "快捷键已触发", message: "已通过 Ctrl/⌘ + Shift + L 打开运行日志页面。" });
      }
    };

    window.addEventListener("keydown", handler);
    return () => {
      offReady(); offDom(); offNotice(); offCheck(); offRoute(); offHidden(); offShown();
      window.removeEventListener("keydown", handler);
    };
  }, [location.pathname, navigate, pushNotice]);

  const currentPage = useMemo(() => {
    return [...navItems]
      .sort((a, b) => b.to.length - a.to.length)
      .find((item) => location.pathname === item.to || location.pathname.startsWith(`${item.to}/`)) ?? navItems[0];
  }, [location.pathname]);
  const healthyProviders = providers.filter((provider) => provider.status === "healthy").length;
  const resolvedTheme = themeMeta[theme];
  const ThemeIcon = resolvedTheme.icon;
  const deliveryPagesReady = ["/bootstrap", "/security", "/release-status", "/version", "/build-checks", "/quick-setup"].length;

  const handleToggleMaximise = () => {
    const next = !desktopMaximised;
    setDesktopMaximised(next);
    const nextState = { ...windowState, maximised: next };
    setWindowState(nextState);
    persistWindowState(nextState);
    toggleDesktopMaximise();
  };

  const cycleTheme = () => {
    const order = ["light", "dark", "system"] as const;
    const next = order[(order.indexOf(theme) + 1) % order.length];
    setTheme(next);
  };

  return (
    <div className="app-shell">
      {/* Desktop Titlebar */}
      {desktopStatus.desktopMode ? (
        <header className="desktop-titlebar">
          <div className="desktop-titlebar__brand">
            <Activity size={14} />
            <span>{desktopStatus.windowTitle}</span>
            <span style={{ color: "var(--text-tertiary)", fontSize: "0.75rem" }}>
              {desktopStatus.desktopMode ? `桌面版 ${desktopVersion}` : "浏览器版"}
            </span>
          </div>
          <div className="desktop-titlebar__actions">
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => openDesktopAdminInBrowser()}>
              <span style={{ fontSize: "0.75rem" }}>外部打开</span>
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => hideDesktopToTray()}>
              <span style={{ fontSize: "0.75rem" }}>隐藏到托盘</span>
            </button>
            <button type="button" className="btn btn-ghost btn-icon" onClick={() => minimiseDesktopWindow()} aria-label="最小化窗口">
              <Minus size={14} />
            </button>
            <button type="button" className="btn btn-ghost btn-icon" onClick={handleToggleMaximise} aria-label={desktopMaximised ? "还原窗口" : "最大化窗口"}>
              {desktopMaximised ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
            </button>
            <button type="button" className="btn btn-danger btn-icon" onClick={() => hideDesktopToTray()} title="隐藏到托盘" aria-label="隐藏到托盘">
              <X size={14} />
            </button>
          </div>
        </header>
      ) : null}

      {/* Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-brand">
          <Activity size={18} strokeWidth={2.5} />
          <span>TokenBridge</span>
        </div>

        <nav className="sidebar-nav" aria-label="TokenBridge 主导航">
          {navGroups.map((group) => {
            const items = navItems.filter((item) => item.group === group && !item.hidden);
            if (items.length === 0) return null;
            return (
              <div key={group} className="nav-group">
                <span className="nav-group-label">{group}</span>
                {items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      className={({ isActive }) => clsx("nav-item", isActive && "active")}
                      title={item.label}
                      aria-label={`${item.label}：${item.description}`}
                    >
                      <Icon size={18} className="nav-icon" />
                      <span className="nav-label">{item.label}</span>
                    </NavLink>
                  );
                })}
              </div>
            );
          })}
        </nav>

        <div className="sidebar-footer">
          <div className="flex items-center gap-2">
            <span className="status-indicator" style={{ background: healthyProviders > 0 ? "var(--accent)" : "var(--text-tertiary)", boxShadow: healthyProviders > 0 ? "0 0 6px var(--accent-glow)" : "none" }} />
            <span>{healthyProviders}/{providers.length} 厂商在线</span>
          </div>
          <div className="flex items-center gap-2">
            <Sparkles size={13} />
            <span>{deliveryPagesReady} 项交付页</span>
          </div>
          {desktopStatus.desktopMode && (
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => showDesktopWindow()} title="恢复窗口">
              显示窗口
            </button>
          )}
        </div>
      </aside>

      {/* Main Column */}
      <div className="main-column">
        {/* Topbar */}
        <header className="topbar">
          <div className="topbar-leading">
            <h1 className="page-title" style={{ fontSize: "1.15rem" }}>{currentPage?.label ?? "TokenBridge 控制台"}</h1>
            <p className="page-kicker">{currentPage?.description ?? "本地 AI 网关控制平面"}</p>
          </div>
          <div className="topbar-actions">
            <button
              type="button"
              className="btn btn-ghost btn-icon"
              title={`当前主题：${resolvedTheme.label}，点击切换`}
              aria-label={`当前主题：${resolvedTheme.label}，点击切换`}
              onClick={cycleTheme}
            >
              <ThemeIcon size={16} />
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-icon"
              title="通知"
              aria-label="打开通知和日志"
              onClick={() => navigate("/logs")}
              style={{ position: "relative" }}
            >
              <Bell size={16} />
              {notices.length > 0 && (
                <span style={{
                  position: "absolute",
                  top: 4,
                  right: 4,
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  background: "var(--accent)",
                  boxShadow: "0 0 0 2px var(--bg-surface), 0 0 6px var(--accent-glow)"
                }} />
              )}
            </button>
          </div>
        </header>

        {/* Notices */}
        {notices.length > 0 && (
          <section className="notice-stack" aria-live="polite" aria-label="系统通知">
            {notices.map((notice) => (
              <article key={notice.id} className={clsx("notice-item", notice.tone)}>
                <div className="notice-body">
                  <strong>{notice.title}</strong>
                  <p>{notice.message}</p>
                </div>
                <button type="button" className="btn btn-ghost btn-icon" onClick={() => dismissNotice(notice.id)} aria-label="关闭通知">
                  <X size={14} />
                </button>
              </article>
            ))}
          </section>
        )}

        {/* Page Content */}
        <main className="page-content page-enter">{children}</main>
      </div>
    </div>
  );
}
