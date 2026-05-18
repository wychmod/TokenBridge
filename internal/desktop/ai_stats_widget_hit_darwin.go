//go:build darwin

package desktop

/*
#cgo CFLAGS: -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#include <stdbool.h>

static NSWindow* aiStatsVisibleWindow(void) {
	for (NSWindow *window in [NSApp windows]) {
		if ([window isVisible]) {
			return window;
		}
	}
	return nil;
}

static void aiStatsReadMouse(double *outX, double *outY, bool *outOK) {
	NSWindow *window = aiStatsVisibleWindow();
	if (window == nil) {
		*outOK = false;
		return;
	}

	NSPoint mouse = [NSEvent mouseLocation];
	NSRect frame = [window frame];
	CGFloat scale = [window backingScaleFactor];
	if (scale <= 0) {
		scale = [[NSScreen mainScreen] backingScaleFactor];
	}

	*outX = (mouse.x - frame.origin.x) * scale;
	*outY = (NSMaxY(frame) - mouse.y) * scale;
	*outOK = true;
}

static bool aiStatsMouseLocation(double *outX, double *outY) {
	__block double x = 0;
	__block double y = 0;
	__block bool ok = false;

	if ([NSThread isMainThread]) {
		aiStatsReadMouse(&x, &y, &ok);
	} else {
		dispatch_sync(dispatch_get_main_queue(), ^{
			aiStatsReadMouse(&x, &y, &ok);
		});
	}

	if (ok) {
		*outX = x;
		*outY = y;
	}
	return ok;
}

static void aiStatsSetIgnoresMouseEvents(bool enabled) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *window = aiStatsVisibleWindow();
		if (window != nil) {
			[window setIgnoresMouseEvents:enabled ? YES : NO];
		}
	});
}
*/
import "C"

import (
	"sync"
	"time"
)

const macHitPollInterval = 35 * time.Millisecond

var (
	macTransparentHitMu    sync.Mutex
	macTransparentHitRects []WidgetHitRect
	macTransparentHitOn    bool
	macTransparentHitRun   bool
	macTransparentPassThru bool
)

func applyAIStatsTransparentHitMode(enabled bool, rects []WidgetHitRect) error {
	macTransparentHitMu.Lock()
	macTransparentHitOn = enabled
	macTransparentHitRects = normalizeWidgetHitRects(rects)
	if enabled && !macTransparentHitRun {
		macTransparentHitRun = true
		go macTransparentHitLoop()
	}
	macTransparentHitMu.Unlock()

	if !enabled {
		setMacAIStatsWindowPassthrough(false)
		return nil
	}
	return updateMacTransparentHitState(normalizeWidgetHitRects(rects))
}

func macTransparentHitLoop() {
	ticker := time.NewTicker(macHitPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		macTransparentHitMu.Lock()
		if !macTransparentHitOn {
			macTransparentHitRun = false
			macTransparentHitMu.Unlock()
			setMacAIStatsWindowPassthrough(false)
			return
		}
		rects := append([]WidgetHitRect(nil), macTransparentHitRects...)
		macTransparentHitMu.Unlock()
		_ = updateMacTransparentHitState(rects)
	}
}

func updateMacTransparentHitState(rects []WidgetHitRect) error {
	var localX C.double
	var localY C.double
	if !bool(C.aiStatsMouseLocation(&localX, &localY)) {
		return nil
	}

	x := int(localX)
	y := int(localY)
	inAction := false
	for _, rect := range rects {
		if x >= rect.Left && x <= rect.Right && y >= rect.Top && y <= rect.Bottom {
			inAction = true
			break
		}
	}
	setMacAIStatsWindowPassthrough(!inAction)
	return nil
}

func setMacAIStatsWindowPassthrough(enabled bool) {
	macTransparentHitMu.Lock()
	if macTransparentPassThru == enabled {
		macTransparentHitMu.Unlock()
		return
	}
	macTransparentPassThru = enabled
	macTransparentHitMu.Unlock()
	C.aiStatsSetIgnoresMouseEvents(C.bool(enabled))
}

func normalizeWidgetHitRects(rects []WidgetHitRect) []WidgetHitRect {
	result := make([]WidgetHitRect, 0, len(rects))
	for _, rect := range rects {
		if rect.Right <= rect.Left || rect.Bottom <= rect.Top {
			continue
		}
		result = append(result, WidgetHitRect{
			Left:   maxInt(0, rect.Left),
			Top:    maxInt(0, rect.Top),
			Right:  maxInt(1, rect.Right),
			Bottom: maxInt(1, rect.Bottom),
		})
	}
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
