//go:build windows

package reader

import "unsafe"

// ensureWindowVisible 保证阅读条在屏幕内并置顶显示
func (w *Window) ensureWindowVisible() {
	if w.hwnd == 0 {
		return
	}
	w.clampWindowOnScreen()
	procShowWindow.Call(uintptr(w.hwnd), swShow)
	procSetWindowPos.Call(uintptr(w.hwnd), ^uintptr(0), 0, 0, 0, 0, 0x0003|0x0010)
	procBringWindowToTop.Call(uintptr(w.hwnd))
	procSetForegroundWindow.Call(uintptr(w.hwnd))
}

func (w *Window) clampWindowOnScreen() {
	if w.hwnd == 0 {
		return
	}
	var wr struct{ Left, Top, Right, Bottom int32 }
	procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&wr)))
	wW := wr.Right - wr.Left
	wH := wr.Bottom - wr.Top
	if wW < 80 {
		wW = defaultReaderWidth
	}
	if wH < 24 {
		wH = 48
	}

	var wa struct{ Left, Top, Right, Bottom int32 }
	procSystemParametersInfoW.Call(0x0030, 0, uintptr(unsafe.Pointer(&wa)), 0) // SPI_GETWORKAREA
	if wa.Right <= wa.Left {
		wa.Left, wa.Top, wa.Right, wa.Bottom = 0, 0, 1920, 1080
	}

	off := wr.Left < wa.Left-200 || wr.Top < wa.Top-200 ||
		wr.Left > wa.Right+200 || wr.Top > wa.Bottom+200
	if off || wr.Left < -500 || wr.Top < -500 {
		left := wa.Left + 80
		top := wa.Top + 80
		procSetWindowPos.Call(
			uintptr(w.hwnd), 0,
			uintptr(left), uintptr(top),
			uintptr(wW), uintptr(wH),
			0x0010|0x0004, // SWP_NOACTIVATE | SWP_NOZORDER
		)
		w.cfg.WindowLeft = float64(left)
		w.cfg.WindowTop = float64(top)
	}
}
