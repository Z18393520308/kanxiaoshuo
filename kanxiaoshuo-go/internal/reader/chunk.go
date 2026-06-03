//go:build windows

package reader

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// 每次装入“阅读控件”的折行行数；滚到块末再取下一批
const linesPerChunk = 5

// 超过此大小不再转 []rune，按 UTF-8 字节分块（省内存、加快打开）
const bigTextThreshold = 3_000_000

type chunkSnapshot struct {
	start, end, lines int
	text              string
}

func (w *Window) visibleLines() int {
	n := w.cfg.VisibleLines
	if n < 1 {
		return 1
	}
	if n > 15 {
		return 15
	}
	return n
}

func (w *Window) avgCharsPerLine() int {
	avg := defaultReaderWidth / (w.cfg.FontSize + 2)
	if avg < 8 {
		return 8
	}
	return avg
}

func (w *Window) bookLen() int {
	if w.bigText {
		return len(w.text)
	}
	return len(w.textRunes)
}

func (w *Window) sliceBook(start, end int) string {
	if w.bigText {
		if start < 0 {
			start = 0
		}
		if end > len(w.text) {
			end = len(w.text)
		}
		if start >= end {
			return ""
		}
		return w.text[start:end]
	}
	if start < 0 {
		start = 0
	}
	if end > len(w.textRunes) {
		end = len(w.textRunes)
	}
	if start >= end {
		return ""
	}
	return string(w.textRunes[start:end])
}

func (w *Window) setBookRunes(text string) {
	if text == "" {
		w.textRunes = nil
		w.bigText = false
		w.text = ""
		w.chunkText = ""
		w.chunkLines = 0
		w.invalidatePrefetch()
		return
	}
	if len(text) > bigTextThreshold {
		w.bigText = true
		w.text = text
		w.textRunes = nil
	} else {
		w.bigText = false
		w.textRunes = []rune(text)
		w.text = ""
	}
	w.invalidatePrefetch()
}

func (w *Window) initCharIndexFromConfig() {
	w.charIndex = w.cfg.CharIndex
	if w.charIndex == 0 && w.cfg.LineIndex > 0 && w.bookLen() > 0 {
		w.charIndex = w.cfg.LineIndex * w.avgCharsPerLine()
	}
	if w.bookLen() > 0 && w.charIndex >= w.bookLen() {
		w.charIndex = 0
	}
	w.clampCharIndex()
	w.bufLineIndex = 0
	w.buildChunkAt(w.charIndex)
}

func (w *Window) clampCharIndex() {
	n := w.bookLen()
	if n == 0 {
		w.charIndex = 0
		return
	}
	if w.charIndex < 0 {
		w.charIndex = 0
	}
	if w.charIndex >= n {
		w.charIndex = n - 1
	}
}

