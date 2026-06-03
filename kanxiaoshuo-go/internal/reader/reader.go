//go:build windows

package reader

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"kanxiaoshuo-go/internal/book"
	"kanxiaoshuo-go/internal/config"

	"golang.org/x/sys/windows"
)

const (
	wsExLayered    = 0x00080000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008

	wsPopup   = 0x80000000
	wsVisible = 0x10000000
	wsBorder  = 0x00800000

	esMultiline = 0x0004
	esReadonly  = 0x0800

	wmHotkey        = 0x0312
	wmDestroy       = 0x0002
	wmNCLButtonDown = 0x00A1
	wmNcPaint       = 0x0085
	wmNcCalcSize    = 0x0083
	wmEraseBkgnd    = 0x0014
	wmLButtonDown   = 0x0201
	wmSetCursor     = 0x0020
	htCaption       = 2

	idcArrow = 32512

	modAlt = 0x0001

	idHotkeyUp   = 1
	idHotkeyDown = 2
	idHotkeyHide = 3
	idHotkeyShow = 4
	idHotkeyMove = 5

	swHide = 0
	swShow = 5

	gwlStyle   = uintptr(0xFFFFFFFFFFFFFFF0)
	gwlExStyle = uintptr(0xFFFFFFFFFFFFFFEC)
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procSetCursorW       = user32.NewProc("SetCursor")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")

	wmMove uint32 = 0x0003
)

var (
	activeReader *Window
	readerMu     sync.Mutex
)

type wndclassex struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

type msg struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// Window 分层自绘透明阅读条
type Window struct {
	cfg             config.Settings
	onSave          func(config.Settings)
	hwnd            windows.Handle
	editHwnd        windows.Handle
	whiteBrush      uintptr // 色键白底画刷
	text            string // 错误信息；大书全文（bigText 模式）
	textRunes       []rune // 小书全书
	bigText         bool   // 大书用 string 按字节分块，避免 []rune 占用过大
	loadedBookPath  string
	charIndex       int    // 当前块起点（rune 下标）
	bufLineIndex    int    // 当前块内显示起始行
	chunkStart      int
	chunkEnd        int
	chunkText       string // 当前装入控件的原文（约 5 折行）
	chunkLines      int
	prefetchNext      chunkSnapshot
	prefetchNextReady bool
	prefetchForEnd    int
	prefetchPrev      chunkSnapshot
	prefetchPrevReady bool
	prefetchForStart  int // 预读块对应的当前 chunkStart
	keyMu           sync.Mutex
	fontHandle      windows.Handle
	running         bool

	loading      bool
	loadProgress int // 0–100；-1 表示不确定（跑马灯）
	loadStatus   string
	loadAnim     int
	loadGen      int
	loadMu       sync.Mutex
	pendingText  string
	pendingErr   error
	pendingPath  string

	applyMu        sync.Mutex
	hasPendingCfg  bool
	pendingCfg     config.Settings
}

func Start(cfg config.Settings, onSave func(config.Settings)) error {
	readerMu.Lock()
	defer readerMu.Unlock()
	if activeReader != nil && activeReader.hwnd != 0 {
		return nil
	}
	if activeReader != nil && activeReader.hwnd == 0 {
		activeReader = nil
	}
	cfg = config.Normalize(cfg)
	w := &Window{cfg: cfg, onSave: onSave}
	go w.run()
	return nil
}

func ApplySettings(cfg config.Settings) {
	readerMu.Lock()
	w := activeReader
	readerMu.Unlock()
	if w == nil {
		return
	}
	w.applyMu.Lock()
	w.pendingCfg = config.Normalize(cfg)
	w.hasPendingCfg = true
	w.applyMu.Unlock()
	w.postUI(wmApplySettings, 0, 0)
}

func (w *Window) handleApplySettings() {
	w.applyMu.Lock()
	if !w.hasPendingCfg {
		w.applyMu.Unlock()
		return
	}
	cfg := w.pendingCfg
	w.hasPendingCfg = false
	w.applyMu.Unlock()

	pathChanged := cfg.BookPath != w.loadedBookPath
	w.cfg = cfg

	if pathChanged || w.bookLen() == 0 {
		w.loadBook()
		return
	}

	w.applyFont()
	w.applyLayout()
	w.buildChunkAt(w.charIndex)
	w.clampBufLineIndex()
	w.applyColorKeyTransparency()
	w.paint()
}

