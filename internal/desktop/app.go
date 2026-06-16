package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"tokenbridge/internal/app"
	"tokenbridge/internal/desktop/autostart"
	"tokenbridge/internal/paths"
)

const desktopVersion = "2.3.0"

type DesktopApp struct {
	ctx         context.Context
	Application *app.Application
	Server      *http.Server
	State       DesktopWindowState
	aiStatsMu   sync.Mutex
	aiStatsCmd  *exec.Cmd
	quitting    bool
}

type DesktopStatus struct {
	Version           string                `json:"version"`
	Platform          string                `json:"platform"`
	ServerAddr        string                `json:"serverAddr"`
	AdminURL          string                `json:"adminUrl"`
	WindowTitle       string                `json:"windowTitle"`
	DesktopMode       bool                  `json:"desktopMode"`
	Notifications     bool                  `json:"notifications"`
	CustomChrome      bool                  `json:"customChrome"`
	TrayEnabled       bool                  `json:"trayEnabled"`
	HideToTrayEnabled bool                  `json:"hideToTrayEnabled"`
	StateRestore      bool                  `json:"stateRestore"`
	WindowState       DesktopWindowState    `json:"windowState"`
	Runtime           DesktopRuntimeSummary `json:"runtime"`
	ConfigSummary     DesktopConfigSummary  `json:"configSummary"`
}

type DesktopRuntimeSummary struct {
	Providers int    `json:"providers"`
	Keys      int    `json:"keys"`
	Rules     int    `json:"rules"`
	Health    string `json:"health"`
}

type DesktopConfigSummary struct {
	Host          string `json:"host"`
	Port          int    `json:"port"`
	AdminPath     string `json:"adminPath"`
	Theme         string `json:"theme"`
	BundleMode    string `json:"bundleMode"`
	UpdateChannel string `json:"updateChannel"`
}

type DesktopSelfCheck struct {
	Health          string             `json:"health"`
	Checks          []DesktopCheckItem `json:"checks"`
	Warnings        []string           `json:"warnings"`
	CompletedAt     string             `json:"completedAt"`
	ServerReachable bool               `json:"serverReachable"`
}

type DesktopCheckItem struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
}

type DesktopWindowState struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	Maximised    bool   `json:"maximised"`
	LastRoute    string `json:"lastRoute"`
	HiddenToTray bool   `json:"hiddenToTray"`
}

type AIStatsWidgetState struct {
	Open bool `json:"open"`
	PID  int  `json:"pid,omitempty"`
}

func defaultDesktopWindowState() DesktopWindowState {
	return DesktopWindowState{Width: 1360, Height: 860, X: 120, Y: 80, LastRoute: "/dashboard"}
}

func NewDesktopApp() *DesktopApp { return &DesktopApp{State: loadDesktopWindowState()} }

func (d *DesktopApp) Startup(ctx context.Context) {
	d.ctx = ctx
	application, err := app.NewWithSettingsAutostart(autostart.NewManager())
	if err != nil {
		panic(fmt.Sprintf("failed to create application: %v", err))
	}
	d.Application = application
	d.Server = &http.Server{Addr: fmt.Sprintf("%s:%d", application.Config.Server.Host, application.Config.Server.Port), Handler: application.Router, ReadTimeout: time.Duration(application.Config.Server.ReadTimeout) * time.Second, WriteTimeout: time.Duration(application.Config.Server.WriteTimeout) * time.Second, IdleTimeout: time.Duration(application.Config.Server.IdleTimeout) * time.Second}
	go func() {
		application.Logger.Info().Str("addr", d.Server.Addr).Msg("http server started")
		if err := d.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			application.Logger.Fatal().Err(err).Msg("server failed")
		}
	}()
	go func() {
		if err := waitForDesktopServer(d.Server.Addr, 10*time.Second); err == nil {
			wailsruntime.EventsEmit(d.ctx, "desktop:server-ready", d.GetDesktopStatus())
		}
	}()
}

func (d *DesktopApp) Shutdown(ctx context.Context) {
	d.HideAIStatsWidget()
	d.persistWindowState()
	if d.Server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.Server.Shutdown(shutdownCtx); err != nil {
			d.Application.Logger.Error().Err(err).Msg("graceful shutdown failed")
		}
	}
	if d.Application != nil && d.Application.CloseLogs != nil {
		_ = d.Application.CloseLogs()
	}
}

func (d *DesktopApp) GetVersion() string { return desktopVersion }

