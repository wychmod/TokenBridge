package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	buildembed "tokenbridge/build/embed"
)

type spaProxy struct {
	router http.Handler
	ready  bool
}

func newSPAProxy() *spaProxy {
	return &spaProxy{}
}

func (p *spaProxy) setRouter(router http.Handler) {
	p.router = router
	p.ready = true
}

// ServeHTTP delegates all requests to the shared router.
// The router handles API endpoints, static assets, and SPA fallback uniformly.
// This ensures desktop and browser modes behave identically.
func (p *spaProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.ready || p.router == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"server not ready"}`))
		return
	}
	p.router.ServeHTTP(w, r)
}

func buildDesktopMenu(app *DesktopApp) *menu.Menu {
	appMenu := menu.NewMenu()

	fileMenu := appMenu.AddSubmenu("应用")
	fileMenu.AddText("显示主窗口", nil, func(_ *menu.CallbackData) { app.ShowMainWindow() })
	fileMenu.AddText("隐藏到托盘", nil, func(_ *menu.CallbackData) { app.HideToTray() })
	fileMenu.AddText("打开管理后台", nil, func(_ *menu.CallbackData) { app.OpenAdminInBrowser() })
	fileMenu.AddText("发送测试通知", nil, func(_ *menu.CallbackData) { app.SendNativeNotice("TokenBridge", "桌面通知链路正常") })
	fileMenu.AddSeparator()
	fileMenu.AddText("退出", nil, func(_ *menu.CallbackData) { app.CloseWindow() })

	windowMenu := appMenu.AddSubmenu("窗口")
	windowMenu.AddText("最小化", nil, func(_ *menu.CallbackData) { app.MinimiseWindow() })
	windowMenu.AddText("最大化 / 还原", nil, func(_ *menu.CallbackData) { app.ToggleMaximiseWindow() })
	windowMenu.AddText("恢复上次页面", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "desktop:restore-route", app.RestoreLastRoute())
	})

	helpMenu := appMenu.AddSubmenu("帮助")
	helpMenu.AddText("运行桌面自检", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "desktop:self-check", app.RunSelfCheck())
	})
	helpMenu.AddText("查看版本信息", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "desktop:menu-version", app.GetDesktopStatus())
	})

	return appMenu
}

func runDesktopTray(app *DesktopApp) {
	systray.SetDoubleClickHandler(func() {
		app.ShowMainWindow()
	})
	systray.Register(func() {
		systray.SetTitle("TokenBridge")
		systray.SetTooltip("TokenBridge 桌面版")
		if len(trayIcon) > 0 {
			systray.SetIcon(trayIcon)
		}

		showItem := systray.AddMenuItem("显示主窗口", "恢复并显示窗口")
		hideItem := systray.AddMenuItem("隐藏到托盘", "隐藏主窗口")
		openItem := systray.AddMenuItem("打开管理后台", "在浏览器中打开后台")
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("退出程序", "关闭 TokenBridge")

		go func() {
			for {
				select {
				case <-showItem.ClickedCh:
					app.ShowMainWindow()
				case <-hideItem.ClickedCh:
					app.HideToTray()
				case <-openItem.ClickedCh:
					app.OpenAdminInBrowser()
				case <-quitItem.ClickedCh:
					app.CloseWindow()
					return
				}
			}
		}()
	}, func() {})
}

func main() {
	if isWidget, adminURL := parseAIStatsWidgetMode(); isWidget {
		if err := runAIStatsWidget(adminURL); err != nil {
			panic(err)
		}
		return
	}

	desktopApp := NewDesktopApp()
	proxy := newSPAProxy()
	runDesktopTray(desktopApp)

	err := wails.Run(&options.App{
		Title:            "TokenBridge",
		Width:            desktopApp.State.Width,
		Height:           desktopApp.State.Height,
		MinWidth:         960,
		MinHeight:        640,
		Frameless:        true,
		DisableResize:    false,
		Fullscreen:       false,
		StartHidden:      false,
		BackgroundColour: &options.RGBA{R: 23, G: 26, B: 33, A: 1},
		AssetServer: &assetserver.Options{
			Assets:  buildembed.AdminAssetsFS(),
			Handler: proxy,
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/admin/api") {
						proxy.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Menu: buildDesktopMenu(desktopApp),
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 false,
			BackdropType:                      windows.Mica,
			DisablePinchZoom:                  true,
			EnableSwipeGestures:               false,
			Theme:                             windows.SystemDefault,
			ResizeDebounceMS:                  10,
			DisableFramelessWindowDecorations: false,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "TokenBridge",
				Message: "本地模型网关桌面版",
			},
		},
		OnStartup: func(ctx context.Context) {
			desktopApp.Startup(ctx)
			if desktopApp.Application != nil && desktopApp.Application.Router != nil {
				proxy.setRouter(desktopApp.Application.Router)
			}
		},
		OnDomReady: func(ctx context.Context) {
			wailsruntime.EventsEmit(ctx, "desktop:dom-ready", desktopApp.GetDesktopStatus())
			wailsruntime.EventsEmit(ctx, "desktop:restore-route", desktopApp.RestoreLastRoute())
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if desktopApp.IsQuitting() {
				return false
			}
			desktopApp.HideToTray()
			return true
		},
		OnShutdown: func(ctx context.Context) {
			systray.Quit()
			desktopApp.Shutdown(ctx)
		},
		Bind: []interface{}{desktopApp},
	})

	if err != nil {
		panic(err)
	}
}
