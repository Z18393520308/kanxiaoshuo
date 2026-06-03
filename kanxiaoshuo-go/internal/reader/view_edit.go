//go:build windows

package reader

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsChild       = 0x40000000
	esAutoVScroll = 0x0040
	wmSetFont     = 0x0030
	emSetReadOnly = 0x00CF
	emLineScroll  = 0x00B6
	emSetLimit     = 0x00D1
	emSetSel       = 0x00B1
	wmCtlColorEdit = 0x0138
	wsExClientEdge = 0x00000200
	wsExWindowEdge = 0x00000100
	wsExStaticEdge = 0x00020000
	lwaColorKey   = 0x00000001
	// 色键：纯白底变透明，文字颜色不参与色键
	colorKeyBGR = 0x00FFFFFF
	gwlWndProc  = ^uintptr(3) // GWLP_WNDPROC = -4
)

var (
	procSetWindowTextW              = user32.NewProc("SetWindowTextW")
	procGetClientRect               = user32.NewProc("GetClientRect")
	procSetLayeredWindowAttributes  = user32.NewProc("SetLayeredWindowAttributes")
	procCallWindowProcW             = user32.NewProc("CallWindowProcW")
	procSetWindowLongPtrWForEdit    = user32.NewProc("SetWindowLongPtrW")
	procSetBkColor     = gdi32.NewProc("SetBkColor")
	procHideCaret      = user32.NewProc("HideCaret")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procGetParent        = user32.NewProc("GetParent")
	editWndProcOld       uintptr
)

func (w *Window) createEditHost() {
	if w.hwnd == 0 {
		return
	}
	editClass, _ := windows.UTF16PtrFromString("EDIT")
	empty, _ := windows.UTF16PtrFromString("")
	inst, _, _ := procGetModuleHandleW()

	h, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(empty)),
		uintptr(wsChild|wsVisible|esMultiline|esReadonly|esAutoVScroll),
		2, 2, defaultReaderWidth-4, 64,
		uintptr(w.hwnd), 0, uintptr(inst), 0,
	)
	if h == 0 {
		showReaderError("无法创建阅读编辑框")
		return
	}
	w.editHwnd = windows.Handle(h)
	stripEditChrome(h)
	disableWindowTheme(h)
	w.hookEditReadOnly(h)
	w.enforceReadOnly()
	if w.fontHandle != 0 {
		procSendMessageW.Call(h, wmSetFont, uintptr(w.fontHandle), 1)
	}
	w.resizeEdit()
	w.applyColorKeyTransparency()
	w.updateEditDisplay()
}

func (w *Window) hookEditReadOnly(editHwnd uintptr) {
	editWndProc := windows.NewCallback(editWndProc)
	editWndProcOld, _, _ = procSetWindowLongPtrWForEdit.Call(editHwnd, gwlWndProc, editWndProc)
}

func stripEditChrome(hwnd uintptr) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= wsBorder
	style &^= 0x00400000 // WS_THICKFRAME
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	ex &^= wsExClientEdge
	ex &^= wsExWindowEdge
	ex &^= wsExStaticEdge
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex)
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, 0x0027)
}

func clearEditSelection(hwnd uintptr) {
	procSendMessageW.Call(hwnd, emSetSel, 0, 0)
	procHideCaret.Call(hwnd)
}

// beginDragWindow 在编辑框上按下左键时，交给父窗口当标题栏拖动
func beginDragWindow(editHwnd uintptr) {
	parent, _, _ := procGetParent.Call(editHwnd)
	if parent == 0 {
		return
	}
	procReleaseCapture.Call()
	procSendMessageW.Call(parent, uintptr(wmNCLButtonDown), uintptr(htCaption), 0)
}

func editWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case 0x0102: // WM_CHAR
		return 0
	case 0x0302: // WM_PASTE
		return 0
	case 0x007B: // WM_CONTEXTMENU
		return 0
	case 0x0085: // WM_NCPAINT
		return 0
	case 0x0201: // WM_LBUTTONDOWN：拖动阅读条
		beginDragWindow(hwnd)
		return 0
	case 0x0203, 0x0204: // 双击 / 右键：禁止选字
		return 0
	case 0x0100: // WM_KEYDOWN：禁止 Ctrl+A、方向键扩选
		vk := wParam & 0xFF
		if vk == 0x41 {
			if st, _, _ := procGetAsyncKeyState.Call(0x11); st&0x8000 != 0 {
				return 0
			}
		}
		if vk >= 0x21 && vk <= 0x28 {
			return 0
		}
	}
	if editWndProcOld != 0 {
		r, _, _ := procCallWindowProcW.Call(editWndProcOld, hwnd, uintptr(msg), wParam, lParam)
		switch msg {
		case 0x000F, 0x0007, 0x0200: // WM_PAINT / WM_SETFOCUS / WM_MOUSEMOVE
			clearEditSelection(hwnd)
		}
		return r
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (w *Window) enforceReadOnly() {
	if w.editHwnd == 0 {
		return
	}
	h := uintptr(w.editHwnd)
	procSendMessageW.Call(h, emSetReadOnly, 1, 0)
	clearEditSelection(h)
}

func (w *Window) applyColorKeyTransparency() {
	if w.hwnd == 0 {
		return
	}
	procSetLayeredWindowAttributes.Call(
		uintptr(w.hwnd),
		uintptr(colorKeyBGR),
		0,
		uintptr(lwaColorKey),
	)
}

func (w *Window) handleCtlColorEdit(hdc uintptr) uintptr {
	r, g, b := parseHexColor(w.cfg.FontColor)
	if r == 0 && g == 0 && b == 0 {
		b = 1
	}
	procSetBkColor.Call(hdc, uintptr(colorKeyBGR))
	procSetTextColor.Call(hdc, uintptr(bgr(r, g, b)))
	if w.whiteBrush != 0 {
		return w.whiteBrush
	}
	br, _, _ := gdi32.NewProc("CreateSolidBrush").Call(colorKeyBGR)
	w.whiteBrush = br
	return br
}

func (w *Window) resizeEdit() {
	if w.editHwnd == 0 || w.hwnd == 0 {
		return
	}
	var rc struct{ Left, Top, Right, Bottom int32 }
	procGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rc)))
	procSetWindowPos.Call(
		uintptr(w.editHwnd), 0,
		0, 0,
		uintptr(rc.Right-rc.Left), uintptr(rc.Bottom-rc.Top),
		0x0010|0x0004,
	)
}

func (w *Window) setEditText(s string) {
	if w.editHwnd == 0 {
		return
	}
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return
	}
	procSetWindowTextW.Call(uintptr(w.editHwnd), uintptr(unsafe.Pointer(p)))
	w.enforceReadOnly()
}

func (w *Window) updateEditDisplay() {
	if w.editHwnd == 0 {
		return
	}
	if w.fontHandle != 0 {
		procSendMessageW.Call(uintptr(w.editHwnd), wmSetFont, uintptr(w.fontHandle), 1)
	}

	var show string
	if w.loading {
		if w.loadProgress >= 0 {
			show = fmt.Sprintf("%s  %d%%", w.loadStatus, w.loadProgress)
		} else {
			show = w.loadStatus
		}
		if show == "" {
			show = "加载中..."
		}
	} else {
		show = w.chunkText
		if show == "" {
			show = w.text
		}
		if show == "" {
			show = "（无内容）"
		}
	}
	w.setEditText(show)

	if !w.loading && w.bufLineIndex > 0 {
		procSendMessageW.Call(uintptr(w.editHwnd), emLineScroll, 0, uintptr(w.bufLineIndex))
	}
	clearEditSelection(uintptr(w.editHwnd))
}

func (w *Window) paint() {
	w.updateEditDisplay()
}
