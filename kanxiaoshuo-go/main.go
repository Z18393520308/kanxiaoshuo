//go:build windows

package main

import (
	"errors"
	"os"
	"unsafe"

	"kanxiaoshuo-go/internal/ui"

	"golang.org/x/sys/windows"
)

func main() {
	// 同一登录会话只启动一个实例，避免热键竞争和多个进程互相覆盖书签。
	name, _ := windows.UTF16PtrFromString(`Local\MoyuLeague.Reader.Go`)
	handle, err := windows.CreateMutex(nil, false, name)
	if handle != 0 {
		defer windows.CloseHandle(handle)
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		showMessage("摸鱼联盟已经运行，请从任务栏通知区域的托盘图标打开设置。", 0x40)
		return
	}
	if err != nil {
		showError("无法检查程序运行状态：" + err.Error())
		return
	}
	if err := ui.Run(); err != nil {
		showError(err.Error())
		os.Exit(1)
	}
}

func showError(msg string) {
	showMessage(msg, 0x10)
}

func showMessage(msg string, flags uintptr) {
	u := windows.NewLazySystemDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	t, _ := windows.UTF16PtrFromString(msg)
	c, _ := windows.UTF16PtrFromString("摸鱼联盟")
	_, _, _ = p.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), flags)
}
