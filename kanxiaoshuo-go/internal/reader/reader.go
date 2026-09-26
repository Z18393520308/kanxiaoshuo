//go:build windows

package reader

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
	"kanxiaoshuo-go/internal/book"
	"kanxiaoshuo-go/internal/config"
	"kanxiaoshuo-go/internal/hotkey"
)

const (
	wsExLayered      = 0x00080000
	wsExToolWindow   = 0x00000080
	wsExTopmost      = 0x00000008
	wsPopup          = 0x80000000
	wsVisible        = 0x10000000
	wsBorder         = 0x00800000
	esMultiline      = 0x0004
	esReadonly       = 0x0800
	wmHotkey         = 0x0312
	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmNCLButtonDown  = 0x00A1
	wmEraseBkgnd     = 0x0014
	wmLButtonDown    = 0x0201
	wmExitSizeMove   = 0x0232
	wmCommandReader  = 0x8000 + 32
	htCaption        = 2
	idcArrow         = 32512
	idHotkeyUp       = 1
	idHotkeyDown     = 2
	idHotkeyHide     = 3
	idHotkeyShow     = 4
	idHotkeyMove     = 5
	swHide           = 0
	swShowNoActivate = 4
	gwlStyle         = ^uintptr(15)
	gwlExStyle       = ^uintptr(19)
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	gdi32                 = windows.NewLazySystemDLL("gdi32.dll")
	procRegisterClassExW  = user32.NewProc("RegisterClassExW")
	procUnregisterClassW  = user32.NewProc("UnregisterClassW")
	procCreateWindowExW   = user32.NewProc("CreateWindowExW")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procSendMessageW      = user32.NewProc("SendMessageW")
	procPostMessageW      = user32.NewProc("PostMessageW")
	procPeekMessageW      = user32.NewProc("PeekMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCreateFontW       = gdi32.NewProc("CreateFontW")
	procDeleteObject      = gdi32.NewProc("DeleteObject")
	procLoadCursorW       = user32.NewProc("LoadCursorW")
	procReleaseCapture    = user32.NewProc("ReleaseCapture")
	procCreateSolidBrush  = gdi32.NewProc("CreateSolidBrush")
	procFillRect          = user32.NewProc("FillRect")
	apiMu                 sync.Mutex
	activeReader          *Window
	classSerial           atomic.Uint64
	windowLookup          sync.Map
	readerWindowCallback  = windows.NewCallback(readerWindowProc)
	readerEditCallback    = windows.NewCallback(readerEditProc)
)

type wndclassex struct {
	Size                               uint32
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

type msg struct {
	Hwnd           windows.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             struct{ X, Y int32 }
	Private        uint32
}

type request struct {
	run    func() error
	result chan error
}

// Window 的所有 Win32 控件、配置和分页状态只允许在 run 的专用 OS 线程使用。
// 外部接口通过命令队列同步获取操作结果，不跨线程直接操作控件。
type Window struct {
	cfg                  config.Settings
	onSave               func(config.Settings)
	hwnd, editHwnd       windows.Handle
	fontHandle           windows.Handle
	brush                uintptr
	colorKey             uint32
	oldEditProc          uintptr
	text                 string
	chunkText            string
	chunkStart, chunkEnd int
	position             int
	previous             []int
	forward              []int
	visible              bool
	registered           map[int]bool
	commands             chan request
	done                 chan struct{}
}

// 系统回调只注册一次，避免关闭/重开永久保留捕获整本书的 Go 闭包。
func readerWindowProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	if entry, ok := windowLookup.Load(hwnd); ok {
		return entry.(*Window).wndProc(hwnd, message, wParam, lParam)
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return r
}

func readerEditProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if entry, ok := windowLookup.Load(windows.Handle(hwnd)); ok {
		return entry.(*Window).editWndProc(hwnd, message, wParam, lParam)
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func liveReader() *Window {
	if activeReader != nil {
		select {
		case <-activeReader.done:
			activeReader = nil
		default:
		}
	}
	return activeReader
}

// Start 等待窗口、热键和书本均准备完成后返回。已有窗口时应用设置并显示。
func Start(cfg config.Settings, onSave func(config.Settings)) error {
	apiMu.Lock()
	defer apiMu.Unlock()
	cfg = config.Normalize(cfg)
	if w := liveReader(); w != nil {
		text, err := prepareText(w, cfg)
		if err != nil {
			return err
		}
		return w.invoke(func() error {
			if err := w.applySettings(cfg, text); err != nil {
				return err
			}
			return w.show()
		})
	}
	if cfg.BookPath == "" {
		return errors.New("请先选择 TXT 小说")
	}
	text, err := book.Load(cfg.BookPath)
	if err != nil {
		return fmt.Errorf("读取小说失败: %w", err)
	}
	w := &Window{cfg: cfg, onSave: onSave, text: text, visible: true, registered: make(map[int]bool), commands: make(chan request, 16), done: make(chan struct{})}
	ready := make(chan error, 1)
	go w.run(ready)
	if err = <-ready; err != nil {
		<-w.done
		return err
	}
	activeReader = w
	return nil
}

func prepareText(w *Window, cfg config.Settings) (string, error) {
	var currentPath string
	if err := w.invoke(func() error { currentPath = w.cfg.BookPath; return nil }); err != nil {
		return "", err
	}
	if cfg.BookPath == currentPath {
		return "", nil
	}
	if cfg.BookPath == "" {
		return "", errors.New("请先选择 TXT 小说")
	}
	text, err := book.Load(cfg.BookPath)
	if err != nil {
		return "", fmt.Errorf("读取小说失败: %w", err)
	}
	return text, nil
}

func ApplySettings(cfg config.Settings) error {
	apiMu.Lock()
	defer apiMu.Unlock()
	w := liveReader()
	if w == nil {
		return errors.New("阅读条尚未启动")
	}
	cfg = config.Normalize(cfg)
	text, err := prepareText(w, cfg)
	if err != nil {
		return err
	}
	return w.invoke(func() error { return w.applySettings(cfg, text) })
}

func operate(optional bool, f func(*Window) error) error {
	apiMu.Lock()
	defer apiMu.Unlock()
	w := liveReader()
	if w == nil {
		if optional {
			return nil
		}
		return errors.New("阅读条尚未启动")
	}
	return w.invoke(func() error { return f(w) })
}
func Show() error { return operate(false, func(w *Window) error { return w.show() }) }
func Hide() error { return operate(false, func(w *Window) error { return w.hide() }) }
func Toggle() error {
	return operate(false, func(w *Window) error {
		if w.visible {
			return w.hide()
		}
		return w.show()
	})
}
func Flush() error { return operate(true, func(w *Window) error { return w.persist() }) }

// Seek 供重新定位文件等明确改变书签的操作使用，不写磁盘，便于上层事务回滚。
// 普通 ApplySettings 始终保留同一本书的当前首行，不受过期 UI 配置影响。
func Seek(positionBytes int) error {
	return operate(false, func(w *Window) error {
		w.previous, w.forward = nil, nil
		w.displayAt(byteBoundary(w.text, positionBytes))
		return nil
	})
}

func Close() error {
	apiMu.Lock()
	defer apiMu.Unlock()
	w := liveReader()
	if w == nil {
		return nil
	}
	err := w.invoke(func() error {
		if err := w.persist(); err != nil {
			return err
		}
		r, _, e := procDestroyWindow.Call(uintptr(w.hwnd))
		if r == 0 {
			return fmt.Errorf("关闭阅读条失败: %w", e)
		}
		return nil
	})
	if err == nil {
		<-w.done
		activeReader = nil
	}
	return err
}

func (w *Window) invoke(f func() error) error {
	req := request{run: f, result: make(chan error, 1)}
	select {
	case w.commands <- req:
	case <-w.done:
		return errors.New("阅读条已关闭")
	}
	r, _, e := procPostMessageW.Call(uintptr(w.hwnd), wmCommandReader, 0, 0)
	if r == 0 {
		return fmt.Errorf("阅读条命令发送失败: %w", e)
	}
	select {
	case err := <-req.result:
		return err
	case <-w.done:
		select {
		case err := <-req.result:
			return err
		default:
			return errors.New("阅读条已关闭")
		}
	}
}

func (w *Window) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)
	className, _ := windows.UTF16PtrFromString(fmt.Sprintf("MoyuReaderBarV4_%d", classSerial.Add(1)))
	inst, _, _ := procGetModuleHandleW()
	cur, _, _ := procLoadCursorW.Call(0, idcArrow)
	wc := wndclassex{WndProc: readerWindowCallback, Instance: inst, ClassName: className, Cursor: windows.Handle(cur)}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		ready <- fmt.Errorf("注册阅读窗口失败: %w", e)
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), uintptr(inst))
	title, _ := windows.UTF16PtrFromString("")
	h, _, err := procCreateWindowExW.Call(wsExLayered|wsExToolWindow|wsExTopmost, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), wsPopup, uintptr(int32(w.cfg.WindowLeft)), uintptr(int32(w.cfg.WindowTop)), uintptr(w.cfg.WindowWidth), 80, 0, 0, uintptr(inst), 0)
	if h == 0 {
		ready <- fmt.Errorf("创建阅读条失败: %w", err)
		return
	}
	w.hwnd = windows.Handle(h)
	windowLookup.Store(w.hwnd, w)
	defer func() {
		w.unregisterHotkeys()
		// 确保子控件释放所选字体后再销毁 GDI 对象。
		procDestroyWindow.Call(h)
		windowLookup.Delete(w.hwnd)
		windowLookup.Delete(w.editHwnd)
		// 启动失败尚未进入消息循环，清除 WM_QUIT 防止下一次复用此线程时提前退出。
		var leftover msg
		procPeekMessageW.Call(uintptr(unsafe.Pointer(&leftover)), 0, 0x0012, 0x0012, 1)
		if w.fontHandle != 0 {
			procDeleteObject.Call(uintptr(w.fontHandle))
		}
		if w.brush != 0 {
			procDeleteObject.Call(w.brush)
		}
	}()
	font, err := createFont(w.cfg)
	if err != nil {
		ready <- err
		return
	}
	w.fontHandle = font
	if err = w.createEditHost(); err != nil {
		ready <- err
		return
	}
	w.applyFontAndLayout()
	w.updateTransparency()
	w.position = w.initialPosition(w.cfg)
	w.displayAt(w.position)
	if err = w.registerHotkeys(w.cfg, true); err != nil {
		ready <- err
		return
	}
	w.ensureWindowVisible()
	ready <- nil
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (w *Window) applySettings(cfg config.Settings, text string) error {
	old := w.cfg
	changed := cfg.BookPath != old.BookPath
	font, err := createFont(cfg)
	if err != nil {
		return err
	}
	w.unregisterHotkeys()
	if err = w.registerHotkeys(cfg, w.visible); err != nil {
		procDeleteObject.Call(uintptr(font))
		rollback := w.registerHotkeys(old, w.visible)
		if rollback != nil {
			return errors.Join(err, fmt.Errorf("恢复原快捷键失败: %w", rollback))
		}
		return err
	}
	w.syncPositionFromView()
	pos := w.position
	w.cfg = cfg
	if changed {
		w.text = text
		pos = w.initialPosition(cfg)
	}
	oldFont := w.fontHandle
	w.fontHandle = font
	w.applyFontAndLayout()
	if oldFont != 0 {
		procDeleteObject.Call(uintptr(oldFont))
	}
	w.updateTransparency()
	w.previous = nil
	w.forward = nil
	w.displayAt(pos)
	w.clampWindowOnScreen()
	return nil
}

