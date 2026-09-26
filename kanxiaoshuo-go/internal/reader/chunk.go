//go:build windows

package reader

const (
	emGetLineCount        = 0x00BA
	emLineIndex           = 0x00BB
	emGetFirstVisibleLine = 0x00CE
)

func (w *Window) visibleLines() int { return max(1, min(15, w.cfg.VisibleLines)) }

// displayAt 每次仅装入一段完整 UTF-8 文本；分页位置由同一个真实 EDIT 的排版返回。
// 不再使用 DrawText 估算行数，也不混用 rune 数和字节数。
func (w *Window) displayAt(pos int) {
	w.position = byteBoundary(w.text, pos)
	w.chunkStart = w.position
	w.chunkEnd = byteBoundary(w.text, min(len(w.text), w.position+chunkBytes))
	w.chunkText = w.text[w.chunkStart:w.chunkEnd]
	if w.chunkText == "" {
		w.setEditText("（已到书末）")
	} else {
		w.setEditText(w.chunkText)
		// 极端文本（大量零宽字符等）可能 64 KB 仍不足一屏，继续扩展，
		// 直到存在下一屏或真正抵达全文末尾，不能将分块末尾误判成书末。
		for w.chunkEnd < len(w.text) && w.lineByteOffset(w.visibleLines()) < 0 {
			w.chunkEnd = byteBoundary(w.text, min(len(w.text), w.chunkEnd+chunkBytes))
			w.chunkText = w.text[w.chunkStart:w.chunkEnd]
			w.setEditText(w.chunkText)
		}
	}
}
func (w *Window) lineByteOffset(line int) int {
	r, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emLineIndex, uintptr(line), 0)
	if int32(r) < 0 {
		return -1
	}
	return utf16OffsetToBytes(w.chunkText, int(r))
}
func (w *Window) syncPositionFromView() {
	if w.chunkText == "" {
		return
	}
	first, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emGetFirstVisibleLine, 0, 0)
	if off := w.lineByteOffset(int(first)); off >= 0 {
		w.position = byteBoundary(w.text, w.chunkStart+off)
	}
}
func (w *Window) pageDown() {
	if n := len(w.forward); n > 0 {
		pos := w.forward[n-1]
		w.forward = w.forward[:n-1]
		w.previous = append(w.previous, w.position)
		w.displayAt(pos)
		return
	}
	if w.position >= len(w.text) {
		return
	}
	first, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emGetFirstVisibleLine, 0, 0)
	offset := w.lineByteOffset(int(first) + w.visibleLines())
	if offset < 0 || offset == 0 {
		return
	} // 已显示最后一页，不再原地反复夹到块内末页。
	next := byteBoundary(w.text, w.chunkStart+offset)
	if next <= w.position || next >= len(w.text) {
		return
	}
	w.previous = append(w.previous, w.position)
	w.displayAt(next)
}
func (w *Window) pageUp() {
	if w.position <= 0 {
		return
	}
	w.forward = append(w.forward, w.position)
	if n := len(w.previous); n > 0 {
		pos := w.previous[n-1]
		w.previous = w.previous[:n-1]
		w.displayAt(pos)
		return
	}
	// 刚续读或重新排版后没有本次翻页历史，在当前起点之前取一段上下文，
	// 使用 EDIT 的实际折行找到紧邻当前页的上一屏，保证边界不丢字。
	end := w.position
	start := byteBoundary(w.text, max(0, end-chunkBytes))
	previous := w.text[start:end]
	w.setEditText(previous)
	count, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emGetLineCount, 0, 0)
	lines := int(count)
	// 末尾 CRLF 产生的空行不属于上一屏内容。
	if lines > 1 {
		last, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emLineIndex, uintptr(lines-1), 0)
		if int(last) == utf16Length(previous) {
			lines--
		}
	}
	line := max(0, lines-w.visibleLines())
	offset, _, _ := procSendMessageW.Call(uintptr(w.editHwnd), emLineIndex, uintptr(line), 0)
	pos := start
	if int32(offset) >= 0 {
		pos += utf16OffsetToBytes(previous, int(offset))
	}
	if pos >= end {
		pos = start
	}
	w.displayAt(pos)
}
