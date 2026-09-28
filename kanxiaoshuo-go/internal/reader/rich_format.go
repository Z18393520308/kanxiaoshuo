//go:build windows

package reader

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	emSetCharFormat        = 0x0400 + 68
	emSetBkgndColor        = 0x0400 + 67
	emSetTextMode          = 0x0400 + 89
	emSetTypographyOptions = 0x0400 + 202
	emPosFromChar          = 0x0400 + 38
	cfmSize                = 0x80000000
	cfmFace                = 0x20000000
	cfmColor               = 0x40000000
	cfmSpacing             = 0x00200000
)

// CHARFORMATW/CHARFORMAT2W follow the Windows SDK C++ layout, including
// the base structure's two tail-padding bytes before wWeight.
type charFormatW struct {
	Size, Mask, Effects uint32
	Height, Offset      int32
	TextColor           uint32
	CharSet, Pitch      byte
	FaceName            [32]uint16
}

type charFormat2W struct {
	charFormatW
	Weight                                  uint16
	Spacing                                 int16
	BackColor, Locale, Reserved             uint32
	Style                                   int16
	Kerning                                 uint16
	Underline, Animation, Author, LineColor byte
}

type pointL struct{ X, Y int32 }

var richEditDLL = windows.NewLazySystemDLL("msftedit.dll")

func (w *Window) textFormat() charFormat2W {
	dpi := 96
	dc, _, _ := procGetDC.Call(uintptr(w.editHwnd))
	if dc != 0 {
		value, _, _ := gdi32.NewProc("GetDeviceCaps").Call(dc, 90) // LOGPIXELSY
		if value > 0 {
			dpi = int(value)
		}
		procReleaseDC.Call(uintptr(w.editHwnd), dc)
	}
	r, g, b := parseHexColor(w.cfg.FontColor)
	format := charFormat2W{}
	format.Size = uint32(unsafe.Sizeof(format))
	format.Mask = cfmSize | cfmFace | cfmColor | cfmSpacing
	format.Height = int32((w.cfg.FontSize*1440 + dpi/2) / dpi)
	format.Spacing = int16((w.cfg.LetterSpacing*1440 + dpi/2) / dpi)
	format.TextColor = bgr(r, g, b)
	face, _ := windows.UTF16FromString(w.cfg.FontFamily)
	copy(format.FaceName[:31], face)
	return format
}

func (w *Window) applyTextFormat() {
	format := w.textFormat()
	// 设置默认格式与现有全文格式，确保下一块文字继承相同字距。
	procSendMessageW.Call(uintptr(w.editHwnd), emSetCharFormat, 0, uintptr(unsafe.Pointer(&format)))
	procSendMessageW.Call(uintptr(w.editHwnd), emSetCharFormat, 4, uintptr(unsafe.Pointer(&format))) // SCF_ALL
}

func (w *Window) characterPoint(index int) pointL {
	var point pointL
	procSendMessageW.Call(uintptr(w.editHwnd), emPosFromChar, uintptr(unsafe.Pointer(&point)), uintptr(index))
	return point
}
