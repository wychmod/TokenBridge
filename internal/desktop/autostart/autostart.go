package autostart

import (
	"fmt"
	"os"
	"runtime"
)

const (
	appName             = "TokenBridge"
	launchAgentLabel    = "dev.tokenbridge.app"
	launchAgentFilename = launchAgentLabel + ".plist"
)

type Manager struct {
	executable string
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Apply(enabled bool) error {
	executable := m.executable
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable for autostart: %w", err)
		}
	}
	return apply(enabled, executable)
}

func apply(enabled bool, executable string) error {
	switch runtime.GOOS {
	case "windows":
		return applyWindows(enabled, executable)
	case "darwin":
		return applyDarwin(enabled, executable)
	default:
		if enabled {
			return fmt.Errorf("autostart is not supported on %s", runtime.GOOS)
		}
		return nil
	}
}
