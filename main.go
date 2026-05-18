package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	buildembed "tokenbridge/build/embed"
	"tokenbridge/internal/desktop"
)

type DesktopApp struct {
	*desktop.DesktopApp
}

func NewDesktopApp() *DesktopApp {
	return &DesktopApp{DesktopApp: desktop.NewDesktopApp()}
}

func main() {
	if isWidget, adminURL := desktop.ParseAIStatsWidgetMode(); isWidget {
		if err := desktop.RunAIStatsWidget(adminURL); err != nil {
			panic(err)
		}
		return
	}

	desktopApp := NewDesktopApp()
	proxy := desktop.NewSPAProxy()
	desktop.RunDesktopTray(desktopApp.DesktopApp)

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
		Menu: desktop.BuildDesktopMenu(desktopApp.DesktopApp),
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
				proxy.SetRouter(desktopApp.Application.Router)
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
