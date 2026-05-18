package desktop

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func BuildDesktopMenu(app *DesktopApp) *menu.Menu {
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
