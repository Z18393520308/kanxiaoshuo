//go:build windows

package reader

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

const (
	wsChild          = 0x40000000
	esAutoVScroll    = 0x0040
	wmSetFont        = 0x0030
	emSetReadOnly    = 0x00CF
	emSetLimit       = 0x00C5
	emSetMargins     = 0x00D3
	emSetRectNP      = 0x00B4
	wmCtlColorEdit   = 0x0133
	wmCtlColorStatic = 0x0138
	lwaColorKey      = 0x00000001
	gwlWndProc       = ^uintptr(3)
)

var (
	procSetWindowTextW             = user32.NewProc("SetWindowTextW")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procCallWindowProcW            = user32.NewProc("CallWindowProcW")
	procSetBkColor                 = gdi32.NewProc("SetBkColor")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
	procHideCaret                  = user32.NewProc("HideCaret")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
)

func (w *Window) createEditHost() error {
	if err := richEditDLL.Load(); err != nil {
		return fmt.Errorf("加载阅读排版组件失败: %w", err)
	}
	class, _ := windows.UTF16PtrFromString("RICHEDIT50W")
	empty, _ := windows.UTF16PtrFromString("")
	inst, _, _ := procGetModuleHandleW()
	h, _, e := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(empty)), wsChild|wsVisible|esMultiline|esReadonly|esAutoVScroll, 0, 0, uintptr(w.cfg.WindowWidth), 80, uintptr(w.hwnd), 0, uintptr(inst), 0)
	if h == 0 {
		return fmt.Errorf("创建阅读文本控件失败: %w", e)
	}
	w.editHwnd = windows.Handle(h)
	windowLookup.Store(w.editHwnd, w)
	theme := windows.NewLazySystemDLL("uxtheme.dll")
	theme.NewProc("SetWindowTheme").Call(h, uintptr(unsafe.Pointer(empty)), uintptr(unsafe.Pointer(empty)))
	w.oldEditProc, _, _ = procSetWindowLongPtrW.Call(h, gwlWndProc, readerEditCallback)
	procSendMessageW.Call(h, emSetTextMode, 1, 0)          // TM_PLAINTEXT，仅显示书籍原文。
	procSendMessageW.Call(h, emSetTypographyOptions, 1, 1) // TO_ADVANCEDTYPOGRAPHY，实际折行计入字距。
	procSendMessageW.Call(h, emSetReadOnly, 1, 0)
	procSendMessageW.Call(h, emSetLimit, chunkBytes*2, 0)
	procSendMessageW.Call(h, emSetMargins, 3, 0)
	return nil
}

func (w *Window) editWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case 0x0100, 0x0102, 0x0104, 0x0300, 0x0301, 0x0302, 0x0303, 0x007b, 0x0203, 0x0204, 0x0205:
		// 文本只读，不允许键盘滚动或选择改变自动保存的首行位置。
		return 0
	case 0x020a:
		if !w.visible {
			return 0
		}
		delta := int16(wParam >> 16)
		if delta > 0 {
			w.pageUp()
		} else if delta < 0 {
			w.pageDown()
		}
		if err := w.persist(); err != nil {
			showReaderError(err.Error())
		}
		return 0
	case wmLButtonDown:
		procReleaseCapture.Call()
		procSendMessageW.Call(uintptr(w.hwnd), wmNCLButtonDown, htCaption, 0)
		return 0
	}
	r, _, _ := procCallWindowProcW.Call(w.oldEditProc, hwnd, uintptr(message), wParam, lParam)
	if message == 0x000f || message == 0x0007 {
		procHideCaret.Call(hwnd)
	}
	return r
}

// 使用与文字不同的色键，黑字和白字都能完整显示。
func (w *Window) updateTransparency() {
	r, g, b := parseHexColor(w.cfg.FontColor)
	key := uint32(0x00010101)
	if bgr(r, g, b) == key {
		key = 0x00020202
	}
	if w.brush != 0 {
		procDeleteObject.Call(w.brush)
	}
	w.colorKey = key
	w.brush, _, _ = procCreateSolidBrush.Call(uintptr(key))
	procSetLayeredWindowAttributes.Call(uintptr(w.hwnd), uintptr(key), 0, lwaColorKey)
	procSendMessageW.Call(uintptr(w.editHwnd), emSetBkgndColor, 0, uintptr(key))
	w.applyTextFormat()
	procInvalidateRect.Call(uintptr(w.hwnd), 0, 1)
	procInvalidateRect.Call(uintptr(w.editHwnd), 0, 1)
}
func (w *Window) handleCtlColorEdit(dc uintptr) uintptr {
	r, g, b := parseHexColor(w.cfg.FontColor)
	procSetBkColor.Call(dc, uintptr(w.colorKey))
	procSetTextColor.Call(dc, uintptr(bgr(r, g, b)))
	return w.brush
}
func (w *Window) resizeEdit() {
	if w.hwnd == 0 || w.editHwnd == 0 {
		return
	}
	var rc rect
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
	procSetWindowPos.Call(uintptr(w.editHwnd), 0, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), 0x0014)
	format := rect{4, 4, max(5, rc.Right-4), max(5, rc.Bottom-4)}
	procSendMessageW.Call(uintptr(w.editHwnd), emSetRectNP, 0, uintptr(unsafe.Pointer(&format)))
}
func (w *Window) setEditText(text string) {
	p, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	// 分块可能为了填满一屏而扩展；Rich Edit 的字符上限同步扩展，避免截断正文。
	procSendMessageW.Call(uintptr(w.editHwnd), 0x0400+53, 0, uintptr(max(chunkBytes*2, richEditLength(text)+1))) // EM_EXLIMITTEXT
	procSetWindowTextW.Call(uintptr(w.editHwnd), uintptr(unsafe.Pointer(p)))
	w.applyTextFormat()
	procHideCaret.Call(uintptr(w.editHwnd))
}