func (w *Window) initialPosition(cfg config.Settings) int {
	pos := cfg.PositionBytes
	if cfg.SchemaVersion < 2 {
		pos = migrateLegacyPosition(w.text, cfg.CharIndex, cfg.LineIndex, cfg.FontSize, 480)
	}
	return byteBoundary(w.text, pos)
}

func (w *Window) wndProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCommandReader:
		for {
			select {
			case req := <-w.commands:
				req.result <- req.run()
			default:
				return 0
			}
		}
	case wmHotkey:
		if err := w.handleHotkey(int(wParam)); err != nil {
			showReaderError(err.Error())
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	case wmClose:
		if err := w.hide(); err != nil {
			showReaderError(err.Error())
		}
		return 0
	case wmExitSizeMove:
		w.clampWindowOnScreen()
		if err := w.persist(); err != nil {
			showReaderError(err.Error())
		}
		return 0
	case 0x007e, 0x02e0: // WM_DISPLAYCHANGE / WM_DPICHANGED
		w.clampWindowOnScreen()
		return 0
	case 0x0005:
		w.resizeEdit()
	case wmCtlColorEdit, wmCtlColorStatic:
		return w.handleCtlColorEdit(wParam)
	case wmEraseBkgnd:
		if w.brush != 0 {
			var rc rect
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			procFillRect.Call(wParam, uintptr(unsafe.Pointer(&rc)), w.brush)
		}
		return 1
	case wmLButtonDown:
		procReleaseCapture.Call()
		procSendMessageW.Call(uintptr(hwnd), wmNCLButtonDown, htCaption, 0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return r
}

func procGetModuleHandleW() (windows.Handle, uintptr, error) {
	r, _, e := kernel32.NewProc("GetModuleHandleW").Call(0)
	return windows.Handle(r), r, e
}

func (w *Window) savePosition() {
	var r rect
	if ok, _, _ := procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r))); ok != 0 {
		w.cfg.WindowLeft = float64(r.Left)
		w.cfg.WindowTop = float64(r.Top)
	}
}
func (w *Window) persist() error {
	w.syncPositionFromView()
	w.savePosition()
	if err := config.SaveProgress(w.cfg.BookPath, w.position, w.cfg.WindowLeft, w.cfg.WindowTop); err != nil {
		return fmt.Errorf("保存阅读进度失败: %w", err)
	}
	w.cfg.PositionBytes = w.position
	w.cfg.SchemaVersion = 2
	w.cfg.CharIndex = 0
	w.cfg.LineIndex = 0
	if w.onSave != nil {
		cfg := w.cfg
		go w.onSave(cfg)
	}
	return nil
}

