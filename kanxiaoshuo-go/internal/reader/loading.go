//go:build windows

package reader

import (
	"fmt"
	"math"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	timerLoadAnim = 1

	wmLoadProgress  = 0x0400 + 102
	wmLoadDone      = 0x0400 + 103
	wmReaderInit    = 0x0400 + 105
	wmApplySettings = 0x0400 + 106
	wmPrefetchNext  = 0x0400 + 107
	wmPrefetchPrev  = 0x0400 + 108
	wmDeferPrefetch = 0x0400 + 109

	loadBarW = 200
	loadBarH = 6
)

var (
	procSetTimer        = user32.NewProc("SetTimer")
	procKillTimer       = user32.NewProc("KillTimer")
	procCreatePen       = gdi32.NewProc("CreatePen")
	procMoveToEx        = gdi32.NewProc("MoveToEx")
	procLineTo          = gdi32.NewProc("LineTo")
	procRoundRect       = gdi32.NewProc("RoundRect")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procFillRect         = user32.NewProc("FillRect")
)

// fillLoadingBackdrop 整窗铺浅色底，避免透明层上进度条看不见
func fillLoadingBackdrop(bits unsafe.Pointer, width, height int) {
	s := unsafe.Slice((*uint32)(bits), width*height)
	const panel = 0xFFF8F8F8 // ARGB
	for i := range s {
		s[i] = panel
	}
}

func (w *Window) beginLoading(status string, progress int) {
	w.loading = true
	w.loadStatus = status
	w.loadProgress = progress
	w.loadAnim = 0
	w.startLoadTimer()
}

func (w *Window) endLoading() {
	w.loading = false
	w.loadProgress = 0
	w.loadStatus = ""
	w.stopLoadTimer()
}

func (w *Window) startLoadTimer() {
	if w.hwnd == 0 {
		return
	}
	procSetTimer.Call(uintptr(w.hwnd), timerLoadAnim, 40, 0)
}

func (w *Window) stopLoadTimer() {
	if w.hwnd == 0 {
		return
	}
	procKillTimer.Call(uintptr(w.hwnd), timerLoadAnim)
}

func (w *Window) drawLoading(hdcMem uintptr, width, height int) {
	cx := width / 2
	cy := height / 2

	w.drawLoadingText(hdcMem, cx, cy-36, w.loadStatus)
	if w.loadProgress >= 0 {
		pct := fmt.Sprintf("%d%%", w.loadProgress)
		w.drawLoadingText(hdcMem, cx, cy-18, pct)
	}
	w.drawSpinner(hdcMem, cx, cy+4, 12)

	barX := cx - loadBarW/2
	barY := height - 16
	if barY < 6 {
		barY = 6
	}
	w.drawProgressBar(hdcMem, barX, barY, loadBarW, loadBarH)
}

func (w *Window) drawLoadingText(hdcMem uintptr, cx, cy int, text string) {
	if text == "" {
		text = "加载中..."
	}
	loadFontH := int32(-12)
	h, _, _ := procCreateFontW.Call(
		uintptr(loadFontH), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(mustUTF16("Microsoft YaHei UI"))),
	)
	if h != 0 {
		procSelectObject.Call(hdcMem, h)
		defer procDeleteObject.Call(h)
	}
	procSetTextColor.Call(hdcMem, 0x00222222)
	procSetBkMode.Call(hdcMem, 1)
	ptr, _ := windows.UTF16PtrFromString(text)
	rc := rect{int32(cx - 120), int32(cy - 8), int32(cx + 120), int32(cy + 16)}
	const dtCenter = 0x00000001
	procDrawTextW.Call(hdcMem, uintptr(unsafe.Pointer(ptr)), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), dtCenter|dtTop|dtNoPrefix)
}

func mustUTF16(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func (w *Window) drawSpinner(hdcMem uintptr, cx, cy, r int) {
	pen, _, _ := procCreatePen.Call(0, 2, 0x00333333)
	if pen == 0 {
		return
	}
	defer procDeleteObject.Call(pen)
	procSelectObject.Call(hdcMem, pen)

	n := 8
	phase := w.loadAnim % n
	for i := 0; i < n; i++ {
		alpha := float64((i+phase)%n) / float64(n-1)
		if alpha < 0.15 {
			alpha = 0.15
		}
		angle := float64(i) * 2 * math.Pi / float64(n)
		x1 := cx + int(math.Cos(angle)*float64(r-3))
		y1 := cy + int(math.Sin(angle)*float64(r-3))
		x2 := cx + int(math.Cos(angle)*float64(r))
		y2 := cy + int(math.Sin(angle)*float64(r))
		procMoveToEx.Call(hdcMem, uintptr(x1), uintptr(y1), 0)
		procLineTo.Call(hdcMem, uintptr(x2), uintptr(y2))
		_ = alpha // 简化：统一颜色
	}
}

func (w *Window) drawProgressBar(hdcMem uintptr, x, y, barW, barH int) {
	track := rect{int32(x), int32(y), int32(x + barW), int32(y + barH)}
	brushTrack, _, _ := procCreateSolidBrush.Call(0x00D0D0D0)
	if brushTrack != 0 {
		procSelectObject.Call(hdcMem, brushTrack)
		procRoundRect.Call(hdcMem, uintptr(track.Left), uintptr(track.Top), uintptr(track.Right), uintptr(track.Bottom), 4, 4)
		procDeleteObject.Call(brushTrack)
	}

	fillW := barW
	if w.loadProgress >= 0 {
		fillW = barW * w.loadProgress / 100
		if fillW < 4 && w.loadProgress > 0 {
			fillW = 4
		}
	} else {
		// 不确定进度：跑马灯
		seg := barW / 3
		off := (w.loadAnim * 8) % (barW + seg)
		fill := rect{int32(x + off - seg), int32(y), int32(x + off), int32(y + barH)}
		if fill.Left < int32(x) {
			fill.Left = int32(x)
		}
		if fill.Right > int32(x+barW) {
			fill.Right = int32(x + barW)
		}
		if fill.Right > fill.Left {
			brushFill, _, _ := procCreateSolidBrush.Call(0x00E06A00) // 蓝 BGR
			if brushFill != 0 {
				procSelectObject.Call(hdcMem, brushFill)
				procRoundRect.Call(hdcMem, uintptr(fill.Left), uintptr(fill.Top), uintptr(fill.Right), uintptr(fill.Bottom), 4, 4)
				procDeleteObject.Call(brushFill)
			}
		}
		return
	}

	if fillW > 0 {
		fill := rect{int32(x), int32(y), int32(x + fillW), int32(y + barH)}
		brushFill, _, _ := procCreateSolidBrush.Call(0x00E06A00)
		if brushFill != 0 {
			procSelectObject.Call(hdcMem, brushFill)
			procRoundRect.Call(hdcMem, uintptr(fill.Left), uintptr(fill.Top), uintptr(fill.Right), uintptr(fill.Bottom), 4, 4)
			procDeleteObject.Call(brushFill)
		}
	}
}