func (w *Window) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className, _ := windows.UTF16PtrFromString("MoyuReaderBarV3")
	wndProc := windows.NewCallback(w.wndProc)

	inst, _, _ := procGetModuleHandleW()
	hbr, _, _ := gdi32.NewProc("CreateSolidBrush").Call(0x00FFFFFF)
	var wc wndclassex
	wc.Size = uint32(unsafe.Sizeof(wc))
	wc.WndProc = wndProc
	wc.Instance = inst
	wc.ClassName = className
	wc.Background = windows.Handle(hbr)
	cur, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc.Cursor = windows.Handle(cur)
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	left := int32(w.cfg.WindowLeft)
	top := int32(w.cfg.WindowTop)
	if left < 0 || top < 0 {
		left, top = 100, 100
	}
	title, _ := windows.UTF16PtrFromString("")

	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(wsExLayered|wsExToolWindow|wsExTopmost),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(wsPopup|wsVisible),
		uintptr(left), uintptr(top), defaultReaderWidth, 80,
		0, 0, uintptr(inst), 0,
	)
	if hwnd == 0 {
		readerMu.Lock()
		activeReader = nil
		readerMu.Unlock()
		showReaderError("无法创建阅读条窗口")
		return
	}
	w.hwnd = windows.Handle(hwnd)
	removeWindowBorder(hwnd)

	readerMu.Lock()
	activeReader = w
	readerMu.Unlock()

	w.applyFont()
	w.applyLayout()
	w.createEditHost()
	w.placeReaderCenter()
	w.beginLoading("准备中...", 0)
	w.ensureWindowVisible()

	w.registerHotkeys()
	w.running = true
	w.postUI(wmReaderInit, 0, 0)

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	w.unregisterHotkeys()
	w.deleteGdiObjects()
	if w.whiteBrush != 0 {
		procDeleteObject.Call(w.whiteBrush)
		w.whiteBrush = 0
	}
}

func procGetModuleHandleW() (windows.Handle, uintptr, error) {
	p := kernel32.NewProc("GetModuleHandleW")
	r, _, e := p.Call(0)
	return windows.Handle(r), r, e
}

func removeWindowBorder(hwnd uintptr) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	style &^= wsBorder
	style &^= 0x00C00000
	style &^= 0x00040000
	procSetWindowLongPtrW.Call(hwnd, gwlStyle, style)

	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	ex &^= 0x00000200
	ex &^= 0x00020000
	procSetWindowLongPtrW.Call(hwnd, gwlExStyle, ex)

	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, 0x0027)
}

func disableWindowTheme(hwnd uintptr) {
	ux := windows.NewLazySystemDLL("uxtheme.dll")
	p := ux.NewProc("SetWindowTheme")
	empty, _ := windows.UTF16PtrFromString("")
	p.Call(hwnd, uintptr(unsafe.Pointer(empty)), uintptr(unsafe.Pointer(empty)))
}

func (w *Window) wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmHotkey:
		w.handleHotkey(int(wParam))
		return 0
	case wmDestroy:
		readerMu.Lock()
		activeReader = nil
		readerMu.Unlock()
		procPostQuitMessage.Call(0)
		return 0
	case wmMove:
		w.savePosition()
	case 0x0005: // WM_SIZE
		w.resizeEdit()
	case wmCtlColorEdit:
		return w.handleCtlColorEdit(wParam)
	case wmNcPaint, wmNcCalcSize:
		return 0
	case wmEraseBkgnd:
		if w.whiteBrush != 0 {
			return w.whiteBrush
		}
		return 1
	case wmLButtonDown:
		procReleaseCapture.Call()
		procSendMessageW.Call(uintptr(w.hwnd), uintptr(wmNCLButtonDown), uintptr(htCaption), 0)
		return 0
	case 0x0084: // WM_NCHITTEST：整块区域可拖动
		return 2 // HTCAPTION
	case wmSetCursor:
		cur, _, _ := procLoadCursorW.Call(0, idcArrow)
		procSetCursorW.Call(cur)
		return 1
	case wmLoadProgress:
		w.loadProgress = int(wParam)
		if w.loadProgress > 100 {
			w.loadProgress = 100
		}
		w.paint()
		return 0
	case wmLoadDone:
		w.finishLoad()
		return 0
	case wmReaderInit:
		w.loadBook()
		return 0
	case wmApplySettings:
		w.handleApplySettings()
		return 0
	case wmPrefetchNext:
		w.doPrefetchNext()
		return 0
	case wmPrefetchPrev:
		w.doPrefetchPrev()
		return 0
	case wmDeferPrefetch:
		if !w.loading {
			w.schedulePrefetchNext()
			w.schedulePrefetchPrev()
		}
		return 0
	case 0x0113: // WM_TIMER
		if wParam == timerLoadAnim && w.loading {
			w.loadAnim++
			w.paint()
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func (w *Window) applyFont() {
	if w.fontHandle != 0 {
		procDeleteObject.Call(uintptr(w.fontHandle))
		w.fontHandle = 0
	}
	size := w.cfg.FontSize
	if size < 3 {
		size = 3
	}
	if size > 24 {
		size = 24
	}
	height := -int32(size)
	name, _ := windows.UTF16PtrFromString("Microsoft YaHei UI")
	h, _, _ := procCreateFontW.Call(
		uintptr(height), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(name)),
	)
	w.fontHandle = windows.Handle(h)
	if w.editHwnd != 0 {
		procSendMessageW.Call(uintptr(w.editHwnd), wmSetFont, uintptr(w.fontHandle), 1)
		w.enforceReadOnly()
	}
	w.applyLayout()
}

