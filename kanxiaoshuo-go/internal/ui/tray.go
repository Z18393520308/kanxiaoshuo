//go:build windows

package ui

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"kanxiaoshuo-go/internal/reader"
)

const (
	trayMessage = 0x8001 // 避开 go-webview2 以 WM_APP（0x8000）处理的 Dispatch 队列。
	trayID      = 1
	trayShow    = 101
	trayHide    = 102
	trayOptions = 103
	trayBook    = 104
	trayExit    = 105
)

var (
	comctl32        = windows.NewLazySystemDLL("comctl32.dll")
	shell32         = windows.NewLazySystemDLL("shell32.dll")
	setSubclass     = comctl32.NewProc("SetWindowSubclass")
	removeSubclass  = comctl32.NewProc("RemoveWindowSubclass")
	defSubclassProc = comctl32.NewProc("DefSubclassProc")
	shellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
)

// notifyIconData 使用 uintptr 和 Windows 原生对齐；不能将 HANDLE 简写成 uint32。
type notifyIconData struct {
	Size            uint32
	Window          uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

type trayIcon struct {
	app            *webApp
	callback       uintptr
	taskbarCreated uintptr
	icon           uintptr
	added          bool
	disposed       bool
}

// newTrayIcon 在设置窗口线程安装子类，并将未处理的消息交回 WebView2 原窗口过程。
func newTrayIcon(app *webApp) (*trayIcon, error) {
	t := &trayIcon{app: app}
	name, _ := windows.UTF16PtrFromString("TaskbarCreated")
	t.taskbarCreated, _, _ = user32.NewProc("RegisterWindowMessageW").Call(uintptr(unsafe.Pointer(name)))
	t.icon, _, _ = user32.NewProc("LoadIconW").Call(0, 32512) // IDI_APPLICATION，共享图标无需 DestroyIcon。
	t.callback = syscall.NewCallback(t.windowProc)
	r, _, err := setSubclass.Call(app.handle, t.callback, trayID, 0)
	if r == 0 {
		return nil, fmt.Errorf("无法初始化系统托盘窗口：%v", err)
	}
	if err := t.add(); err != nil {
		removeSubclass.Call(app.handle, t.callback, trayID)
		return nil, err
	}
	return t, nil
}

func (t *trayIcon) data() notifyIconData {
	d := notifyIconData{
		Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: t.app.handle, ID: trayID,
		Flags: 0x1 | 0x2 | 0x4, CallbackMessage: trayMessage, Icon: t.icon,
	}
	tip, _ := windows.UTF16FromString("摸鱼联盟 · 双击恢复阅读，右键打开菜单")
	copy(d.Tip[:], tip)
	return d
}

func (t *trayIcon) add() error {
	d := t.data()
	r, _, err := shellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&d))) // NIM_ADD
	if r == 0 {
		return fmt.Errorf("系统托盘图标创建失败：%v", err)
	}
	t.added = true
	return nil
}

func (t *trayIcon) dispose() {
	if t.disposed {
		return
	}
	t.disposed = true
	if t.added {
		d := t.data()
		shellNotifyIcon.Call(2, uintptr(unsafe.Pointer(&d))) // NIM_DELETE
		t.added = false
	}
	removeSubclass.Call(t.app.handle, t.callback, trayID)
}

func (t *trayIcon) windowProc(hwnd, msg, wParam, lParam, subclassID, refData uintptr) uintptr {
	if t.taskbarCreated != 0 && msg == t.taskbarCreated {
		t.added = false // Explorer 重启会清空托盘。失败时主动恢复设置入口。
		if err := t.add(); err != nil {
			t.app.showSettings(err.Error(), false)
		}
		return 0
	}
	switch msg {
	case trayMessage:
		switch lParam & 0xffff {
		case 0x0203, 0x0400, 0x0401: // WM_LBUTTONDBLCLK / NIN_SELECT / NIN_KEYSELECT。
			t.action(trayShow)
		case 0x0205, 0x007B: // WM_RBUTTONUP / WM_CONTEXTMENU。
			t.menu()
		}
		return 0
	case 0x0010: // WM_CLOSE：读书时关闭设置仅隐藏；明确退出走刷盘关闭流程。
		if !t.app.quitting.Load() {
			go t.app.closeSettings()
		}
		return 0
	case 0x0011: // WM_QUERYENDSESSION：关机前尽量刷盘，不能留到消息循环被终止后。
		_ = reader.Flush()
		return 1
	case 0x0082: // WM_NCDESTROY
		t.dispose()
	}
	r, _, _ := defSubclassProc.Call(hwnd, msg, wParam, lParam)
	return r
}

func (t *trayIcon) menu() {
	menu, _, _ := user32.NewProc("CreatePopupMenu").Call()
	if menu == 0 {
		return
	}
	defer user32.NewProc("DestroyMenu").Call(menu)
	items := []struct {
		id   uintptr
		text string
	}{
		{trayShow, "显示阅读条"}, {trayHide, "隐藏阅读条"},
		{0, ""}, {trayOptions, "打开设置"}, {trayBook, "更换书籍"},
		{0, ""}, {trayExit, "退出摸鱼联盟"},
	}
	for _, item := range items {
		if item.id == 0 {
			user32.NewProc("AppendMenuW").Call(menu, 0x0800, 0, 0) // MF_SEPARATOR
			continue
		}
		text, _ := windows.UTF16PtrFromString(item.text)
		user32.NewProc("AppendMenuW").Call(menu, 0, item.id, uintptr(unsafe.Pointer(text)))
	}
	var point struct{ X, Y int32 }
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	user32.NewProc("SetForegroundWindow").Call(t.app.handle)
	command, _, _ := user32.NewProc("TrackPopupMenu").Call(menu, 0x0100|0x0002,
		uintptr(point.X), uintptr(point.Y), 0, t.app.handle, 0)
	// 按 Win32 约定投递空消息，确保点击菜单外部能正常关闭菜单。
	user32.NewProc("PostMessageW").Call(t.app.handle, 0, 0, 0)
	if command != 0 {
		t.action(command)
	}
}

func (t *trayIcon) action(command uintptr) {
	if t.app.quitting.Load() {
		return
	}
	switch command {
	case trayOptions:
		t.app.showSettings("", false)
	case trayBook:
		t.app.showSettings("", true)
	case trayExit:
		t.app.requestExit()
	default:
		go func() {
			t.app.actionMu.Lock()
			defer t.app.actionMu.Unlock()
			var err error
			if command == trayShow {
				err = t.app.showReader()
			} else if command == trayHide && t.app.started {
				err = reader.Hide()
			}
			if err != nil {
				t.app.showSettings(err.Error(), false)
			}
		}()
	}
}
