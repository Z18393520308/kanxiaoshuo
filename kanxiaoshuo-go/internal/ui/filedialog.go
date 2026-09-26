//go:build windows

package ui

import (
	"fmt"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PickFile 必须在 owner 所在的界面线程调用。取消选择时返回空路径、nil。
func PickFile(owner uintptr) (string, error) {
	comdlg32 := windows.NewLazySystemDLL("comdlg32.dll")
	buf := make([]uint16, 32768)
	// OPENFILENAME 的筛选器是以两个 NUL 结尾的多字符串，不能使用拒绝内嵌 NUL 的 UTF16PtrFromString。
	filter := utf16.Encode([]rune("文本文件 (*.txt)\x00*.txt\x00所有文件 (*.*)\x00*.*\x00\x00"))
	title, _ := windows.UTF16PtrFromString("选择小说 TXT")
	defExt, _ := windows.UTF16PtrFromString("txt")

	type openFileName struct {
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
		Reserved        uintptr
		ReservedValue   uint32
		FlagsEx         uint32
	}
	o := openFileName{
		StructSize:  uint32(unsafe.Sizeof(openFileName{})),
		Owner:       windows.Handle(owner),
		Filter:      &filter[0],
		FilterIndex: 1,
		File:        &buf[0],
		MaxFile:     uint32(len(buf)),
		Title:       title,
		DefExt:      defExt,
		// EXPLORER | FILEMUSTEXIST | PATHMUSTEXIST | NOCHANGEDIR
		Flags: 0x00080000 | 0x00001000 | 0x00000800 | 0x00000008,
	}
	r, _, _ := comdlg32.NewProc("GetOpenFileNameW").Call(uintptr(unsafe.Pointer(&o)))
	if r == 0 {
		code, _, _ := comdlg32.NewProc("CommDlgExtendedError").Call()
		if code != 0 {
			return "", fmt.Errorf("文件选择窗口打开失败（0x%04X）", code)
		}
		return "", nil
	}
	return windows.UTF16ToString(buf), nil
}
