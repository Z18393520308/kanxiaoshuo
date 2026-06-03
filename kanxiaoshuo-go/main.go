//go:build windows

package main

import (
	"os"
	"unsafe"

	"kanxiaoshuo-go/internal/ui"

	"golang.org/x/sys/windows"
)

func main() {
	if err := ui.Run(); err != nil {
		showError(err.Error())
		os.Exit(1)
	}
}

func showError(msg string) {
	u := windows.NewLazySystemDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	t, _ := windows.UTF16PtrFromString(msg)
	c, _ := windows.UTF16PtrFromString("摸鱼联盟")
	_, _, _ = p.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x10) // MB_ICONERROR
}
