package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"tokenbridge/internal/aitoolusage"
)

type AIStatsWidget struct {
	ctx      context.Context
	adminURL string
	client   *http.Client
}

func parseAIStatsWidgetMode() (bool, string) {
	args := os.Args[1:]
	for index, arg := range args {
		if arg != "--ai-stats-widget" {
			continue
		}
		adminURL := ""
		for i := index + 1; i < len(args); i++ {
			if args[i] == "--admin-url" && i+1 < len(args) {
				adminURL = args[i+1]
				break
			}
		}
		return true, adminURL
	}
	return false, ""
}

func runAIStatsWidget(adminURL string) error {
	adminURL = strings.TrimRight(adminURL, "/")
	if adminURL == "" {
		return fmt.Errorf("missing --admin-url for AI stats widget")
	}

	widget := &AIStatsWidget{
		adminURL: adminURL,
		client:   &http.Client{Timeout: 6 * time.Second},
	}

	return wails.Run(&options.App{
		Title:            "TokenBridge AI Stats",
		Width:            380,
		Height:           430,
		MinWidth:         320,
		MinHeight:        360,
		Frameless:        true,
		AlwaysOnTop:      true,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(serveAIStatsWidgetHTML),
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 true,
			BackdropType:                      windows.Mica,
			DisablePinchZoom:                  true,
			EnableSwipeGestures:               false,
			Theme:                             windows.SystemDefault,
			DisableFramelessWindowDecorations: true,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		OnStartup: widget.Startup,
		Bind:      []interface{}{widget},
	})
}

func (w *AIStatsWidget) Startup(ctx context.Context) {
	w.ctx = ctx
	wailsruntime.WindowSetAlwaysOnTop(ctx, true)
	wailsruntime.WindowSetBackgroundColour(ctx, 0, 0, 0, 0)
}