func (d *DesktopApp) GetDesktopStatus() DesktopStatus {
	status := DesktopStatus{Version: desktopVersion, Platform: runtime.GOOS, WindowTitle: "TokenBridge", DesktopMode: true, Notifications: true, CustomChrome: true, TrayEnabled: true, HideToTrayEnabled: true, StateRestore: true, WindowState: d.State}
	if d.Application != nil && d.Server != nil {
		status.ServerAddr = d.Server.Addr
		status.AdminURL = buildDesktopAdminURL(d.Application.Config.Server.Host, d.Application.Config.Server.Port, d.Application.Config.Server.AdminPath)
		overview, err := d.Application.Admin.Overview(context.Background())
		if err == nil {
			status.Runtime = DesktopRuntimeSummary{Providers: overview.Providers, Keys: overview.Keys, Rules: overview.Rules, Health: deriveDesktopHealth(overview.Providers, overview.Keys)}
			status.ConfigSummary = DesktopConfigSummary{Host: overview.Settings.Host, Port: overview.Settings.Port, AdminPath: overview.Settings.AdminPath, Theme: overview.Settings.Theme, BundleMode: overview.Settings.BundleMode, UpdateChannel: overview.Settings.UpdateChannel}
		}
	}
	return status
}

func (d *DesktopApp) GetRuntimeSummary() DesktopRuntimeSummary { return d.GetDesktopStatus().Runtime }
func (d *DesktopApp) GetConfigSummary() DesktopConfigSummary {
	return d.GetDesktopStatus().ConfigSummary
}

func (d *DesktopApp) RunSelfCheck() DesktopSelfCheck {
	status := d.GetDesktopStatus()
	checks := []DesktopCheckItem{}
	warnings := []string{}

	serverReachable := waitForDesktopServer(status.ServerAddr, 2*time.Second) == nil
	checks = append(checks, makeCheck("port", "监听端口占用检查", "确认本地服务端口可被当前进程监听并可访问", boolStatus(serverReachable), map[bool]string{true: fmt.Sprintf("服务地址 %s 可访问", status.ServerAddr), false: fmt.Sprintf("服务地址 %s 不可访问", status.ServerAddr)}[serverReachable]))
	if !serverReachable {
		warnings = append(warnings, "本地 HTTP 服务暂时不可达")
	}

	providerOK, providerDetail := d.checkProviderConfig()
	checks = append(checks, makeCheck("providers", "Provider 配置检查", "确认至少存在一个有效 Provider 配置", boolStatus(providerOK), providerDetail))
	if !providerOK {
		warnings = append(warnings, providerDetail)
	}

	dbOK, dbDetail := d.checkDatabaseWritable()
	checks = append(checks, makeCheck("database", "本地数据库可写检查", "确认数据库文件所在目录可创建和写入", boolStatus(dbOK), dbDetail))
	if !dbOK {
		warnings = append(warnings, dbDetail)
	}

	adminOK, adminDetail := checkAdminAssets()
	checks = append(checks, makeCheck("assets", "管理后台资源检查", "确认桌面版和浏览器版所需前端资源存在", boolStatus(adminOK), adminDetail))
	if !adminOK {
		warnings = append(warnings, adminDetail)
	}

	pkgOK, pkgDetail := checkPackagingResources()
	checks = append(checks, makeCheck("packaging", "打包资源完整性检查", "确认图标、脚本、配置模板和核心打包文件齐全", boolStatus(pkgOK), pkgDetail))
	if !pkgOK {
		warnings = append(warnings, pkgDetail)
	}

	health := "healthy"
	if len(warnings) > 0 {
		health = "warning"
	}
	result := DesktopSelfCheck{Health: health, Checks: checks, Warnings: warnings, CompletedAt: time.Now().Format(time.RFC3339), ServerReachable: serverReachable}
	if d.ctx != nil {
		wailsruntime.EventsEmit(d.ctx, "desktop:self-check", result)
	}
	return result
}

