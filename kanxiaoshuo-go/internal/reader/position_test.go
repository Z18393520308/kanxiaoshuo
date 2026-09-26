package reader

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUTF16Offsets(t *testing.T) {
	s := "A中😀\r\n文"
	want := []int{0, 1, 4, 4, 8, 9, 10, 13, 13}
	for n, w := range want {
		if got := utf16OffsetToBytes(s, n); got != w {
			t.Errorf("units %d: got %d want %d", n, got, w)
		}
	}
}

func TestUTF8ChunksNeverSplitCharacters(t *testing.T) {
	s := strings.Repeat("中😀\r\nA", 100000)
	start := 0
	for start < len(s) {
		end := byteBoundary(s, min(start+chunkBytes, len(s)))
		if end <= start || !utf8.ValidString(s[start:end]) {
			t.Fatalf("invalid chunk %d:%d", start, end)
		}
		if end < len(s) && s[end] == '\n' {
			t.Fatal("split CRLF")
		}
		start = end
	}
}

func TestLegacyPositionMigration(t *testing.T) {
	s := "一\r\n二😀\r\n三"
	if got := migrateLegacyPosition(s, 3, 0, 12, 480); got != 14 {
		t.Fatalf("small-book pos %d", got)
	}
	big := strings.Repeat("中\r\n", 1_000_001)
	if got := migrateLegacyPosition(big, 6, 0, 12, 480); got != 10 {
		t.Fatalf("large-book byte pos %d", got)
	}
}

func TestClampNegativeMonitor(t *testing.T) {
	x, y := clampRect(-1800, 100, 480, 50, -1920, 0, 0, 1080)
	if x != -1800 || y != 100 {
		t.Fatalf("valid negative monitor moved: %d,%d", x, y)
	}
	x, y = clampRect(4000, 2000, 480, 50, 0, 0, 1920, 1040)
	if x != 1440 || y != 990 {
		t.Fatalf("offscreen not clamped: %d,%d", x, y)
	}
}
