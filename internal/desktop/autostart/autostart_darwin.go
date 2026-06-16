//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func applyDarwin(enabled bool, executable string) error {
	plistPath, err := launchAgentPath()
	if err != nil {
		return err
	}

	if !enabled {
		_ = exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), plistPath).Run()
		if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove macOS LaunchAgent: %w", err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("create macOS LaunchAgents directory: %w", err)
	}
	if err := os.WriteFile(plistPath, []byte(launchAgentPlist(executable)), 0o644); err != nil {
		return fmt.Errorf("write macOS LaunchAgent: %w", err)
	}
	_ = exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), plistPath).Run()
	if err := exec.Command("launchctl", "bootstrap", "gui/"+fmt.Sprint(os.Getuid()), plistPath).Run(); err != nil {
		return fmt.Errorf("load macOS LaunchAgent: %w", err)
	}
	return nil
}

func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for LaunchAgent: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentFilename), nil
}
