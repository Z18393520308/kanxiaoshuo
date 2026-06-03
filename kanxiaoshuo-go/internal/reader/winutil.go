//go:build windows

package reader

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
)

func showReaderError(msg string) {
	u := windows.NewLazySystemDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	t, _ := windows.UTF16PtrFromString(msg)
	c, _ := windows.UTF16PtrFromString("摸鱼联盟 · 阅读条")
	_, _, _ = p.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x10)
}

func (w *Window) placeReaderCenter() {
	if w.hwnd == 0 {
		return
	}
	var wa struct{ Left, Top, Right, Bottom int32 }
	procSystemParametersInfoW.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0)
	if wa.Right <= wa.Left {
		wa.Left, wa.Top, wa.Right, wa.Bottom = 0, 0, 1920, 1080
	}
	wW := int32(defaultReaderWidth)
	lh := int32(w.lineHeightPx())
	vis := int32(w.visibleLines())
	wH := lh*vis + 8
	if wH < 32 {
		wH = 32
	}
	left := wa.Left + (wa.Right-wa.Left-wW)/2
	top := wa.Top + 120
	procSetWindowPos.Call(
		uintptr(w.hwnd), ^uintptr(0),
		uintptr(left), uintptr(top),
		uintptr(wW), uintptr(wH),
		0x0010|0x0004,
	)
	w.cfg.WindowLeft = float64(left)
	w.cfg.WindowTop = float64(top)
}
