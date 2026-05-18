//go:build !windows

package desktop

import "tokenbridge/internal/aitoolusage"

type aiStatsNativeOverlay struct{}

func aiStatsNativeOverlaySupported() bool {
	return false
}

func newAIStatsNativeOverlay(_, _, _ func()) *aiStatsNativeOverlay {
	return &aiStatsNativeOverlay{}
}

func (o *aiStatsNativeOverlay) Show(_ aitoolusage.RealtimeSnapshot, _, _ int) bool {
	return false
}

func (o *aiStatsNativeOverlay) UpdateSnapshot(_ aitoolusage.RealtimeSnapshot) {}

func (o *aiStatsNativeOverlay) Close() {}
