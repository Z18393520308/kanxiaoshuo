//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// PickFile 打开 TXT 文件选择对话框
func PickFile() string {
	comdlg32 := windows.NewLazySystemDLL("comdlg32.dll")
	getOpenFileName := comdlg32.NewProc("GetOpenFileNameW")

	buf := make([]uint16, 512)
	filter, _ := windows.UTF16PtrFromString("文本文件\000*.txt\000")
	title, _ := windows.UTF16PtrFromString("选择小说 TXT")

	type ofn struct {
		StructSize      uint32
		Owner           windows.Handle
		Instance        windows.Handle
		Filter          *uint16
		CustomFilter    *uint16
		MaxCustomFilter uint32
		FilterIndex     uint32
		File            *uint16
		MaxFile         uint32
		FileTitle       *uint16
		MaxFileTitle    uint32
		InitialDir      *uint16
		Title           *uint16
		Flags           uint32
		FileOffset      uint16
		FileExtension   uint16
		DefExt          *uint16
		CustData        uintptr
		Hook            uintptr
		TemplateName    *uint16
	}

	o := ofn{
		StructSize: uint32(unsafe.Sizeof(ofn{})),
		Filter:     filter,
		File:       &buf[0],
		MaxFile:    uint32(len(buf)),
		Title:      title,
		Flags:      0x00080000 | 0x00001000,
	}
	r, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&o)))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf)
}