func (d *DesktopApp) GetWindowState() DesktopWindowState { return d.State }
func (d *DesktopApp) SaveWindowState(next DesktopWindowState) DesktopWindowState {
	if next.Width > 0 {
		d.State.Width = next.Width
	}
	if next.Height > 0 {
		d.State.Height = next.Height
	}
	d.State.X = next.X
	d.State.Y = next.Y
	d.State.Maximised = next.Maximised
	if next.LastRoute != "" {
		d.State.LastRoute = next.LastRoute
	}
	d.State.HiddenToTray = next.HiddenToTray
	d.persistWindowState()
	return d.State
}
func (d *DesktopApp) RestoreLastRoute() string {
	if d.State.LastRoute == "" {
		return "/dashboard"
	}
	return d.State.LastRoute
}
func (d *DesktopApp) HideToTray() {
	if d.ctx == nil {
		return
	}
	d.State.HiddenToTray = true
	d.persistWindowState()
	wailsruntime.WindowHide(d.ctx)
	wailsruntime.EventsEmit(d.ctx, "desktop:window-hidden", d.State)
}
func (d *DesktopApp) ShowMainWindow() {
	if d.ctx == nil {
		return
	}
	d.State.HiddenToTray = false
	d.persistWindowState()
	wailsruntime.WindowShow(d.ctx)
	wailsruntime.WindowUnminimise(d.ctx)
	wailsruntime.EventsEmit(d.ctx, "desktop:window-shown", d.State)
}
func (d *DesktopApp) MinimiseWindow() {
	if d.ctx == nil {
		return
	}
	wailsruntime.WindowMinimise(d.ctx)
}
func (d *DesktopApp) ToggleMaximiseWindow() {
	if d.ctx == nil {
		return
	}
	d.State.Maximised = !d.State.Maximised
	d.persistWindowState()
	wailsruntime.WindowToggleMaximise(d.ctx)
}
func (d *DesktopApp) IsQuitting() bool { return d.quitting }
func (d *DesktopApp) CloseWindow() {
	d.quitting = true
	if d.ctx == nil {
		os.Exit(0)
	}
	wailsruntime.Quit(d.ctx)
}
func (d *DesktopApp) OpenAdminInBrowser() {
	if d.ctx == nil {
		return
	}
	status := d.GetDesktopStatus()
	if status.AdminURL != "" {
		wailsruntime.BrowserOpenURL(d.ctx, status.AdminURL)
	}
}
func (d *DesktopApp) GetAIStatsWidgetState() AIStatsWidgetState {
	d.aiStatsMu.Lock()
	defer d.aiStatsMu.Unlock()
	return d.aiStatsWidgetStateLocked()
}
func (d *DesktopApp) ToggleAIStatsWidget() AIStatsWidgetState {
	if d.GetAIStatsWidgetState().Open {
		return d.HideAIStatsWidget()
	}
	return d.ShowAIStatsWidget()
}
func (d *DesktopApp) ShowAIStatsWidget() AIStatsWidgetState {
	d.aiStatsMu.Lock()
	defer d.aiStatsMu.Unlock()
	if state := d.aiStatsWidgetStateLocked(); state.Open {
		return state
	}
	status := d.GetDesktopStatus()
	if status.AdminURL == "" {
		d.emitAIStatsWidgetStateLocked()
		return AIStatsWidgetState{}
	}
	executable, err := os.Executable()
	if err != nil {
		d.emitAIStatsWidgetStateLocked()
		return AIStatsWidgetState{}
	}
	cmd := exec.Command(executable, "--ai-stats-widget", "--admin-url", status.AdminURL)
	if err := cmd.Start(); err != nil {
		d.emitAIStatsWidgetStateLocked()
		return AIStatsWidgetState{}
	}
	d.aiStatsCmd = cmd
	go d.waitForAIStatsWidget(cmd)
	d.emitAIStatsWidgetStateLocked()
	return d.aiStatsWidgetStateLocked()
}
func (d *DesktopApp) HideAIStatsWidget() AIStatsWidgetState {
	d.aiStatsMu.Lock()
	defer d.aiStatsMu.Unlock()
	if d.aiStatsCmd != nil && d.aiStatsCmd.Process != nil && d.aiStatsCmd.ProcessState == nil {
		_ = d.aiStatsCmd.Process.Kill()
	}
	d.aiStatsCmd = nil
	d.emitAIStatsWidgetStateLocked()
	return AIStatsWidgetState{}
}
func (d *DesktopApp) SendNativeNotice(title string, message string) {
	if d.ctx == nil {
		return
	}
	if title == "" {
		title = "TokenBridge"
	}
	if message == "" {
		message = "桌面通知"
	}
	wailsruntime.EventsEmit(d.ctx, "desktop:notice", map[string]string{"title": title, "message": message})
	_, _ = wailsruntime.MessageDialog(d.ctx, wailsruntime.MessageDialogOptions{Type: wailsruntime.InfoDialog, Title: title, Message: message})
}

func (d *DesktopApp) waitForAIStatsWidget(cmd *exec.Cmd) {
	_ = cmd.Wait()
	d.aiStatsMu.Lock()
	defer d.aiStatsMu.Unlock()
	if d.aiStatsCmd == cmd {
		d.aiStatsCmd = nil
		d.emitAIStatsWidgetStateLocked()
	}
}

