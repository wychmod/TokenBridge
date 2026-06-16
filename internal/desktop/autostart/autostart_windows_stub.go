//go:build !windows

package autostart

func applyWindows(enabled bool, executable string) error {
	return nil
}

func windowsCommand(executable string) string {
	return `"` + executable + `"`
}
