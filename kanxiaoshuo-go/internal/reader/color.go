//go:build windows

package reader

import "strconv"

func parseHexColor(hex string) (r, g, b byte) {
	if len(hex) == 7 && hex[0] == '#' {
		rv, _ := strconv.ParseUint(hex[1:3], 16, 8)
		gv, _ := strconv.ParseUint(hex[3:5], 16, 8)
		bv, _ := strconv.ParseUint(hex[5:7], 16, 8)
		return byte(rv), byte(gv), byte(bv)
	}
	return 0, 0, 0
}

func bgr(r, g, b byte) uint32 {
	return uint32(b)<<16 | uint32(g)<<8 | uint32(r)
}

// layeredTextBGR 分层透明窗绘制用色：纯黑 #000000 的 RGB 为 0，与透明底相同会导致整段文字不可见
func layeredTextBGR(hex string) uintptr {
	r, g, b := parseHexColor(hex)
	if r == 0 && g == 0 && b == 0 {
		b = 1
	}
	return uintptr(bgr(r, g, b))
}
