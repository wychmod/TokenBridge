//go:build !darwin

package autostart

func applyDarwin(enabled bool, executable string) error {
	return nil
}
