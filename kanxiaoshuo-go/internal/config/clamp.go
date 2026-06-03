package config

// ReaderWidth 阅读条宽度（像素），与 reader 包 defaultReaderWidth 保持一致
const ReaderWidth = 480

// MaxFontSize 根据可见行数给出推荐最大字号，避免单行被裁切
func MaxFontSize(visibleLines int) int {
	if visibleLines < 1 {
		visibleLines = 1
	}
	if visibleLines > 15 {
		visibleLines = 15
	}
	// 行数越多，允许的最大字号越小
	max := 28 - visibleLines*2
	if max > 24 {
		max = 24
	}
	if max < 6 {
		max = 6
	}
	return max
}

// Normalize 校验并修正配置（字号、行数等）
func Normalize(s Settings) Settings {
	if s.VisibleLines < 1 {
		s.VisibleLines = 1
	}
	if s.VisibleLines > 15 {
		s.VisibleLines = 15
	}
	if s.FontSize < 3 {
		s.FontSize = 3
	}
	max := MaxFontSize(s.VisibleLines)
	if s.FontSize > max {
		s.FontSize = max
	}
	if s.FontColor == "" {
		s.FontColor = "#000000"
	}
	return s
}
