//go:build windows

package reader

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

func showReaderError(message string) {
	text, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString("摸鱼联盟 · 阅读条")
	user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