func (d *DesktopApp) aiStatsWidgetStateLocked() AIStatsWidgetState {
	if d.aiStatsCmd == nil || d.aiStatsCmd.Process == nil {
		return AIStatsWidgetState{}
	}
	if d.aiStatsCmd.ProcessState != nil && d.aiStatsCmd.ProcessState.Exited() {
		d.aiStatsCmd = nil
		return AIStatsWidgetState{}
	}
	return AIStatsWidgetState{Open: true, PID: d.aiStatsCmd.Process.Pid}
}

func (d *DesktopApp) emitAIStatsWidgetStateLocked() {
	if d.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(d.ctx, "desktop:ai-stats-widget", d.aiStatsWidgetStateLocked())
}

func (d *DesktopApp) checkProviderConfig() (bool, string) {
	if d.Application == nil || d.Application.Providers == nil {
		return false, "Provider 服务未初始化"
	}
	providers, err := d.Application.Providers.List(context.Background())
	if err != nil {
		return false, fmt.Sprintf("读取 Provider 失败：%v", err)
	}
	if len(providers) == 0 {
		return false, "未配置任何 Provider"
	}
	enabled := 0
	for _, item := range providers {
		if item.Enabled && item.BaseURL != "" {
			enabled++
		}
	}
	if enabled == 0 {
		return false, "存在 Provider，但没有可用的启用项或缺少 BaseURL"
	}
	return true, fmt.Sprintf("共 %d 个 Provider，其中 %d 个可用", len(providers), enabled)
}

func (d *DesktopApp) checkDatabaseWritable() (bool, string) {
	if d.Application == nil {
		return false, "应用未初始化"
	}
	dbPath := d.Application.Config.Database.Path
	if dbPath == "" {
		return false, "数据库路径为空"
	}
	abs, _ := filepath.Abs(dbPath)
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Sprintf("数据库目录不可创建：%v", err)
	}
	testFile := filepath.Join(dir, ".write-test.tmp")
	if err := os.WriteFile(testFile, []byte("ok"), 0o644); err != nil {
		return false, fmt.Sprintf("数据库目录不可写：%v", err)
	}
	_ = os.Remove(testFile)
	return true, fmt.Sprintf("数据库目录可写：%s", dir)
}

func checkAdminAssets() (bool, string) {
	checks := []string{"web/admin/dist/index.html", "build/embed/admin/index.html", "web/admin/public/favicon.png"}
	missing := []string{}
	for _, item := range checks {
		if _, err := os.Stat(item); err != nil {
			missing = append(missing, item)
		}
	}
	if len(missing) > 0 {
		return false, "缺少资源：" + strings.Join(missing, "、")
	}
	return true, "管理后台构建资源完整"
}

func checkPackagingResources() (bool, string) {
	checks := []string{"build/package.ps1", "build/desktop.ps1", "build/assets/app-icon.ico", "cmd/tokenbridge/tray-icon.ico", "configs/config.example.yaml"}
	missing := []string{}
	for _, item := range checks {
		if _, err := os.Stat(item); err != nil {
			missing = append(missing, item)
		}
	}
	if len(missing) > 0 {
		return false, "缺少打包资源：" + strings.Join(missing, "、")
	}
	return true, "桌面版与便携版打包关键资源齐全"
}

func makeCheck(key, title, description, status, detail string) DesktopCheckItem {
	return DesktopCheckItem{Key: key, Title: title, Description: description, Status: status, Detail: detail}
}
func boolStatus(ok bool) string {
	if ok {
		return "ready"
	}
	return "blocked"
}

func (d *DesktopApp) persistWindowState() {
	statePath := desktopStatePath()
	_ = os.MkdirAll(filepath.Dir(statePath), 0o755)
	data, err := json.MarshalIndent(d.State, "", "  ")
	if err == nil {
		_ = os.WriteFile(statePath, data, 0o644)
	}
}
func desktopStatePath() string {
	appPaths, err := paths.Resolve()
	if err != nil {
		return "desktop-state.json"
	}
	return filepath.Join(appPaths.ConfigDir, "desktop-state.json")
}
func loadDesktopWindowState() DesktopWindowState {
	state := defaultDesktopWindowState()
	statePath := desktopStatePath()
	data, err := os.ReadFile(statePath)
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	if state.LastRoute == "" {
		state.LastRoute = "/dashboard"
	}
	if state.Width == 0 {
		state.Width = 1360
	}
	if state.Height == 0 {
		state.Height = 860
	}
	return state
}
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
func deriveDesktopHealth(providers int, keys int) string {
	if providers == 0 || keys == 0 {
		return "warning"
	}
	return "healthy"
}
func waitForDesktopServer(addr string, timeout time.Duration) error {
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
func buildDesktopAdminURL(host string, port int, adminPath string) string {
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