func (w *Window) deleteGdiObjects() {
	if w.fontHandle != 0 {
		procDeleteObject.Call(uintptr(w.fontHandle))
	}
}

func (w *Window) postUI(msg uint32, wParam, lParam uintptr) {
	if w.hwnd == 0 {
		return
	}
	p := user32.NewProc("PostMessageW")
	p.Call(uintptr(w.hwnd), uintptr(msg), wParam, lParam)
}

func (w *Window) loadBook() {
	path := w.cfg.BookPath
	if path == "" {
		return
	}

	w.loadMu.Lock()
	w.loadGen++
	gen := w.loadGen
	w.loadMu.Unlock()

	w.textRunes = nil
	w.bigText = false
	w.text = ""
	w.chunkText = ""
	w.invalidatePrefetch()
	w.beginLoading("正在读取...", 0)
	w.applyLayout()
	w.ensureWindowVisible()
	w.paint()

	hwnd := w.hwnd
	go func() {
		var lastPct int
		text, err := book.LoadWithProgress(path, func(p int) {
			if hwnd == 0 || p < lastPct {
				return
			}
			if p-lastPct < 2 && p < 100 {
				return
			}
			lastPct = p
			w.postUI(wmLoadProgress, uintptr(p), 0)
		})

		w.loadMu.Lock()
		stale := w.loadGen != gen
		if !stale {
			w.pendingText = text
			w.pendingErr = err
			w.pendingPath = path
		}
		w.loadMu.Unlock()
		if stale {
			return
		}
		w.postUI(wmLoadDone, 0, 0)
	}()
}

func (w *Window) finishLoad() {
	w.loadMu.Lock()
	text := w.pendingText
	err := w.pendingErr
	path := w.pendingPath
	w.loadMu.Unlock()

	if err != nil {
		w.bigText = false
		w.text = fmt.Sprintf("无法读取文件: %v", err)
		w.textRunes = []rune(w.text)
	} else {
		w.setBookRunes(text)
	}
	w.loadedBookPath = path
	w.initCharIndexFromConfig()
	w.endLoading()
	w.applyLayout()
	w.placeReaderCenter()
	w.ensureWindowVisible()
	w.paint()
	w.savePosition()
	w.cfg.CharIndex = w.charIndex
	_ = config.Save(w.cfg)
}

func (w *Window) savePosition() {
	var rect struct {
		Left, Top, Right, Bottom int32
	}
	procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&rect)))
	w.cfg.WindowLeft = float64(rect.Left)
	w.cfg.WindowTop = float64(rect.Top)
}

func (w *Window) persist() {
	w.savePosition()
	w.cfg.CharIndex = w.charIndex
	w.cfg.LineIndex = w.bufLineIndex
	_ = config.Save(w.cfg)
	if w.onSave != nil {
		w.onSave(w.cfg)
	}
}

func (w *Window) registerHotkeys() {
	base := uintptr(w.hwnd)
	procRegisterHotKey.Call(base, idHotkeyUp, 0, 0x26)
	procRegisterHotKey.Call(base, idHotkeyDown, 0, 0x28)
	procRegisterHotKey.Call(base, idHotkeyHide, modAlt, 0x43)
	procRegisterHotKey.Call(base, idHotkeyShow, modAlt, 0x53)
	procRegisterHotKey.Call(base, idHotkeyMove, modAlt, 0x54)
}

func (w *Window) unregisterHotkeys() {
	base := uintptr(w.hwnd)
	procUnregisterHotKey.Call(base, idHotkeyUp)
	procUnregisterHotKey.Call(base, idHotkeyDown)
	procUnregisterHotKey.Call(base, idHotkeyHide)
	procUnregisterHotKey.Call(base, idHotkeyShow)
	procUnregisterHotKey.Call(base, idHotkeyMove)
}

func (w *Window) handleHotkey(id int) {
	w.keyMu.Lock()
	defer w.keyMu.Unlock()

	switch id {
	case idHotkeyHide:
		procShowWindow.Call(uintptr(w.hwnd), swHide)
	case idHotkeyShow:
		procShowWindow.Call(uintptr(w.hwnd), swShow)
		procSetWindowPos.Call(uintptr(w.hwnd), ^uintptr(0), 0, 0, 0, 0, 0x0003|0x0010)
	case idHotkeyMove:
		procSetWindowPos.Call(uintptr(w.hwnd), 0, 100, 100, 0, 0, 0x0001|0x0004)
		w.paint()
	case idHotkeyUp:
		w.pageUp()
		w.syncCharIndexFromView()
		w.paint()
		w.persist()
	case idHotkeyDown:
		w.pageDown()
		w.syncCharIndexFromView()
		w.paint()
		w.persist()
	}
}
