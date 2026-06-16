//go:build windows

package autostart

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const windowsRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func applyWindows(enabled bool, executable string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Windows startup registry key: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(appName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("remove Windows startup registry value: %w", err)
		}
		return nil
	}
	if err := key.SetStringValue(appName, windowsCommand(executable)); err != nil {
		return fmt.Errorf("write Windows startup registry value: %w", err)
	}
	return nil
}

func windowsCommand(executable string) string {
	return `"` + executable + `"`
}
