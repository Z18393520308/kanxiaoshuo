//go:build windows

package reader

import "unsafe"

// fillReadingBackdrop 阅读条铺不透明浅底（分层窗必须带 alpha，否则整窗不可见）
func fillReadingBackdrop(bits unsafe.Pointer, width, height int) {
	s := unsafe.Slice((*uint32)(bits), width*height)
	const panel = 0xFFF2F2F2 // ARGB 浅灰
	for i := range s {
		s[i] = panel
	}
}
