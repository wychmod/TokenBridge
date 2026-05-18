//go:build windows

package desktop

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"tokenbridge/internal/aitoolusage"
)

const (
	overlayWidth  = 340
	overlayHeight = 92

	overlayKeyR = 1
	overlayKeyG = 2
	overlayKeyB = 3

	wsPopup       = 0x80000000
	wsVisible     = 0x10000000
	wsExLayered   = 0x00080000
	wsExTopmost   = 0x00000008
	wsExToolWin   = 0x00000080
	wsExNoActive  = 0x08000000
	lwaColorKey   = 0x00000001
	swShowNoAct   = 4
	wmPaint       = 0x000F
	wmEraseBkgnd  = 0x0014
	wmClose       = 0x0010
	wmDestroy     = 0x0002
	wmLButtonUp   = 0x0202
	wmNCHitTest   = 0x0084
	htTransparent = ^uintptr(0)
	htClient      = 1
	bkTransparent = 1
)

var (
	overlayUser32               = windows.NewLazySystemDLL("user32.dll")
	overlayGDI32                = windows.NewLazySystemDLL("gdi32.dll")
	overlayKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procOverlayBeginPaint       = overlayUser32.NewProc("BeginPaint")
	procOverlayCreateWindowEx   = overlayUser32.NewProc("CreateWindowExW")
	procOverlayDefWindowProc    = overlayUser32.NewProc("DefWindowProcW")
	procOverlayDestroyWindow    = overlayUser32.NewProc("DestroyWindow")
	procOverlayDispatchMessage  = overlayUser32.NewProc("DispatchMessageW")
	procOverlayEndPaint         = overlayUser32.NewProc("EndPaint")
	procOverlayFillRect         = overlayUser32.NewProc("FillRect")
	procOverlayGetMessage       = overlayUser32.NewProc("GetMessageW")
	procOverlayGetWindowRect    = overlayUser32.NewProc("GetWindowRect")
	procOverlayInvalidateRect   = overlayUser32.NewProc("InvalidateRect")
	procOverlayLoadCursor       = overlayUser32.NewProc("LoadCursorW")
	procOverlayPostMessage      = overlayUser32.NewProc("PostMessageW")
	procOverlayPostQuitMessage  = overlayUser32.NewProc("PostQuitMessage")
	procOverlayRegisterClassEx  = overlayUser32.NewProc("RegisterClassExW")
	procOverlaySetLayeredAttrs  = overlayUser32.NewProc("SetLayeredWindowAttributes")
	procOverlaySetWindowPos     = overlayUser32.NewProc("SetWindowPos")
	procOverlayShowWindow       = overlayUser32.NewProc("ShowWindow")
	procOverlayTranslateMessage = overlayUser32.NewProc("TranslateMessage")
	procOverlayUpdateWindow     = overlayUser32.NewProc("UpdateWindow")
	procOverlayCreateFont       = overlayGDI32.NewProc("CreateFontW")
	procOverlayCreateSolidBrush = overlayGDI32.NewProc("CreateSolidBrush")
	procOverlayDeleteObject     = overlayGDI32.NewProc("DeleteObject")
	procOverlaySelectObject     = overlayGDI32.NewProc("SelectObject")
	procOverlaySetBkMode        = overlayGDI32.NewProc("SetBkMode")
	procOverlaySetTextColor     = overlayGDI32.NewProc("SetTextColor")
	procOverlayTextOut          = overlayGDI32.NewProc("TextOutW")
	procOverlayGetModuleHandle  = overlayKernel32.NewProc("GetModuleHandleW")
	overlayWndProc              = windows.NewCallback(aiStatsOverlayWndProc)
	overlayClassName, _         = windows.UTF16PtrFromString("TokenBridgeAIStatsNativeOverlay")
	overlayTitle, _             = windows.UTF16PtrFromString("TokenBridge AI Stats Overlay")
	overlayClassOnce            sync.Once
	overlayRegistryMu           sync.Mutex
	overlayByHWND               = map[windows.Handle]*aiStatsNativeOverlay{}
	overlayActive               *aiStatsNativeOverlay
	overlayTextHalo             = []overlayTextOffset{
		{x: 0, y: 2},
		{x: 2, y: 0},
		{x: -2, y: 0},
		{x: 0, y: -2},
		{x: 1, y: 1},
		{x: -1, y: 1},
		{x: 1, y: -1},
		{x: -1, y: -1},
	}
)

