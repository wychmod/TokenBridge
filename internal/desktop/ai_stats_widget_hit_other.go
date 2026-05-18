//go:build !windows && !darwin

package desktop

func applyAIStatsTransparentHitMode(_ bool, _ []WidgetHitRect) error {
	return nil
}
