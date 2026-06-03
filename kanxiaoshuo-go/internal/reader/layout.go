//go:build windows

package reader

import (
	"unsafe"
)

const (
	defaultReaderWidth = 480
	padX               = 0
	padY               = 0
)

func (w *Window) lineHeightPx() int {
	if w.fontHandle == 0 {
		return int(w.cfg.FontSize) + 4
	}
	hdc, _, _ := procGetDC.Call(uintptr(w.hwnd))
	if hdc == 0 {
		return int(w.cfg.FontSize) + 4
	}
	defer procReleaseDC.Call(uintptr(w.hwnd), hdc)

	procSelectObject.Call(hdc, uintptr(w.fontHandle))
	var tm struct {
		Height, Ascent, Descent, InternalLeading, ExternalLeading int32
		AveCharWidth, BreakChar                                   int32
		Italic, Underline, StrikeOut, CharSet, OutPrecision       byte
		ClipPrecision, Quality, PitchAndFamily                     byte
	}
	procGetTextMetricsW.Call(hdc, uintptr(unsafe.Pointer(&tm)))
	// 使用 ascent+descent 保证整行字形不被裁切
	h := int(tm.Ascent) + int(tm.Descent) + int(tm.ExternalLeading)
	if h < int(tm.Height) {
		h = int(tm.Height)
	}
	if h < 8 {
		h = int(w.cfg.FontSize)*2 + 4
	}
	return h
}

func (w *Window) applyLayout() {
	lines := w.cfg.VisibleLines
	if lines < 1 {
		lines = 1
	}
	if lines > 15 {
		lines = 15
	}

	lh := w.lineHeightPx()
	// 每行留足高度，避免大字号时下半截被裁切出现黑块
	winH := lh*lines + 8
	if winH < lh+6 {
		winH = lh + 6
	}
	winW := defaultReaderWidth

	if w.hwnd != 0 {
		var rect struct{ Left, Top, Right, Bottom int32 }
		procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rect)))
		procSetWindowPos.Call(
			uintptr(w.hwnd), ^uintptr(0),
			uintptr(rect.Left), uintptr(rect.Top),
			uintptr(winW), uintptr(winH),
			0x0010,
		)
	}
}

var (
	procGetDC                   = user32.NewProc("GetDC")
	procReleaseDC               = user32.NewProc("ReleaseDC")
	procSelectObject            = gdi32.NewProc("SelectObject")
	procGetTextMetricsW         = gdi32.NewProc("GetTextMetricsW")
	procGetWindowRect           = user32.NewProc("GetWindowRect")
	procSystemParametersInfoW   = user32.NewProc("SystemParametersInfoW")
)
