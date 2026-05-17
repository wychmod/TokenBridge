//go:build !windows && !darwin

package main

func applyAIStatsTransparentHitMode(_ bool, _ []WidgetHitRect) error {
	return nil
}