type aiStatsNativeOverlay struct {
	mu        sync.Mutex
	hwnd      windows.Handle
	running   bool
	ready     chan bool
	done      chan struct{}
	x         int
	y         int
	snapshot  aitoolusage.RealtimeSnapshot
	onUnpin   func()
	onRefresh func()
	onClose   func()
}

type overlayPoint struct {
	x int32
	y int32
}

type overlayRect struct {
	left   int32
	top    int32
	right  int32
	bottom int32
}

type overlayMsg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      overlayPoint
}

type overlayPaintStruct struct {
	hdc         windows.Handle
	erase       int32
	rcPaint     overlayRect
	restore     int32
	incUpdate   int32
	rgbReserved [32]byte
}

type overlayWndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

func aiStatsNativeOverlaySupported() bool {
	return true
}

func newAIStatsNativeOverlay(onUnpin, onRefresh, onClose func()) *aiStatsNativeOverlay {
	return &aiStatsNativeOverlay{
		onUnpin:   onUnpin,
		onRefresh: onRefresh,
		onClose:   onClose,
	}
}

func (o *aiStatsNativeOverlay) Show(snapshot aitoolusage.RealtimeSnapshot, x, y int) bool {
	o.UpdateSnapshot(snapshot)
	o.mu.Lock()
	if o.running && o.hwnd != 0 {
		hwnd := o.hwnd
		o.mu.Unlock()
		overlayMoveWindow(hwnd, x, y)
		overlayInvalidate(hwnd)
		return true
	}
	if o.running {
		ready := o.ready
		o.mu.Unlock()
		select {
		case ok := <-ready:
			return ok
		case <-time.After(time.Second):
			return false
		}
	}
	o.x = x
	o.y = y
	o.ready = make(chan bool, 1)
	o.done = make(chan struct{})
	o.running = true
	ready := o.ready
	o.mu.Unlock()

	go o.run()

	select {
	case ok := <-ready:
		return ok
	case <-time.After(time.Second):
		return false
	}
}

func (o *aiStatsNativeOverlay) UpdateSnapshot(snapshot aitoolusage.RealtimeSnapshot) {
	o.mu.Lock()
	o.snapshot = snapshot
	hwnd := o.hwnd
	o.mu.Unlock()
	if hwnd != 0 {
		overlayInvalidate(hwnd)
	}
}

func (o *aiStatsNativeOverlay) Close() {
	o.mu.Lock()
	hwnd := o.hwnd
	o.mu.Unlock()
	if hwnd != 0 {
		procOverlayPostMessage.Call(uintptr(hwnd), wmClose, 0, 0)
	}
}

func (o *aiStatsNativeOverlay) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	overlayClassOnce.Do(registerAIStatsOverlayClass)

	overlayRegistryMu.Lock()
	overlayActive = o
	overlayRegistryMu.Unlock()

	hwnd, ok := o.createWindow()
	if !ok {
		o.signalReady(false)
		o.finish(false)
		return
	}

	overlayRegistryMu.Lock()
	overlayByHWND[hwnd] = o
	overlayRegistryMu.Unlock()

	o.mu.Lock()
	o.hwnd = hwnd
	o.mu.Unlock()

	key := overlayColorRef(overlayKeyR, overlayKeyG, overlayKeyB)
	ret, _, _ := procOverlaySetLayeredAttrs.Call(uintptr(hwnd), key, 255, lwaColorKey)
	if ret == 0 {
		procOverlayDestroyWindow.Call(uintptr(hwnd))
		o.signalReady(false)
		o.finish(false)
		return
	}
	procOverlayShowWindow.Call(uintptr(hwnd), swShowNoAct)
	procOverlayUpdateWindow.Call(uintptr(hwnd))
	overlayInvalidate(hwnd)

	o.signalReady(true)

	var msg overlayMsg
	for {
		ret, _, _ := procOverlayGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procOverlayTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procOverlayDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	o.finish(true)
}

