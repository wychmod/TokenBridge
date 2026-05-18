package desktop

import "github.com/getlantern/systray"

func RunDesktopTray(app *DesktopApp) {
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
