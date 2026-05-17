//go:build windows

package main

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsExTransparent = 0x00000020
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	hitPollInterval = 35 * time.Millisecond
)

var (
	gwlExStyleIndex              = ^uintptr(0) - 19
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetCursorPos             = user32.NewProc("GetCursorPos")
	procGetWindowLongPtr         = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos             = user32.NewProc("SetWindowPos")

	transparentHitMu    sync.Mutex
	transparentHitHWND  windows.Handle
	transparentHitRects []WidgetHitRect
	transparentHitOn    bool
	transparentHitRun   bool
	transparentPassThru bool
)

type winRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

type winPoint struct {
	x int32
	y int32
}

func applyAIStatsTransparentHitMode(enabled bool, rects []WidgetHitRect) error {
	hwnd, err := currentProcessWindowHandle()
	if err != nil {
		return err
	}

	transparentHitMu.Lock()
	transparentHitHWND = hwnd
	transparentHitOn = enabled
	transparentHitRects = normalizeWidgetHitRects(rects)
	if enabled && !transparentHitRun {
		transparentHitRun = true
		go transparentHitLoop()
	}
	transparentHitMu.Unlock()

	if !enabled {
		return setAIStatsWindowPassthrough(hwnd, false)
	}
	return updateTransparentHitState(hwnd, normalizeWidgetHitRects(rects))
}

func transparentHitLoop() {
	ticker := time.NewTicker(hitPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		transparentHitMu.Lock()
		if !transparentHitOn {
			hwnd := transparentHitHWND
			transparentHitRun = false
			transparentHitMu.Unlock()
			if hwnd != 0 {
				_ = setAIStatsWindowPassthrough(hwnd, false)
			}
			return
		}
		hwnd := transparentHitHWND
		rects := append([]WidgetHitRect(nil), transparentHitRects...)
		transparentHitMu.Unlock()
		_ = updateTransparentHitState(hwnd, rects)
	}
}

func updateTransparentHitState(hwnd windows.Handle, rects []WidgetHitRect) error {
	if hwnd == 0 {
		return nil
	}
	var windowRect winRect
	ret, _, callErr := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&windowRect)))
	if ret == 0 {
		return fmt.Errorf("get AI stats widget window rect failed: %w", callErr)
	}
	var point winPoint
	ret, _, callErr = procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if ret == 0 {
		return fmt.Errorf("get cursor position failed: %w", callErr)
	}
	localX := int(point.x - windowRect.left)
	localY := int(point.y - windowRect.top)
	inAction := false
	for _, rect := range rects {
		if localX >= rect.Left && localX <= rect.Right && localY >= rect.Top && localY <= rect.Bottom {
			inAction = true
			break
		}
	}
	return setAIStatsWindowPassthrough(hwnd, !inAction)
}

func setAIStatsWindowPassthrough(hwnd windows.Handle, enabled bool) error {
	transparentHitMu.Lock()
	if transparentPassThru == enabled {
		transparentHitMu.Unlock()
		return nil
	}
	transparentHitMu.Unlock()

	style, err := getAIStatsWindowExStyle(hwnd)
	if err != nil {
		return err
	}
	next := style
	if enabled {
		next = style | uintptr(wsExTransparent)
	} else {
		next = style &^ uintptr(wsExTransparent)
	}
	if next != style {
		if err := setAIStatsWindowExStyle(hwnd, next); err != nil {
			return err
		}
	}
	flushAIStatsWindowStyle(hwnd)
	transparentHitMu.Lock()
	transparentPassThru = enabled
	transparentHitMu.Unlock()
	return nil
}

func getAIStatsWindowExStyle(hwnd windows.Handle) (uintptr, error) {
	style, _, callErr := procGetWindowLongPtr.Call(uintptr(hwnd), gwlExStyleIndex)
	if style == 0 && callErr != windows.ERROR_SUCCESS {
		return 0, fmt.Errorf("get AI stats widget exstyle failed: %w", callErr)
	}
	return style, nil
}

func setAIStatsWindowExStyle(hwnd windows.Handle, style uintptr) error {
	ret, _, callErr := procSetWindowLongPtr.Call(uintptr(hwnd), gwlExStyleIndex, style)
	if ret == 0 && callErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("set AI stats widget exstyle failed: %w", callErr)
	}
	return nil
}

func flushAIStatsWindowStyle(hwnd windows.Handle) {
	procSetWindowPos.Call(
		uintptr(hwnd),
		0,
		0,
		0,
		0,
		0,
		uintptr(swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged),
	)
}

func currentProcessWindowHandle() (windows.Handle, error) {
	currentPID := uint32(os.Getpid())
	var result windows.Handle
	callback := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != currentPID {
			return 1
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		if visible == 0 {
			return 1
		}
		result = windows.Handle(hwnd)
		return 0
	})
	procEnumWindows.Call(callback, 0)
	if result == 0 {
		return 0, fmt.Errorf("AI stats widget window handle not found")
	}
	return result, nil
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
