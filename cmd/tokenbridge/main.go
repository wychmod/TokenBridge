package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"

	"tokenbridge/internal/app"
)

const trayTooltip = "TokenBridge 本地 AI 网关"

func main() {
	application, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create application: %v\n", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", application.Config.Server.Host, application.Config.Server.Port),
		Handler:      application.Router,
		ReadTimeout:  time.Duration(application.Config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(application.Config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(application.Config.Server.IdleTimeout) * time.Second,
	}

	adminURL := buildAdminURL(application.Config.Server.Host, application.Config.Server.Port, application.Config.Server.AdminPath)

	// Windows 正式模式：单实例检查，防止重复启动
	if shouldRunInTray() {
		mutexHandle, ok := acquireInstanceLock()
		if !ok {
			application.Logger.Info().Msg("检测到已有实例在运行，正在唤起管理后台")
			if err := waitForServer(server.Addr, 5*time.Second); err == nil {
				if err := openAdminPage(adminURL, application.Config.Server.PreferBrowser); err != nil {
					application.Logger.Warn().Err(err).Msg("唤起管理后台失败")
				}
				return
			}
			application.Logger.Warn().Msg("单实例锁残留，继续启动新实例")
		} else {
			defer releaseInstanceLock(mutexHandle)
		}
	}

	shutdown := make(chan struct{})
	var shutdownOnce sync.Once
	requestShutdown := func() {
		shutdownOnce.Do(func() {
			close(shutdown)
		})
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			application.Logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	application.Logger.Info().Str("addr", server.Addr).Msg("tokenbridge started")

	if application.Config.Server.AutoOpenAdmin {
		go func() {
			if err := waitForServer(server.Addr, 10*time.Second); err != nil {
				application.Logger.Warn().Err(err).Msg("admin page auto-open skipped because server was not reachable in time")
				return
			}

			if err := openAdminPage(adminURL, application.Config.Server.PreferBrowser); err != nil {
				application.Logger.Warn().Err(err).Str("url", adminURL).Msg("failed to auto-open admin page")
			}
		}()
	}

	if shouldRunInTray() {
		go watchSignals(requestShutdown)
		runTray(adminURL, requestShutdown)
	} else {
		watchSignalsBlocking(requestShutdown)
	}

	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		application.Logger.Error().Err(err).Msg("graceful shutdown failed")
	}
	if application.CloseLogs != nil {
		_ = application.CloseLogs()
	}
}

func shouldRunInTray() bool {
	return runtime.GOOS == "windows" && !isGoRunProcess() && !hasConsoleOverride()
}

func hasConsoleOverride() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("TB_CONSOLE_MODE")))
	return value == "1" || value == "true" || value == "console"
}

func isGoRunProcess() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	exeName := strings.ToLower(filepath.Base(exePath))
	if strings.HasPrefix(exeName, "tokenbridge") {
		return false
	}

	return strings.HasSuffix(exeName, ".exe")
}

func runTray(adminURL string, requestShutdown func()) {
	trayExited := make(chan struct{})

	go func() {
		systray.Run(func() {
			systray.SetTitle("TokenBridge")
			systray.SetTooltip(trayTooltip)
			if len(trayIcon) > 0 {
				systray.SetIcon(trayIcon)
			}

			openItem := systray.AddMenuItem("打开管理后台", "在浏览器中打开管理后台")
			statusItem := systray.AddMenuItem("服务状态：运行中", "当前服务状态")
			statusItem.Disable()
			systray.AddSeparator()
			exitItem := systray.AddMenuItem("退出程序", "关闭 TokenBridge")

			go func() {
				for {
					select {
					case <-openItem.ClickedCh:
						_ = openAdminPage(adminURL, "chrome")
					case <-exitItem.ClickedCh:
						requestShutdown()
						systray.Quit()
						return
					case <-trayExited:
						return
					}
				}
			}()
		}, func() {
			close(trayExited)
		})
	}()
}

func watchSignals(requestShutdown func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	requestShutdown()
	systray.Quit()
}

func watchSignalsBlocking(requestShutdown func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	requestShutdown()
}

func waitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("server %s did not become reachable within %s", addr, timeout)
}

func buildAdminURL(host string, port int, adminPath string) string {
	hostname := host
	if host == "0.0.0.0" || host == "" {
		hostname = "127.0.0.1"
	}

	path := adminPath
	if path == "" {
		path = "/admin"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return fmt.Sprintf("http://%s:%d%s", hostname, port, path)
}

func openAdminPage(rawURL string, preferBrowser string) error {
	if runtime.GOOS != "windows" {
		return openWithDefaultBrowser(rawURL)
	}

	browser := strings.ToLower(strings.TrimSpace(preferBrowser))
	if browser == "" || browser == "chrome" {
		if err := openWithChrome(rawURL); err == nil {
			return nil
		}
	}

	return openWithDefaultBrowser(rawURL)
}

func openWithChrome(rawURL string) error {
	chromePaths := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Google", "Chrome", "Application", "chrome.exe"),
	}

	for _, chromePath := range chromePaths {
		if chromePath == "" {
			continue
		}
		if _, err := os.Stat(chromePath); err == nil {
			return exec.Command(chromePath, rawURL).Start()
		}
	}

	if chromeCmd, err := exec.LookPath("chrome"); err == nil {
		return exec.Command(chromeCmd, rawURL).Start()
	}

	return fmt.Errorf("chrome executable not found")
}

func openWithDefaultBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	case "darwin":
		return exec.Command("open", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}
