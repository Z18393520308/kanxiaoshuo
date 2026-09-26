package reader

import "unicode/utf8"

const chunkBytes = 64 * 1024

// byteBoundary 向前夹到完整 UTF-8 字符边界，同时避免从 CRLF 中间续读。
func byteBoundary(text string, pos int) int {
	if pos < 0 {
		return 0
	}
	if pos > len(text) {
		pos = len(text)
	}
	for pos > 0 && pos < len(text) && !utf8.RuneStart(text[pos]) {
		pos--
	}
	if pos > 0 && pos < len(text) && text[pos] == '\n' && text[pos-1] == '\r' {
		pos--
	}
	return pos
}

// utf16OffsetToBytes 将 EDIT 的 UTF-16 code unit 下标转换为 UTF-8 字节偏移。
// 若传入位置处于代理项中间，向前对齐到完整字符。
func utf16OffsetToBytes(text string, units int) int {
	if units <= 0 {
		return 0
	}
	used := 0
	for i, r := range text {
		n := 1
		if r > 0xffff {
			n = 2
		}
		if used+n > units {
			return i
		}
		used += n
		if used == units {
			return i + utf8.RuneLen(r)
		}
	}
	return len(text)
}

func utf16Length(text string) int {
	n := 0
	for _, r := range text {
		n++
		if r > 0xffff {
			n++
		}
	}
	return n
}

// migrateLegacyPosition 旧版会删掉换行：小书按 rune、大于 3 MB 按字节计数。
// 只能迁移旧版保存的近似位置，无法恢复旧版本身估算时已丢失的信息。
func migrateLegacyPosition(text string, oldIndex, oldLine, fontSize, width int) int {
	if oldIndex <= 0 && oldLine > 0 {
		oldIndex = oldLine * max(8, width/(fontSize+2))
	}
	if oldIndex <= 0 {
		return 0
	}
	strippedBytes := 0
	for _, r := range text {
		if r != '\r' && r != '\n' {
			strippedBytes += utf8.RuneLen(r)
		}
	}
	big := strippedBytes > 3_000_000
	count := 0
	for i, r := range text {
		if r == '\r' || r == '\n' {
			continue
		}
		if count >= oldIndex {
			return i
		}
		step := 1
		if big {
			step = utf8.RuneLen(r)
		}
		if count+step > oldIndex {
			return i
		}
		count += step
	}
	return len(text)
}

// clampRect 可处理位于主屏左侧的负坐标显示器。
func clampRect(left, top, width, height, workLeft, workTop, workRight, workBottom int32) (int32, int32) {
	maxLeft := max(workLeft, workRight-width)
	maxTop := max(workTop, workBottom-height)
	return min(max(left, workLeft), maxLeft), min(max(top, workTop), maxTop)
}