func (o *aiStatsNativeOverlay) createWindow() (windows.Handle, bool) {
	style := uintptr(wsPopup)
	exStyle := uintptr(wsExLayered | wsExTopmost | wsExToolWin | wsExNoActive)
	instance := overlayModuleHandle()
	hwnd, _, _ := procOverlayCreateWindowEx.Call(
		exStyle,
		uintptr(unsafe.Pointer(overlayClassName)),
		uintptr(unsafe.Pointer(overlayTitle)),
		style,
		uintptr(int32(o.x)),
		uintptr(int32(o.y)),
		overlayWidth,
		overlayHeight,
		0,
		0,
		uintptr(instance),
		0,
	)
	return windows.Handle(hwnd), hwnd != 0
}

func (o *aiStatsNativeOverlay) signalReady(ok bool) {
	o.mu.Lock()
	ready := o.ready
	o.mu.Unlock()
	if ready == nil {
		return
	}
	select {
	case ready <- ok:
	default:
	}
}

func (o *aiStatsNativeOverlay) finish(_ bool) {
	o.mu.Lock()
	hwnd := o.hwnd
	done := o.done
	o.hwnd = 0
	o.running = false
	o.mu.Unlock()

	if hwnd != 0 {
		overlayRegistryMu.Lock()
		delete(overlayByHWND, hwnd)
		if overlayActive == o {
			overlayActive = nil
		}
		overlayRegistryMu.Unlock()
	}
	if done != nil {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}

func registerAIStatsOverlayClass() {
	cursor, _, _ := procOverlayLoadCursor.Call(0, 32512)
	wc := overlayWndClassEx{
		size:      uint32(unsafe.Sizeof(overlayWndClassEx{})),
		style:     0x0002 | 0x0001,
		wndProc:   overlayWndProc,
		instance:  overlayModuleHandle(),
		cursor:    windows.Handle(cursor),
		className: overlayClassName,
	}
	procOverlayRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
}

func aiStatsOverlayWndProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	overlay := lookupAIStatsOverlay(windows.Handle(hwnd))
	switch msg {
	case wmNCHitTest:
		if overlay != nil && overlay.hitTestScreenPoint(lParam) {
			return htClient
		}
		return htTransparent
	case wmLButtonUp:
		if overlay != nil {
			overlay.handleClick(localX(lParam), localY(lParam))
		}
		return 0
	case wmPaint:
		if overlay != nil {
			overlay.paint(windows.Handle(hwnd))
			return 0
		}
	case wmEraseBkgnd:
		return 1
	case wmClose:
		procOverlayDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procOverlayPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procOverlayDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func lookupAIStatsOverlay(hwnd windows.Handle) *aiStatsNativeOverlay {
	overlayRegistryMu.Lock()
	defer overlayRegistryMu.Unlock()
	if overlay := overlayByHWND[hwnd]; overlay != nil {
		return overlay
	}
	return overlayActive
}

func (o *aiStatsNativeOverlay) hitTestScreenPoint(lParam uintptr) bool {
	var rect overlayRect
	procOverlayGetWindowRect.Call(uintptr(o.hwnd), uintptr(unsafe.Pointer(&rect)))
	x := screenX(lParam) - int(rect.left)
	y := screenY(lParam) - int(rect.top)
	return overlayButtonAt(x, y) >= 0
}

func (o *aiStatsNativeOverlay) handleClick(x, y int) {
	switch overlayButtonAt(x, y) {
	case 0:
		if o.onUnpin != nil {
			o.onUnpin()
		}
	case 1:
		if o.onRefresh != nil {
			o.onRefresh()
		}
	case 2:
		if o.onClose != nil {
			o.onClose()
		}
	}
}

func (o *aiStatsNativeOverlay) paint(hwnd windows.Handle) {
	var ps overlayPaintStruct
	hdc, _, _ := procOverlayBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	defer procOverlayEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	full := overlayRect{left: 0, top: 0, right: overlayWidth, bottom: overlayHeight}
	brush, _, _ := procOverlayCreateSolidBrush.Call(overlayColorRef(overlayKeyR, overlayKeyG, overlayKeyB))
	procOverlayFillRect.Call(hdc, uintptr(unsafe.Pointer(&full)), brush)
	procOverlayDeleteObject.Call(brush)
	procOverlaySetBkMode.Call(hdc, bkTransparent)

	o.mu.Lock()
	snapshot := o.snapshot
	o.mu.Unlock()
	data := overlayDisplayData(snapshot)

	labelFont := overlayCreateFont(-12, 700, "Microsoft YaHei UI")
	valueFont := overlayCreateFont(-21, 800, "Segoe UI Variable Display")
	buttonFont := overlayCreateFont(-17, 400, "Segoe MDL2 Assets")
	defer procOverlayDeleteObject.Call(uintptr(labelFont))
	defer procOverlayDeleteObject.Call(uintptr(valueFont))
	defer procOverlayDeleteObject.Call(uintptr(buttonFont))

	drawOverlayMetric(hdc, labelFont, valueFont, 12, 17, "今日花费", data.todayCost)
	drawOverlayMetric(hdc, labelFont, valueFont, 174, 17, "今日请求", data.todayRequests)
	drawOverlayMetric(hdc, labelFont, valueFont, 12, 53, "累计花费", data.totalCost)
	drawOverlayMetric(hdc, labelFont, valueFont, 174, 53, "累计请求", data.totalRequests)

	drawOverlayButton(hdc, buttonFont, overlayButtonRects()[0], "\uE718", 0x00e8f49f)
	drawOverlayButton(hdc, buttonFont, overlayButtonRects()[1], "\uE72C", 0x00ffffff)
	drawOverlayButton(hdc, buttonFont, overlayButtonRects()[2], "\uE8BB", 0x00ffffff)
}

type overlayDisplay struct {
	todayCost     string
	todayRequests string
	totalCost     string
	totalRequests string
}

func overlayDisplayData(snapshot aitoolusage.RealtimeSnapshot) overlayDisplay {
	return overlayDisplay{
		todayCost:     overlayMoney(snapshot.Today.TotalCostUSD),
		todayRequests: overlayInt(snapshot.Today.TotalRequests),
		totalCost:     overlayMoney(snapshot.Total.TotalCostUSD),
		totalRequests: overlayInt(snapshot.Total.TotalRequests),
	}
}

func drawOverlayMetric(hdc uintptr, labelFont, valueFont windows.Handle, x, y int, label, value string) {
	overlayText(hdc, labelFont, x, y, label, overlayColorRef(184, 255, 246), true)
	overlayText(hdc, valueFont, x, y+14, value, overlayColorRef(255, 255, 255), true)
}

func drawOverlayButton(hdc uintptr, font windows.Handle, rect overlayRect, text string, color uintptr) {
	x := int(rect.left) + 7
	y := int(rect.top) + 5
	overlayText(hdc, font, x, y, text, color, true)
}

func overlayText(hdc uintptr, font windows.Handle, x, y int, text string, color uintptr, shadow bool) {
	old, _, _ := procOverlaySelectObject.Call(hdc, uintptr(font))
	defer procOverlaySelectObject.Call(hdc, old)
	if shadow {
		procOverlaySetTextColor.Call(hdc, overlayColorRef(0, 5, 8))
		for _, offset := range overlayTextHalo {
			overlayTextOut(hdc, x+offset.x, y+offset.y, text)
		}
	}
	procOverlaySetTextColor.Call(hdc, color)
	overlayTextOut(hdc, x, y, text)
}

type overlayTextOffset struct {
	x int
	y int
}

func overlayTextOut(hdc uintptr, x, y int, text string) {
	chars, err := windows.UTF16FromString(text)
	if err != nil || len(chars) == 0 {
		return
	}
	procOverlayTextOut.Call(
		hdc,
		uintptr(int32(x)),
		uintptr(int32(y)),
		uintptr(unsafe.Pointer(&chars[0])),
		uintptr(len(chars)-1),
	)
}

func overlayCreateFont(height, weight int, face string) windows.Handle {
	facePtr, _ := windows.UTF16PtrFromString(face)
	font, _, _ := procOverlayCreateFont.Call(
		uintptr(uint32(int32(height))),
		0,
		0,
		0,
		uintptr(weight),
		0,
		0,
		0,
		1,
		0,
		0,
		5,
		0,
		uintptr(unsafe.Pointer(facePtr)),
	)
	return windows.Handle(font)
}

func overlayModuleHandle() windows.Handle {
	handle, _, _ := procOverlayGetModuleHandle.Call(0)
	return windows.Handle(handle)
}

func overlayButtonRects() []overlayRect {
	return []overlayRect{
		{left: 232, top: 3, right: 260, bottom: 31},
		{left: 268, top: 3, right: 296, bottom: 31},
		{left: 304, top: 3, right: 332, bottom: 31},
	}
}

func overlayButtonAt(x, y int) int {
	for index, rect := range overlayButtonRects() {
		if x >= int(rect.left) && x <= int(rect.right) && y >= int(rect.top) && y <= int(rect.bottom) {
			return index
		}
	}
	return -1
}

func overlayMoveWindow(hwnd windows.Handle, x, y int) {
	procOverlaySetWindowPos.Call(uintptr(hwnd), ^uintptr(0), uintptr(int32(x)), uintptr(int32(y)), overlayWidth, overlayHeight, 0x0010)
	procOverlayShowWindow.Call(uintptr(hwnd), swShowNoAct)
}

func overlayInvalidate(hwnd windows.Handle) {
	procOverlayInvalidateRect.Call(uintptr(hwnd), 0, 1)
}

func overlayColorRef(r, g, b uint8) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

func overlayMoney(usd float64) string {
	cny := usd * 7.2
	decimals := 4
	if math.Abs(cny) >= 10 {
		decimals = 2
	}
	return "¥" + overlayFloat(cny, decimals)
}

func overlayFloat(value float64, decimals int) string {
	raw := fmt.Sprintf("%.*f", decimals, value)
	parts := strings.SplitN(raw, ".", 2)
	whole := overlayComma(parts[0])
	if len(parts) == 1 {
		return whole
	}
	return whole + "." + parts[1]
}

func overlayInt(value int64) string {
	return overlayComma(fmt.Sprintf("%d", value))
}

func overlayComma(value string) string {
	sign := ""
	if strings.HasPrefix(value, "-") {
		sign = "-"
		value = strings.TrimPrefix(value, "-")
	}
	if len(value) <= 3 {
		return sign + value
	}
	var builder strings.Builder
	builder.WriteString(sign)
	first := len(value) % 3
	if first == 0 {
		first = 3
	}
	builder.WriteString(value[:first])
	for i := first; i < len(value); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(value[i : i+3])
	}
	return builder.String()
}

func localX(lParam uintptr) int {
	return int(int16(uint16(lParam & 0xffff)))
}

func localY(lParam uintptr) int {
	return int(int16(uint16((lParam >> 16) & 0xffff)))
}

func screenX(lParam uintptr) int {
	return localX(lParam)
}

func screenY(lParam uintptr) int {
	return localY(lParam)
}