func (w *Window) calcChunkSnapshot(startRune int) chunkSnapshot {
	var snap chunkSnapshot
	snap.lines = 1

	n := w.bookLen()
	if n == 0 {
		return snap
	}
	if startRune < 0 {
		startRune = 0
	}
	if startRune >= n {
		startRune = n - 1
	}

	avg := w.avgCharsPerLine()
	minEnd := startRune + 1
	maxEnd := startRune + avg*linesPerChunk*4
	if maxEnd > n {
		maxEnd = n
	}

	lo, hi := minEnd, maxEnd
	for lo < hi {
		mid := (lo + hi + 1) / 2
		lines := w.measureWrappedLines(w.sliceBook(startRune, mid))
		if lines <= linesPerChunk {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	end := lo
	if end <= startRune {
		end = startRune + avg
		if end > n {
			end = n
		}
	}

	text := w.sliceBook(startRune, end)
	lines := w.measureWrappedLines(text)
	if lines < 1 {
		lines = 1
	}

	snap.start = startRune
	snap.end = end
	snap.text = text
	snap.lines = lines
	return snap
}

func (w *Window) applyChunk(snap chunkSnapshot) {
	w.chunkStart = snap.start
	w.chunkEnd = snap.end
	w.chunkText = snap.text
	w.chunkLines = snap.lines
	if w.chunkLines < 1 {
		w.chunkLines = 1
	}
	w.clampBufLineIndex()
}

func (w *Window) buildChunkAt(startRune int) {
	if w.bookLen() == 0 {
		w.chunkStart = 0
		w.chunkEnd = 0
		w.chunkText = ""
		w.chunkLines = 1
		w.invalidatePrefetch()
		return
	}
	w.applyChunk(w.calcChunkSnapshot(startRune))
	w.invalidatePrefetch()
	// 预读延后到绘制之后，避免阻塞首次显示
	w.postUI(wmDeferPrefetch, 0, 0)
}

// calcChunkSnapshotEndingAt 计算以 endRune 为结尾、约 linesPerChunk 折行的上一块
func (w *Window) calcChunkSnapshotEndingAt(endRune int) chunkSnapshot {
	if endRune <= 0 || w.bookLen() == 0 {
		return w.calcChunkSnapshot(0)
	}
	if endRune > w.bookLen() {
		endRune = w.bookLen()
	}

	lo, hi := 0, endRune-1
	var best chunkSnapshot
	for lo <= hi {
		mid := (lo + hi) / 2
		snap := w.calcChunkSnapshot(mid)
		switch {
		case snap.end == endRune:
			return snap
		case snap.end < endRune:
			best = snap
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}
	if best.end > 0 {
		return best
	}
	est := endRune - w.avgCharsPerLine()*linesPerChunk*2
	if est < 0 {
		est = 0
	}
	return w.calcChunkSnapshot(est)
}

func (w *Window) invalidatePrefetch() {
	w.prefetchNextReady = false
	w.prefetchForEnd = -1
	w.prefetchPrevReady = false
	w.prefetchForStart = -1
}

func (w *Window) schedulePrefetchNext() {
	if w.hwnd == 0 || w.loading || w.chunkEnd >= w.bookLen() {
		return
	}
	if w.prefetchNextReady && w.prefetchForEnd == w.chunkEnd {
		return
	}
	w.postUI(wmPrefetchNext, 0, 0)
}

// doPrefetchNext 在 UI 线程预计算下一块（块末翻页时无需再等）
func (w *Window) doPrefetchNext() {
	if w.loading || w.bookLen() == 0 || w.chunkEnd >= w.bookLen() {
		w.invalidatePrefetch()
		return
	}
	if w.prefetchNextReady && w.prefetchForEnd == w.chunkEnd {
		return
	}
	snap := w.calcChunkSnapshot(w.chunkEnd)
	w.prefetchNext = snap
	w.prefetchForEnd = w.chunkEnd
	w.prefetchNextReady = true
}

func (w *Window) applyPrefetchNext() bool {
	if !w.prefetchNextReady || w.prefetchNext.start != w.chunkEnd {
		return false
	}
	w.applyChunk(w.prefetchNext)
	w.prefetchNextReady = false
	w.prefetchForEnd = -1
	w.schedulePrefetchNext()
	w.schedulePrefetchPrev()
	return true
}

func (w *Window) schedulePrefetchPrev() {
	if w.hwnd == 0 || w.loading || w.chunkStart <= 0 {
		return
	}
	if w.prefetchPrevReady && w.prefetchForStart == w.chunkStart {
		return
	}
	w.postUI(wmPrefetchPrev, 0, 0)
}

func (w *Window) doPrefetchPrev() {
	if w.loading || w.bookLen() == 0 || w.chunkStart <= 0 {
		w.prefetchPrevReady = false
		w.prefetchForStart = -1
		return
	}
	if w.prefetchPrevReady && w.prefetchForStart == w.chunkStart {
		return
	}
	snap := w.calcChunkSnapshotEndingAt(w.chunkStart)
	w.prefetchPrev = snap
	w.prefetchForStart = w.chunkStart
	w.prefetchPrevReady = true
}

func (w *Window) applyPrefetchPrev() bool {
	if !w.prefetchPrevReady || w.prefetchPrev.end != w.chunkStart {
		return false
	}
	w.applyChunk(w.prefetchPrev)
	w.prefetchPrevReady = false
	w.prefetchForStart = -1
	w.schedulePrefetchNext()
	w.schedulePrefetchPrev()
	return true
}

// maybeSchedulePrefetchSoon 接近块末时提前触发预读
func (w *Window) maybeSchedulePrefetchSoon() {
	if w.chunkEnd >= w.bookLen() {
		return
	}
	vis := w.visibleLines()
	lastScreen := w.chunkLines - vis
	if lastScreen < 0 {
		lastScreen = 0
	}
	if w.bufLineIndex >= lastScreen {
		w.schedulePrefetchNext()
	}
}

// maybeSchedulePrefetchPrevSoon 接近块首时提前预读上一块
func (w *Window) maybeSchedulePrefetchPrevSoon() {
	if w.chunkStart <= 0 {
		return
	}
	if w.bufLineIndex <= w.visibleLines() {
		w.schedulePrefetchPrev()
	}
}

func (w *Window) measureWrappedLines(text string) int {
	if text == "" {
		return 1
	}
	lh := w.lineHeightPx()
	if lh < 1 {
		lh = 16
	}

	hdc, _, _ := procGetDC.Call(uintptr(w.hwnd))
	if hdc == 0 {
		avg := w.avgCharsPerLine()
		r := len([]rune(text))
		n := r / avg
		if n < 1 {
			return 1
		}
		return n
	}
	defer procReleaseDC.Call(uintptr(w.hwnd), hdc)

	hdcMem, _, _ := procCreateCompatibleDC.Call(hdc)
	if hdcMem == 0 {
		return 1
	}
	defer procDeleteDC.Call(hdcMem)

	if w.fontHandle != 0 {
		procSelectObject.Call(hdcMem, uintptr(w.fontHandle))
	}
	procSetBkMode.Call(hdcMem, 1)

	ptr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return 1
	}
	rc := rect{0, 0, defaultReaderWidth - 8, 0}
	procDrawTextW.Call(
		hdcMem,
		uintptr(unsafe.Pointer(ptr)),
		^uintptr(0),
		uintptr(unsafe.Pointer(&rc)),
		dtLeft|dtWordBreak|dtNoPrefix|dtCalcrect,
	)
	totalH := int(rc.Bottom - rc.Top)
	if totalH < lh {
		return 1
	}
	return (totalH + lh - 1) / lh
}

func (w *Window) clampBufLineIndex() {
	vis := w.visibleLines()
	maxStart := w.chunkLines - vis
	if maxStart < 0 {
		maxStart = 0
	}
	if w.bufLineIndex < 0 {
		w.bufLineIndex = 0
	}
	if w.bufLineIndex > maxStart {
		w.bufLineIndex = maxStart
	}
}

func (w *Window) pageDown() {
	if w.bookLen() == 0 {
		return
	}
	vis := w.visibleLines()
	next := w.bufLineIndex + vis
	if next < w.chunkLines {
		w.bufLineIndex = next
		w.clampBufLineIndex()
		w.maybeSchedulePrefetchSoon()
		return
	}
	if w.chunkEnd >= w.bookLen() {
		w.bufLineIndex = max(0, w.chunkLines-vis)
		return
	}
	if w.applyPrefetchNext() {
		w.bufLineIndex = 0
		return
	}
	w.charIndex = w.chunkEnd
	w.bufLineIndex = 0
	w.buildChunkAt(w.charIndex)
}

func (w *Window) pageUp() {
	if w.bookLen() == 0 {
		return
	}
	vis := w.visibleLines()
	if w.bufLineIndex >= vis {
		w.bufLineIndex -= vis
		w.maybeSchedulePrefetchPrevSoon()
		return
	}
	if w.chunkStart <= 0 {
		w.bufLineIndex = 0
		return
	}
	if w.applyPrefetchPrev() {
		w.bufLineIndex = w.chunkLines - vis
		if w.bufLineIndex < 0 {
			w.bufLineIndex = 0
		}
		return
	}
	prevStart := w.chunkStart - w.avgCharsPerLine()*linesPerChunk
	if prevStart < 0 {
		prevStart = 0
	}
	w.charIndex = prevStart
	w.buildChunkAt(prevStart)
	w.bufLineIndex = w.chunkLines - vis
	if w.bufLineIndex < 0 {
		w.bufLineIndex = 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (w *Window) syncCharIndexFromView() {
	if w.bookLen() == 0 {
		return
	}
	off := w.bufLineIndex * w.avgCharsPerLine()
	w.charIndex = w.chunkStart + off
	w.clampCharIndex()
}