func (w *Window) hotkeys(cfg config.Settings) []struct {
	id          int
	name, value string
} {
	return []struct {
		id          int
		name, value string
	}{
		{idHotkeyUp, "上一页", cfg.Hotkeys.Up}, {idHotkeyDown, "下一页", cfg.Hotkeys.Down}, {idHotkeyHide, "隐藏", cfg.Hotkeys.Hide}, {idHotkeyShow, "显示", cfg.Hotkeys.Show}, {idHotkeyMove, "重置位置", cfg.Hotkeys.Move},
	}
}
func (w *Window) registerHotkeys(cfg config.Settings, visible bool) error {
	if err := hotkey.Validate(cfg.Hotkeys.Map()); err != nil {
		return err
	}
	for _, k := range w.hotkeys(cfg) {
		if !visible && (k.id == idHotkeyUp || k.id == idHotkeyDown) {
			continue
		}
		b, err := hotkey.Parse(k.value)
		if err != nil {
			w.unregisterHotkeys()
			return fmt.Errorf("%s快捷键: %w", k.name, err)
		}
		if r, _, e := procRegisterHotKey.Call(uintptr(w.hwnd), uintptr(k.id), uintptr(b.Modifiers|0x4000), uintptr(b.Key)); r == 0 {
			w.unregisterHotkeys()
			return fmt.Errorf("%s快捷键 %s 被其他程序占用或无法注册: %w", k.name, b.Label, e)
		}
		w.registered[k.id] = true
	}
	return nil
}
func (w *Window) unregisterHotkeys() {
	for id := range w.registered {
		procUnregisterHotKey.Call(uintptr(w.hwnd), uintptr(id))
		delete(w.registered, id)
	}
}
func (w *Window) show() error {
	if !w.visible {
		// 只重新获取翻页热键；显示、隐藏等入口保持注册。
		added := []int{}
		for _, k := range w.hotkeys(w.cfg) {
			if k.id != idHotkeyUp && k.id != idHotkeyDown {
				continue
			}
			b, err := hotkey.Parse(k.value)
			if err != nil {
				return err
			}
			r, _, e := procRegisterHotKey.Call(uintptr(w.hwnd), uintptr(k.id), uintptr(b.Modifiers|0x4000), uintptr(b.Key))
			if r == 0 {
				for _, id := range added {
					procUnregisterHotKey.Call(uintptr(w.hwnd), uintptr(id))
					delete(w.registered, id)
				}
				return fmt.Errorf("无法显示阅读条：%s快捷键 %s 被占用: %w", k.name, b.Label, e)
			}
			added = append(added, k.id)
			w.registered[k.id] = true
		}
	}
	w.visible = true
	w.ensureWindowVisible()
	return nil
}
func (w *Window) hide() error {
	err := w.persist()
	for _, id := range []int{idHotkeyUp, idHotkeyDown} {
		procUnregisterHotKey.Call(uintptr(w.hwnd), uintptr(id))
		delete(w.registered, id)
	}
	w.visible = false
	procShowWindow.Call(uintptr(w.hwnd), swHide)
	return err
}
func (w *Window) handleHotkey(id int) error {
	switch id {
	case idHotkeyHide:
		return w.hide()
	case idHotkeyShow:
		return w.show()
	case idHotkeyMove:
		procSetWindowPos.Call(uintptr(w.hwnd), 0, 100, 100, 0, 0, 0x0015)
		w.clampWindowOnScreen()
		return w.persist()
	case idHotkeyUp:
		if w.visible {
			w.pageUp()
			return w.persist()
		}
	case idHotkeyDown:
		if w.visible {
			w.pageDown()
			return w.persist()
		}
	}
	return nil
}
