package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
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
	ctx       context.Context
	adminURL  string
	client    *http.Client
	overlayMu sync.Mutex
	overlay   *aiStatsNativeOverlay
}

type WidgetHitRect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

func ParseAIStatsWidgetMode() (bool, string) {
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

func RunAIStatsWidget(adminURL string) error {
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
		Width:            340,
		Height:           280,
		MinWidth:         300,
		MinHeight:        220,
		Frameless:        true,
		AlwaysOnTop:      false,
		DisableResize:    false,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(serveAIStatsWidgetHTML),
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 true,
			BackdropType:                      windows.None,
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
	wailsruntime.WindowSetAlwaysOnTop(ctx, false)
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

func (w *AIStatsWidget) SetTransparentHitMode(enabled bool, rects []WidgetHitRect) {
	if w.ctx == nil {
		return
	}
	_ = applyAIStatsTransparentHitMode(enabled, rects)
}

func (w *AIStatsWidget) SupportsNativeOverlay() bool {
	return aiStatsNativeOverlaySupported()
}

func (w *AIStatsWidget) ShowNativeOverlay() bool {
	if w.ctx == nil {
		return false
	}
	snapshot, err := w.GetSnapshot()
	if err != nil {
		snapshot = aitoolusage.RealtimeSnapshot{}
	}
	x, y := wailsruntime.WindowGetPosition(w.ctx)

	w.overlayMu.Lock()
	if w.overlay == nil {
		w.overlay = newAIStatsNativeOverlay(
			func() { go w.HideNativeOverlay() },
			func() { go w.RefreshNativeOverlay() },
			func() { go w.Close() },
		)
	}
	overlay := w.overlay
	w.overlayMu.Unlock()

	if !overlay.Show(snapshot, x, y) {
		return false
	}
	wailsruntime.WindowSetAlwaysOnTop(w.ctx, false)
	wailsruntime.WindowHide(w.ctx)
	return true
}

func (w *AIStatsWidget) HideNativeOverlay() bool {
	w.overlayMu.Lock()
	overlay := w.overlay
	w.overlayMu.Unlock()
	if overlay != nil {
		overlay.Close()
	}
	if w.ctx != nil {
		wailsruntime.WindowSetAlwaysOnTop(w.ctx, false)
		wailsruntime.WindowShow(w.ctx)
	}
	return true
}

func (w *AIStatsWidget) RefreshNativeOverlay() bool {
	w.overlayMu.Lock()
	overlay := w.overlay
	w.overlayMu.Unlock()
	if overlay == nil {
		return false
	}
	snapshot, err := w.GetSnapshot()
	if err != nil {
		return false
	}
	overlay.UpdateSnapshot(snapshot)
	return true
}

func (w *AIStatsWidget) Close() {
	_ = applyAIStatsTransparentHitMode(false, nil)
	w.overlayMu.Lock()
	overlay := w.overlay
	w.overlayMu.Unlock()
	if overlay != nil {
		overlay.Close()
	}
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
      --text: rgba(248, 250, 252, 0.98);
      --muted: rgba(202, 213, 226, 0.78);
      --faint: rgba(161, 176, 190, 0.62);
      --accent: #66c7b8;
      --accent-strong: #9ff4e8;
      --info: #8bc7ff;
      --line: rgba(218, 232, 245, 0.14);
      --line-strong: rgba(102, 199, 184, 0.34);
      --panel: rgba(12, 16, 22, 0.88);
      --panel-soft: rgba(255, 255, 255, 0.052);
      --panel-strong: rgba(3, 8, 14, 0.44);
      font-family: "Segoe UI", system-ui, sans-serif;
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
      padding: 0;
      background: transparent;
    }
    button { border: 0; color: inherit; cursor: pointer; background: transparent; font: inherit; }
    .widget {
      width: 100%;
      height: 100%;
      display: grid;
      grid-template-rows: 32px auto minmax(106px, 1fr);
      gap: 7px;
      padding: 10px;
      border: 1px solid var(--line);
      border-radius: 16px;
      background:
        linear-gradient(145deg, rgba(102, 199, 184, 0.14), transparent 34%),
        linear-gradient(315deg, rgba(139, 199, 255, 0.10), transparent 44%),
        var(--panel);
      box-shadow: 0 18px 42px rgba(0, 0, 0, 0.34), inset 0 1px 0 rgba(255,255,255,0.08);
      backdrop-filter: blur(14px) saturate(1.16);
      overflow: hidden;
      contain: paint;
      transition: background 150ms ease, border-color 150ms ease, box-shadow 150ms ease, backdrop-filter 150ms ease;
    }
    .topbar {
      min-width: 0;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      --wails-draggable: drag;
      -webkit-app-region: drag;
      cursor: grab;
    }
    .topbar:active { cursor: grabbing; }
    .brand {
      flex: 1;
      min-width: 0;
      display: inline-flex;
      align-items: center;
      gap: 8px;
    }
    .brand > div:last-child {
      min-width: 0;
    }
    .mark {
      width: 24px;
      height: 24px;
      display: inline-grid;
      place-items: center;
      border-radius: 8px;
      color: #06110f;
      background: linear-gradient(135deg, var(--accent), #c8fff6);
      font-size: 12px;
      font-weight: 850;
    }
    .brand strong {
      display: block;
      font-size: 12px;
      line-height: 1.1;
      letter-spacing: 0;
    }
    .brand span {
      display: block;
      margin-top: 1px;
      color: var(--faint);
      font-size: 10px;
      line-height: 1.1;
    }
    .brand-meta {
      display: inline-flex;
      align-items: center;
      gap: 7px;
      min-width: 0;
      margin-top: 2px;
      color: var(--faint);
      font-size: 10px;
      line-height: 1;
    }
    .actions {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      --wails-draggable: no-drag;
      -webkit-app-region: no-drag;
    }
    .icon-btn {
      width: 32px;
      height: 32px;
      display: inline-grid;
      place-items: center;
      border-radius: 10px;
      color: var(--muted);
      background: rgba(255,255,255,0.035);
      transition: color 140ms ease, background 140ms ease, transform 140ms ease;
    }
    .icon-btn svg {
      width: 15px;
      height: 15px;
      stroke-width: 2.3;
    }
    .icon-btn:hover {
      color: var(--text);
      background: rgba(255,255,255,0.08);
      transform: translateY(-1px);
    }
    .icon-btn.is-pinned {
      color: #06110f;
      background: linear-gradient(135deg, var(--accent), #c8fff6);
    }
    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: end;
      gap: 10px;
      padding: 10px 11px;
      border: 1px solid var(--line-strong);
      border-radius: 13px;
      background: var(--panel-strong);
    }
    .metric-label {
      display: block;
      color: var(--muted);
      font-size: 10px;
      font-weight: 700;
      line-height: 1;
      text-transform: uppercase;
    }
    .metric-value {
      display: block;
      margin-top: 5px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-family: "Cascadia Mono", Consolas, monospace;
      font-weight: 850;
      letter-spacing: 0;
      line-height: 1;
    }
    .hero .metric-value {
      color: #f8fffd;
      font-size: 28px;
    }
    .hero-sub {
      display: block;
      margin-top: 5px;
      color: var(--faint);
      font-size: 11px;
      font-style: normal;
      line-height: 1.2;
    }
    .pulse {
      width: 34px;
      height: 34px;
      display: grid;
      place-items: center;
      border-radius: 12px;
      color: var(--accent-strong);
      background: rgba(102, 199, 184, 0.11);
      box-shadow: 0 0 0 1px rgba(102,199,184,0.18);
      font-weight: 850;
    }
    .stats {
      min-height: 0;
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 6px;
    }
    .tile {
      min-width: 0;
      padding: 8px 9px;
      border: 1px solid var(--line);
      border-radius: 11px;
      background: var(--panel-soft);
    }
    .tile .metric-value {
      color: var(--text);
      font-size: 15px;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
      white-space: nowrap;
    }
    .dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--accent);
      box-shadow: 0 0 0 4px rgba(102,199,184,0.12);
    }
    .status.syncing .dot { background: var(--info); box-shadow: 0 0 0 4px rgba(139,199,255,0.13); }
    .status.offline .dot,
    .status.error .dot { background: #ff8b96; box-shadow: 0 0 0 4px rgba(255,139,150,0.13); }
    .status.error { color: #ffb4bd; }
    .transparent-metrics {
      display: none;
    }
    html.is-transparent,
    html.is-transparent body {
      background: transparent !important;
    }
    body.is-transparent {
      padding: 4px 8px;
      background: transparent !important;
    }
    body.is-transparent,
    body.is-transparent .widget,
    body.is-transparent .topbar,
    body.is-transparent .actions,
    body.is-transparent .transparent-metrics {
      background: transparent !important;
    }
    body.is-transparent *,
    body.is-transparent *::before,
    body.is-transparent *::after {
      border-color: transparent !important;
      background-color: transparent !important;
      background-image: none !important;
      box-shadow: none !important;
      backdrop-filter: none !important;
      -webkit-backdrop-filter: none !important;
    }
    body.is-transparent .widget {
      grid-template-rows: 28px max-content 1fr;
      gap: 0;
      padding: 2px 6px;
      border-color: transparent;
      background: transparent !important;
      box-shadow: none;
      backdrop-filter: none;
      text-shadow: none;
    }
    body.is-transparent .brand,
    body.is-transparent .hero,
    body.is-transparent .stats,
    body.is-transparent .brand-meta {
      display: none;
    }
    body.is-transparent .topbar {
      justify-content: flex-end;
      pointer-events: none;
    }
    body.is-transparent .actions {
      pointer-events: auto;
    }
    body.is-transparent .icon-btn {
      color: #f8fffd;
      background: transparent;
      box-shadow: none;
      backdrop-filter: none;
      width: 28px;
      height: 28px;
      filter:
        drop-shadow(0 1px 1px rgba(0,0,0,0.86))
        drop-shadow(0 0 1px rgba(0,0,0,0.8));
    }
    body.is-transparent .icon-btn.is-pinned {
      color: #9ff4e8;
      background: transparent;
    }
    body.is-transparent .icon-btn:hover {
      color: #ffffff;
      background: transparent;
      transform: translateY(-1px) scale(1.04);
    }
    body.is-transparent .transparent-metrics {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 5px 12px;
      align-self: start;
      padding: 2px 0 0;
      pointer-events: none;
    }
    body.is-transparent .transparent-item {
      min-width: 0;
    }
    body.is-transparent .transparent-item span {
      display: block;
      color: #b8fff6;
      font-family: "Microsoft YaHei UI", "Segoe UI", system-ui, sans-serif;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0;
      line-height: 1;
      -webkit-text-stroke: 0.2px rgba(0,5,8,0.9);
      text-shadow:
        0 2px 1px rgba(0,5,8,0.95),
        1px 0 1px rgba(0,5,8,0.88),
        -1px 0 1px rgba(0,5,8,0.88),
        0 0 3px rgba(0,5,8,0.8);
    }
    body.is-transparent .transparent-item strong {
      display: block;
      margin-top: 2px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: #ffffff;
      font-family: "Segoe UI Variable Display", "Segoe UI", "Microsoft YaHei UI", system-ui, sans-serif;
      font-size: 20px;
      font-weight: 800;
      letter-spacing: 0;
      line-height: 1;
      -webkit-text-stroke: 0.25px rgba(0,5,8,0.9);
      text-shadow:
        0 2px 1px rgba(0,5,8,0.95),
        2px 0 1px rgba(0,5,8,0.9),
        -2px 0 1px rgba(0,5,8,0.9),
        0 -1px 1px rgba(0,5,8,0.88),
        0 0 4px rgba(0,5,8,0.82);
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
          <span class="brand-meta">
            <span class="status live" id="statusWrap"><i class="dot"></i><span id="status">Live</span></span>
            <span id="updated">--:--</span>
          </span>
        </div>
      </div>
      <div class="actions" id="actions">
        <button class="icon-btn" id="pin" type="button" title="置顶并进入透明模式" aria-label="置顶并进入透明模式" aria-pressed="false">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 17v5" /><path d="M9 10.76 6.12 13.64a1 1 0 0 0 .71 1.7h10.34a1 1 0 0 0 .71-1.7L15 10.76V5l1.3-1.3a1 1 0 0 0-.7-1.7H8.4a1 1 0 0 0-.7 1.7L9 5z" /></svg>
        </button>
        <button class="icon-btn" id="refresh" type="button" title="刷新统计" aria-label="刷新统计">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" /><path d="M3 21v-5h5" /><path d="M3 12a9 9 0 0 1 15.74-6.26L21 8" /><path d="M16 8h5V3" /></svg>
        </button>
        <button class="icon-btn" id="close" type="button" title="关闭" aria-label="关闭">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M18 6 6 18" /><path d="m6 6 12 12" /></svg>
        </button>
      </div>
    </header>

    <section class="hero">
      <div>
        <span class="metric-label">今日花费</span>
        <strong class="metric-value" id="todayCost">--</strong>
        <em class="hero-sub" id="todaySub">-- requests today</em>
      </div>
      <div class="pulse" title="实时刷新状态">AI</div>
    </section>

    <section class="stats" aria-label="AI usage summary">
      <div class="tile"><span class="metric-label">今日请求</span><strong class="metric-value" id="todayRequests">--</strong></div>
      <div class="tile"><span class="metric-label">累计请求</span><strong class="metric-value" id="totalRequests">--</strong></div>
      <div class="tile"><span class="metric-label">累计花费</span><strong class="metric-value" id="totalCost">--</strong></div>
      <div class="tile"><span class="metric-label">缓存命中</span><strong class="metric-value" id="cacheHit">--</strong></div>
    </section>

    <section class="transparent-metrics" aria-label="transparent AI usage summary">
      <div class="transparent-item"><span>今日花费</span><strong id="floatTodayCost">--</strong></div>
      <div class="transparent-item"><span>今日请求</span><strong id="floatTodayRequests">--</strong></div>
      <div class="transparent-item"><span>累计花费</span><strong id="floatTotalCost">--</strong></div>
      <div class="transparent-item"><span>累计请求</span><strong id="floatTotalRequests">--</strong></div>
    </section>

  </main>
  <script>
    const $ = (id) => document.getElementById(id);
    let bridge;
    let pinned = false;
    let nativeOverlaySupported = false;
    let hitRegionFrame = 0;
    const rate = 7.2;

    function setStatus(text, state) {
      $("status").textContent = text;
      $("statusWrap").className = "status " + (state || "live");
    }

    function collectActionRects() {
      const scale = window.devicePixelRatio || 1;
      const bleed = 18;
      const rect = $("actions").getBoundingClientRect();
      return [{
        left: Math.max(0, Math.floor((rect.left - bleed) * scale)),
        top: Math.max(0, Math.floor((rect.top - bleed) * scale)),
        right: Math.ceil((rect.right + bleed) * scale),
        bottom: Math.ceil((rect.bottom + bleed) * scale)
      }].filter((item) => item.right > item.left && item.bottom > item.top);
    }

    function syncHitMode() {
      if (!bridge?.SetTransparentHitMode) return;
      bridge.SetTransparentHitMode(pinned, pinned ? collectActionRects() : []);
    }

    function requestHitModeSync() {
      if (hitRegionFrame) window.cancelAnimationFrame(hitRegionFrame);
      hitRegionFrame = window.requestAnimationFrame(() => {
        hitRegionFrame = 0;
        syncHitMode();
      });
    }

    function setPinned(next, syncNative) {
      pinned = Boolean(next);
      document.documentElement.classList.toggle("is-transparent", pinned);
      document.body.classList.toggle("is-transparent", pinned);
      $("pin").classList.toggle("is-pinned", pinned);
      $("pin").setAttribute("aria-pressed", String(pinned));
      $("pin").title = pinned ? "取消置顶并恢复普通模式" : "置顶并进入透明模式";
      $("pin").setAttribute("aria-label", $("pin").title);
      if (syncNative && bridge) bridge.SetAlwaysOnTop(pinned);
      requestHitModeSync();
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
      const todayCost = money(today.total_cost_usd);
      const todayRequests = compact(today.total_requests);
      const totalCost = money(total.total_cost_usd);
      const totalRequests = compact(total.total_requests);
      $("todayCost").textContent = todayCost;
      $("todaySub").textContent = smallUSD(today.total_cost_usd) + " · " + todayRequests + " requests today";
      $("todayRequests").textContent = todayRequests;
      $("totalRequests").textContent = totalRequests;
      $("totalCost").textContent = totalCost;
      $("cacheHit").textContent = pct(total.cache_hit_rate);
      $("floatTodayCost").textContent = todayCost;
      $("floatTodayRequests").textContent = todayRequests;
      $("floatTotalCost").textContent = totalCost;
      $("floatTotalRequests").textContent = totalRequests;
      const updated = data.updated_at ? new Date(data.updated_at) : new Date();
      $("updated").textContent = updated.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    }

    $("refresh").addEventListener("click", () => loadStats());
    $("pin").addEventListener("click", async () => {
      if (!pinned && nativeOverlaySupported && bridge?.ShowNativeOverlay) {
        try {
          if (await bridge.ShowNativeOverlay()) return;
        } catch (_) {}
        setStatus("Overlay unavailable", "error");
        return;
      }
      setPinned(!pinned, true);
    });
    $("close").addEventListener("click", () => {
      if (bridge?.SetTransparentHitMode) bridge.SetTransparentHitMode(false, []);
      bridge?.Close();
    });

    const widget = document.querySelector(".widget");
    const topbar = document.querySelector(".topbar");
    topbar.addEventListener("pointerdown", (event) => {
      if (event.target.closest("button")) return;
      if (pinned) return;
      widget.classList.add("is-dragging");
    });
    window.addEventListener("pointerup", () => widget.classList.remove("is-dragging"));
    window.addEventListener("blur", () => widget.classList.remove("is-dragging"));
    window.addEventListener("resize", requestHitModeSync);

	waitForBridge()
		.then(async (resolved) => {
			bridge = resolved;
			try {
				nativeOverlaySupported = Boolean(await bridge.SupportsNativeOverlay?.());
			} catch (_) {
				nativeOverlaySupported = false;
			}
			setPinned(false, true);
			return loadStats();
		})
      .then(() => setInterval(loadStats, 12000))
      .catch(() => {
        setStatus("Bridge error", "error");
      });
  </script>
</body>
</html>`
