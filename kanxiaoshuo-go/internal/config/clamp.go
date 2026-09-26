package config

import (
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"kanxiaoshuo-go/internal/hotkey"
)

const ReaderWidth = 480

// MaxFontSize 保留旧接口。窗口高度随行数增长，字号不再随行数被强制缩小。
func MaxFontSize(visibleLines int) int { return 48 }

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Normalize 只修正合法取值；不改变旧版位置格式，避免提前标记迁移完成。
func Normalize(s Settings) Settings {
	s.VisibleLines = clamp(s.VisibleLines, 1, 15)
	s.FontSize = clamp(s.FontSize, 3, 48)
	if s.WindowWidth == 0 {
		s.WindowWidth = ReaderWidth
	}
	s.WindowWidth = clamp(s.WindowWidth, 240, 1600)
	s.FontColor = strings.ToUpper(strings.TrimSpace(s.FontColor))
	if !colorPattern.MatchString(s.FontColor) {
		s.FontColor = "#000000"
	}
	s.FontFamily = strings.TrimSpace(s.FontFamily)
	if s.FontFamily == "" || strings.ContainsRune(s.FontFamily, '\x00') || utf8.RuneCountInString(s.FontFamily) > 64 {
		s.FontFamily = Default().FontFamily
	}
	if math.IsNaN(s.WindowLeft) || math.IsInf(s.WindowLeft, 0) {
		s.WindowLeft = Default().WindowLeft
	}
	if math.IsNaN(s.WindowTop) || math.IsInf(s.WindowTop, 0) {
		s.WindowTop = Default().WindowTop
	}
	if s.PositionBytes < 0 {
		s.PositionBytes = 0
	}
	if s.CharIndex < 0 {
		s.CharIndex = 0
	}
	if s.LineIndex < 0 {
		s.LineIndex = 0
	}
	d := Default().Hotkeys
	s.Hotkeys.Up = normalizeHotkey(s.Hotkeys.Up, d.Up)
	s.Hotkeys.Down = normalizeHotkey(s.Hotkeys.Down, d.Down)
	s.Hotkeys.Hide = normalizeHotkey(s.Hotkeys.Hide, d.Hide)
	s.Hotkeys.Show = normalizeHotkey(s.Hotkeys.Show, d.Show)
	s.Hotkeys.Move = normalizeHotkey(s.Hotkeys.Move, d.Move)
	s.RecentBooks = normalizeRecent(s.RecentBooks)
	return s
}

func normalizeHotkey(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	if binding, err := hotkey.Parse(value); err == nil {
		return binding.Label
	}
	return value // 错误由保存/注册时明确报告，不能静默换成另一组按键。
}

func normalizeRecent(books []RecentBook) []RecentBook {
	// 不修改调用者传入的 slice，防止设置快照互相影响。
	copyBooks := append([]RecentBook(nil), books...)
	sort.SliceStable(copyBooks, func(i, j int) bool { return copyBooks[i].LastRead.After(copyBooks[j].LastRead) })
	seen := make(map[string]bool)
	result := make([]RecentBook, 0, min(len(books), MaxRecentBooks))
	for _, book := range copyBooks {
		key := pathKey(book.Path)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		book.Name = filepath.Base(book.Path)
		if book.PositionBytes < 0 {
			book.PositionBytes = 0
		}
		result = append(result, book)
		if len(result) == MaxRecentBooks {
			break
		}
	}
	return result
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