func (w *AIStatsWidget) GetSnapshot() (aitoolusage.RealtimeSnapshot, error) {
	req, err := http.NewRequest(http.MethodGet, w.adminURL+"/api/ai-tool-usage/realtime", nil)
	if err != nil {
		return aitoolusage.RealtimeSnapshot{}, err
	}
	res, err := w.client.Do(req)
	if err != nil {
		return aitoolusage.RealtimeSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return aitoolusage.RealtimeSnapshot{}, fmt.Errorf("AI stats request failed: %s %s", res.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data  aitoolusage.RealtimeSnapshot `json:"data"`
		Error string                       `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return aitoolusage.RealtimeSnapshot{}, err
	}
	if payload.Error != "" {
		return aitoolusage.RealtimeSnapshot{}, fmt.Errorf("%s", payload.Error)
	}
	return payload.Data, nil
}

func (w *AIStatsWidget) SetAlwaysOnTop(enabled bool) {
	if w.ctx == nil {
		return
	}
	wailsruntime.WindowSetAlwaysOnTop(w.ctx, enabled)
}

func (w *AIStatsWidget) Close() {
	if w.ctx == nil {
		os.Exit(0)
		return
	}
	wailsruntime.Quit(w.ctx)
}

func serveAIStatsWidgetHTML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, aiStatsWidgetHTML)
}

const aiStatsWidgetHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>TokenBridge AI Stats</title>
  <style>
    :root {
      color-scheme: dark;
      --alpha: 0.88;
      --panel: rgba(13, 17, 23, var(--alpha));
      --wash-accent: rgba(102, 199, 184, 0.15);
      --wash-info: rgba(121, 175, 217, 0.11);
      --panel-soft: rgba(255, 255, 255, 0.06);
      --line: rgba(218, 232, 245, 0.14);
      --line-strong: rgba(102, 199, 184, 0.36);
      --hero-bg: rgba(4, 9, 13, 0.32);
      --tile-bg: rgba(255, 255, 255, 0.045);
      --trend-bg: rgba(255, 255, 255, 0.04);
      --status-bg: rgba(255, 255, 255, 0.055);
      --hover-bg: rgba(255, 255, 255, 0.08);
      --pulse-bg: rgba(102, 199, 184, 0.12);
      --pulse-line: rgba(102, 199, 184, 0.16);
      --bar-fade: rgba(102, 199, 184, 0.18);
      --shadow: rgba(0, 0, 0, 0.34);
      --shadow-drag: rgba(0, 0, 0, 0.28);
      --edge-light: rgba(255, 255, 255, 0.08);
      --edge-light-soft: rgba(255, 255, 255, 0.06);
      --dot-ring: rgba(102, 199, 184, 0.12);
      --info-ring: rgba(121, 175, 217, 0.12);
      --error-ring: rgba(255, 139, 150, 0.12);
      --text: rgba(248, 250, 252, 0.96);
      --muted: rgba(196, 207, 218, 0.72);
      --faint: rgba(156, 170, 184, 0.58);
      --accent: #66c7b8;
      --info: #79afd9;
      --warning: #d9ad63;
      font-family: Outfit, "Segoe UI", system-ui, sans-serif;
    }
    * { box-sizing: border-box; }
    html, body {
      width: 100%;
      height: 100%;
      margin: 0;
      overflow: hidden;
      background: transparent;
      color: var(--text);
      -webkit-font-smoothing: antialiased;
      user-select: none;
    }
    body {
      padding: 8px;
    }
    button, input { font: inherit; }
    button {
      border: 0;
      color: inherit;
      cursor: pointer;
      background: transparent;
    }
    .widget {
      width: 100%;
      height: 100%;
      display: grid;
      grid-template-rows: auto auto 1fr auto;
      gap: 12px;
      padding: 14px;
      border: 1px solid var(--line);
      border-radius: 18px;
      background:
        linear-gradient(145deg, var(--wash-accent), transparent 34%),
        linear-gradient(315deg, var(--wash-info), transparent 42%),
        var(--panel);
      box-shadow: 0 18px 42px var(--shadow), inset 0 1px 0 var(--edge-light);
      backdrop-filter: blur(14px) saturate(1.18);
      contain: paint;
      overflow: hidden;
      transition: background 140ms ease, border-color 140ms ease, box-shadow 140ms ease, backdrop-filter 140ms ease;
      text-shadow: 0 1px 10px rgba(0, 0, 0, 0.62);
    }
    .widget.is-dragging {
      box-shadow: 0 12px 30px var(--shadow-drag), inset 0 1px 0 var(--edge-light-soft);
      backdrop-filter: blur(8px) saturate(1.05);
    }
    .topbar {
      min-height: 42px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      --wails-draggable: drag;
      -webkit-app-region: drag;
      cursor: grab;
    }
    .topbar:active {
      cursor: grabbing;
    }
    .brand {
      display: inline-flex;
      align-items: center;
      gap: 9px;
      flex: 1;
      min-width: 0;
    }
    .mark {
      width: 24px;
      height: 24px;
      display: inline-grid;
      place-items: center;
      border-radius: 8px;
      color: #071311;
      background: linear-gradient(135deg, var(--accent), #b2efe5);
      font-weight: 800;
      font-size: 13px;
    }
    .brand strong {
      display: block;
      font-size: 13px;
      letter-spacing: 0;
      line-height: 1.1;
    }
    .brand span {
      display: block;
      margin-top: 2px;
      color: var(--muted);
      font-size: 11px;
      line-height: 1.1;
    }
    .actions {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      --wails-draggable: no-drag;
      -webkit-app-region: no-drag;
    }
    .icon-btn {
      width: 34px;
      height: 34px;
      display: inline-grid;
      place-items: center;
      border-radius: 10px;
      color: var(--muted);
      transition: background 140ms ease, color 140ms ease, transform 140ms ease;
    }
    .pin-btn {
      width: auto;
      min-width: 68px;
      grid-auto-flow: column;
      grid-auto-columns: max-content;
      gap: 5px;
      padding: 0 10px;
      border: 1px solid var(--line);
      font-size: 12px;
      font-weight: 650;
    }
    .pin-btn.is-pinned {
      color: #071311;
      border-color: rgba(102,199,184,0.45);
      background: linear-gradient(135deg, var(--accent), #b2efe5);
    }
    .icon-btn:hover {
      color: var(--text);
      background: var(--hover-bg);
      transform: translateY(-1px);
    }
    .pin-btn.is-pinned:hover {
      color: #071311;
      background: linear-gradient(135deg, #7bd3c7, #c3f5ed);
    }
    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 12px;
      align-items: end;
      padding: 14px;
      border: 1px solid var(--line-strong);
      border-radius: 14px;
      background: var(--hero-bg);
    }
    .hero span {
      color: var(--muted);
      font-size: 11px;
      font-weight: 600;
      text-transform: uppercase;
    }
    .hero strong {
      display: block;
      margin-top: 4px;
      font-family: "Geist Mono", "Cascadia Mono", Consolas, monospace;
      font-size: 32px;
      line-height: 1.05;
      letter-spacing: 0;
    }
    .hero em {
      display: block;
      margin-top: 6px;
      color: var(--faint);
      font-size: 12px;
      font-style: normal;
    }
    .pulse {
      width: 42px;
      height: 42px;
      display: grid;
      place-items: center;
      border-radius: 14px;
      color: var(--accent);
      background: var(--pulse-bg);
      box-shadow: 0 0 0 1px var(--pulse-line);
    }
    .stats {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
    }
    .tile {
      min-width: 0;
      padding: 11px;
      border: 1px solid var(--line);
      border-radius: 12px;
      background: var(--tile-bg);
    }
    .tile span {
      display: block;
      color: var(--muted);
      font-size: 11px;
      line-height: 1.2;
    }
    .tile strong {
      display: block;
      margin-top: 6px;
      font-family: "Geist Mono", "Cascadia Mono", Consolas, monospace;
      font-size: 18px;
      line-height: 1.1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .details {
      min-height: 0;
      display: grid;
      grid-template-rows: auto auto;
      gap: 8px;
    }
    .trend {
      display: grid;
      grid-template-columns: repeat(7, 1fr);
      align-items: end;
      gap: 5px;
      height: 62px;
      padding: 10px 10px 8px;
      border: 1px solid var(--line);
      border-radius: 12px;
      background: var(--trend-bg);
    }
    .bar {
      min-height: 4px;
      border-radius: 99px 99px 3px 3px;
      background: linear-gradient(180deg, var(--accent), var(--bar-fade));
    }
    .meta {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
      color: var(--muted);
      font-size: 12px;
    }
    .meta-card {
      min-width: 0;
      padding: 10px;
      border-radius: 12px;
      background: var(--tile-bg);
    }
    .meta-card span {
      display: block;
      color: var(--faint);
      font-size: 10px;
      text-transform: uppercase;
    }
    .meta-card strong {
      display: block;
      margin-top: 4px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 12px;
    }
    .footer {
      display: grid;
      grid-template-columns: minmax(78px, auto) minmax(0, 1fr) auto;
      gap: 10px;
      align-items: center;
      min-height: 44px;
      padding: 8px 10px;
      border: 1px solid var(--line);
      border-radius: 13px;
      background: var(--tile-bg);
      color: var(--faint);
      font-size: 11px;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      white-space: nowrap;
      min-height: 26px;
      padding: 4px 8px;
      border-radius: 999px;
      background: var(--status-bg);
      color: var(--muted);
    }
    .dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--accent);
      box-shadow: 0 0 0 4px var(--dot-ring);
    }
    .status.syncing .dot { background: var(--info); box-shadow: 0 0 0 4px var(--info-ring); }
    .status.offline .dot,
    .status.error .dot { background: #ff8b96; box-shadow: 0 0 0 4px var(--error-ring); }
    .status.error {
      color: #ffb4bd;
    }
    .opacity {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      border-radius: 999px;
      --wails-draggable: no-drag;
      -webkit-app-region: no-drag;
    }
    .opacity input {
      width: 100%;
      min-width: 120px;
      accent-color: var(--accent);
    }
    .error {
      color: #ffb4bd;
    }
  </style>
</head>
<body>
  <main class="widget">
    <header class="topbar">
      <div class="brand">
        <div class="mark">AI</div>
        <div>
          <strong>实时 AI 统计</strong>
          <span>TokenBridge AI Spend</span>
        </div>
      </div>
      <div class="actions">
        <button class="icon-btn pin-btn is-pinned" id="pin" type="button" title="窗口置顶" aria-pressed="true">
          <span id="pinIcon">↑</span><span id="pinLabel">置顶中</span>
        </button>
        <button class="icon-btn" id="refresh" type="button" title="刷新">↻</button>
        <button class="icon-btn" id="close" type="button" title="关闭">×</button>
      </div>
    </header>

    <section class="hero">
      <div>
        <span>今日花费</span>
        <strong id="todayCost">--</strong>
        <em id="todaySub">-- requests today</em>
      </div>
      <div class="pulse" title="实时刷新状态">⌁</div>
    </section>

    <section class="stats" aria-label="AI usage summary">
      <div class="tile"><span>今日请求</span><strong id="todayRequests">--</strong></div>
      <div class="tile"><span>累计请求</span><strong id="totalRequests">--</strong></div>
      <div class="tile"><span>累计花费</span><strong id="totalCost">--</strong></div>
      <div class="tile"><span>缓存命中</span><strong id="cacheHit">--</strong></div>
    </section>

    <section class="details">
      <div class="trend" id="trend" aria-label="seven day AI cost trend"></div>
      <div class="meta">
        <div class="meta-card"><span>Top Tool</span><strong id="topTool">--</strong></div>
        <div class="meta-card"><span>Top Model</span><strong id="topModel">--</strong></div>
      </div>
    </section>

    <footer class="footer">
      <span class="status live" id="statusWrap"><i class="dot"></i><span id="status">Live</span></span>
      <label class="opacity" title="调节浮窗显示强度" aria-label="调节浮窗显示强度">
        <input id="opacity" type="range" min="0" max="100" value="88" />
      </label>
      <span id="updated">--:--</span>
    </footer>
  </main>
  <script>
    const $ = (id) => document.getElementById(id);
    let bridge;
    let pinned = localStorage.getItem("ai-stats-pinned") !== "false";
    const rate = 7.2;

    function setStatus(text, state) {
      $("status").textContent = text;
      $("statusWrap").className = "status " + (state || "live");
    }

    function setPinned(next, syncNative) {
      pinned = Boolean(next);
      localStorage.setItem("ai-stats-pinned", String(pinned));
      $("pin").classList.toggle("is-pinned", pinned);
      $("pin").setAttribute("aria-pressed", String(pinned));
      $("pin").title = pinned ? "取消置顶，窗口不再覆盖其他应用" : "置顶窗口，保持浮在桌面上";
      $("pinIcon").textContent = pinned ? "↑" : "↕";
      $("pinLabel").textContent = pinned ? "置顶中" : "置顶";
      if (syncNative && bridge) bridge.SetAlwaysOnTop(pinned);
    }

    function setOpacity(value) {
      const raw = Number(value);
      const normalized = Math.max(0, Math.min(100, Number.isFinite(raw) ? raw : 88));
      const root = document.documentElement;
      const panelAlpha = normalized / 100;
      const chromeScale = Math.min(1, normalized / 88);
      const rgba = (r, g, b, a) => "rgba(" + r + ", " + g + ", " + b + ", " + (a * chromeScale).toFixed(3) + ")";
      root.style.setProperty("--alpha", panelAlpha.toFixed(2));
      root.style.setProperty("--wash-accent", rgba(102, 199, 184, 0.15));
      root.style.setProperty("--wash-info", rgba(121, 175, 217, 0.11));
      root.style.setProperty("--panel-soft", rgba(255, 255, 255, 0.06));
      root.style.setProperty("--line", rgba(218, 232, 245, 0.14));
      root.style.setProperty("--line-strong", rgba(102, 199, 184, 0.36));
      root.style.setProperty("--hero-bg", rgba(4, 9, 13, 0.32));
      root.style.setProperty("--tile-bg", rgba(255, 255, 255, 0.045));
      root.style.setProperty("--trend-bg", rgba(255, 255, 255, 0.04));
      root.style.setProperty("--status-bg", rgba(255, 255, 255, 0.055));
      root.style.setProperty("--hover-bg", rgba(255, 255, 255, 0.08));
      root.style.setProperty("--pulse-bg", rgba(102, 199, 184, 0.12));
      root.style.setProperty("--pulse-line", rgba(102, 199, 184, 0.16));
      root.style.setProperty("--bar-fade", rgba(102, 199, 184, 0.18));
      root.style.setProperty("--shadow", rgba(0, 0, 0, 0.34));
      root.style.setProperty("--shadow-drag", rgba(0, 0, 0, 0.28));
      root.style.setProperty("--edge-light", rgba(255, 255, 255, 0.08));
      root.style.setProperty("--edge-light-soft", rgba(255, 255, 255, 0.06));
      root.style.setProperty("--dot-ring", rgba(102, 199, 184, 0.12));
      root.style.setProperty("--info-ring", rgba(121, 175, 217, 0.12));
      root.style.setProperty("--error-ring", rgba(255, 139, 150, 0.12));
      $("opacity").value = String(normalized);
      $("opacity").title = "调节浮窗背景和边框透明度，文字保持显示，当前 " + normalized + "%";
      $("opacity").closest(".opacity")?.setAttribute("aria-label", $("opacity").title);
      localStorage.setItem("ai-stats-opacity", String(normalized));
    }

    function money(usd) {
      const cny = Number(usd || 0) * rate;
      return "¥" + cny.toLocaleString(undefined, { minimumFractionDigits: cny >= 10 ? 2 : 4, maximumFractionDigits: cny >= 10 ? 2 : 4 });
    }

    function smallUSD(usd) {
      return "$" + Number(usd || 0).toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 });
    }

    function compact(value) {
      return Number(value || 0).toLocaleString();
    }

    function pct(value) {
      return (Number(value || 0) * 100).toFixed(1) + "%";
    }

    async function waitForBridge() {
      for (let i = 0; i < 80; i++) {
        if (window.go?.main?.AIStatsWidget) return window.go.main.AIStatsWidget;
        await new Promise((resolve) => setTimeout(resolve, 50));
      }
      throw new Error("AI stats bridge is not ready");
    }

    async function loadStats() {
      try {
        setStatus("Syncing", "syncing");
        const data = await bridge.GetSnapshot();
        render(data || {});
        setStatus("Live", "live");
      } catch (error) {
        setStatus("Offline", "offline");
      }
    }

    function render(data) {
      const today = data.today || {};
      const total = data.total || {};
      $("todayCost").textContent = money(today.total_cost_usd);
      $("todaySub").textContent = smallUSD(today.total_cost_usd) + " · " + compact(today.total_requests) + " requests today";
      $("todayRequests").textContent = compact(today.total_requests);
      $("totalRequests").textContent = compact(total.total_requests);
      $("totalCost").textContent = money(total.total_cost_usd);
      $("cacheHit").textContent = pct(total.cache_hit_rate);
      $("topTool").textContent = data.top_tool ? data.top_tool.name + " · " + money(data.top_tool.cost_usd) : "No recent tool";
      $("topModel").textContent = data.top_model ? data.top_model.name + " · " + money(data.top_model.cost_usd) : "No recent model";
      const updated = data.updated_at ? new Date(data.updated_at) : new Date();
      $("updated").textContent = updated.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
      renderTrend(data.trend || []);
    }

    function renderTrend(points) {
      const root = $("trend");
      root.innerHTML = "";
      const max = Math.max(0.000001, ...points.map((point) => Number(point.cost_usd || 0)));
      for (const point of points.slice(-7)) {
        const bar = document.createElement("div");
        bar.className = "bar";
        bar.style.height = Math.max(6, Math.round((Number(point.cost_usd || 0) / max) * 48)) + "px";
        bar.title = point.day + ": " + money(point.cost_usd) + " · " + compact(point.requests) + " requests";
        root.appendChild(bar);
      }
      while (root.children.length < 7) {
        const bar = document.createElement("div");
        bar.className = "bar";
        bar.style.height = "4px";
        root.prepend(bar);
      }
    }

    $("opacity").addEventListener("input", (event) => setOpacity(event.target.value));
    $("refresh").addEventListener("click", () => loadStats());
    $("pin").addEventListener("click", () => setPinned(!pinned, true));
    $("close").addEventListener("click", () => bridge?.Close());

    const widget = document.querySelector(".widget");
    const topbar = document.querySelector(".topbar");
    topbar.addEventListener("pointerdown", (event) => {
      if (event.target.closest("button")) return;
      widget.classList.add("is-dragging");
    });
    window.addEventListener("pointerup", () => widget.classList.remove("is-dragging"));
    window.addEventListener("blur", () => widget.classList.remove("is-dragging"));

    setOpacity(localStorage.getItem("ai-stats-opacity") || 88);
    setPinned(pinned, false);
    waitForBridge()
      .then((resolved) => {
        bridge = resolved;
        setPinned(pinned, true);
        return loadStats();
      })
      .then(() => setInterval(loadStats, 12000))
      .catch(() => {
        setStatus("Bridge error", "error");
      });
  </script>
</body>
</html>`
