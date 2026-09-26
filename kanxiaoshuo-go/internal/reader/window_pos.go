//go:build windows

package reader

import "unsafe"

var (
	procMonitorFromRect = user32.NewProc("MonitorFromRect")
	procGetMonitorInfoW = user32.NewProc("GetMonitorInfoW")
)

type monitorInfo struct {
	Size          uint32
	Monitor, Work rect
	Flags         uint32
}

func (w *Window) ensureWindowVisible() {
	w.clampWindowOnScreen()
	procShowWindow.Call(uintptr(w.hwnd), swShowNoActivate)
	procSetWindowPos.Call(uintptr(w.hwnd), ^uintptr(0), 0, 0, 0, 0, 0x0013)
}
func (w *Window) clampWindowOnScreen() {
	if w.hwnd == 0 {
		return
	}
	var r rect
	if ok, _, _ := procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok == 0 {
		return
	}
	monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&r)), 2) // MONITOR_DEFAULTTONEAREST
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return
	}
	left, top := clampRect(r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top, info.Work.Left, info.Work.Top, info.Work.Right, info.Work.Bottom)
	if left != r.Left || top != r.Top {
		procSetWindowPos.Call(uintptr(w.hwnd), 0, uintptr(left), uintptr(top), 0, 0, 0x0015)
	}
	w.savePosition()
}
