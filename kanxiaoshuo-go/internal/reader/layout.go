//go:build windows

package reader

import (
	"fmt"
	"golang.org/x/sys/windows"
	"kanxiaoshuo-go/internal/config"
	"unsafe"
)

// textMetricW 必须精确对应 Windows TEXTMETRICW（60 字节），不能省略中间字段。
type textMetricW struct {
	Height, Ascent, Descent, InternalLeading, ExternalLeading                        int32
	AveCharWidth, MaxCharWidth, Weight, Overhang, DigitizedAspectX, DigitizedAspectY int32
	FirstChar, LastChar, DefaultChar, BreakChar                                      uint16
	Italic, Underlined, StruckOut, PitchAndFamily, CharSet                           byte
}

var (
	procGetDC           = user32.NewProc("GetDC")
	procReleaseDC       = user32.NewProc("ReleaseDC")
	procSelectObject    = gdi32.NewProc("SelectObject")
	procGetTextMetricsW = gdi32.NewProc("GetTextMetricsW")
	procGetWindowRect   = user32.NewProc("GetWindowRect")
)

func createFont(cfg config.Settings) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(cfg.FontFamily)
	if err != nil {
		return 0, fmt.Errorf("字体名称无效: %w", err)
	}
	h, _, e := procCreateFontW.Call(uintptr(-int32(cfg.FontSize)), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 3, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return 0, fmt.Errorf("创建字体失败: %w", e)
	}
	return windows.Handle(h), nil
}
func (w *Window) lineHeightPx() int {
	// Rich Edit 的实际行高可能包含字体回退/额外行距，直接测量相邻行。
	second, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emLineIndex, 1, 0)
	if int32(second) > 0 {
		firstPoint, secondPoint := w.characterPoint(0), w.characterPoint(int(second))
		if height := secondPoint.Y - firstPoint.Y; height > 0 {
			return int(height)
		}
	}
	dc, _, _ := procGetDC.Call(uintptr(w.editHwnd))
	if dc == 0 {
		return w.cfg.FontSize + 4
	}
	defer procReleaseDC.Call(uintptr(w.editHwnd), dc)
	old, _, _ := procSelectObject.Call(dc, uintptr(w.fontHandle))
	defer procSelectObject.Call(dc, old)
	var tm textMetricW
	r, _, _ := procGetTextMetricsW.Call(dc, uintptr(unsafe.Pointer(&tm)))
	if r == 0 {
		return w.cfg.FontSize + 4
	}
	// 控件尚未完成排版时回退到字体度量。
	return max(1, int(tm.Height))
}
func (w *Window) applyFontAndLayout() {
	procSendMessageW.Call(uintptr(w.editHwnd), wmSetFont, uintptr(w.fontHandle), 0)
	w.setEditText("国Ag\r\n国Ag")
	var r rect
	procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r)))
	height := w.lineHeightPx()*w.visibleLines() + 8
	procSetWindowPos.Call(uintptr(w.hwnd), ^uintptr(0), uintptr(r.Left), uintptr(r.Top), uintptr(w.cfg.WindowWidth), uintptr(height), 0x0010)
	w.resizeEdit()
}
