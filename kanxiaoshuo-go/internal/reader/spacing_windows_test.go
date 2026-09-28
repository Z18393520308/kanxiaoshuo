//go:build windows

package reader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"kanxiaoshuo-go/internal/config"
)

func TestRichFormatWindowsABI(t *testing.T) {
	if size, offset := unsafe.Sizeof(charFormat2W{}), unsafe.Offsetof(charFormat2W{}.Spacing); size != 116 || offset != 94 {
		t.Fatalf("CHARFORMAT2W size/spacing offset = %d/%d, want 116/94", size, offset)
	}
}

func TestReaderWindowsSpacingAndScreenEdges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	cfg := config.Default()
	cfg.Hotkeys = config.Hotkeys{Up: "Ctrl+Alt+Shift+F6", Down: "Ctrl+Alt+Shift+F7", Hide: "Ctrl+Alt+Shift+F8", Show: "Ctrl+Alt+Shift+F9", Move: "Ctrl+Alt+Shift+F10"}
	cfg.FontSize, cfg.WindowWidth, cfg.VisibleLines = 18, 300, 2
	cfg.BookPath = filepath.Join(dir, "字距.txt")
	text := strings.Repeat("汉字阅读甲乙丙丁戊己庚辛壬癸", 40) + "\r\nA中😀\r\n\r\n下一段继续。"
	if err := os.WriteFile(cfg.BookPath, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Start(cfg, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Close(); err != nil {
			t.Error(err)
		}
	})
	var naturalAdvance int32
	var naturalLineBytes int
	if err := operate(false, func(w *Window) error {
		naturalAdvance = w.characterPoint(1).X - w.characterPoint(0).X
		naturalLineBytes = w.lineByteOffset(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cfg.LetterSpacing = 8
	if err := ApplySettings(cfg); err != nil {
		t.Fatal(err)
	}
	if err := operate(false, func(w *Window) error {
		advance := w.characterPoint(1).X - w.characterPoint(0).X
		if extra := advance - naturalAdvance; extra < 7 || extra > 9 {
			return fmt.Errorf("8 px spacing not rendered: advance %d -> %d", naturalAdvance, advance)
		}
		if offset := w.lineByteOffset(1); offset <= 0 || offset >= naturalLineBytes {
			return fmt.Errorf("spacing not used in wrapping: %d -> %d", naturalLineBytes, offset)
		}
		// Native line indexes count CRLF as one; bookmark positions must count both bytes.
		start := strings.Index(w.text, "A中😀")
		w.displayAt(start)
		if got := w.lineByteOffset(1); got != len("A中😀\r\n") {
			return fmt.Errorf("native newline mapping: got %d", got)
		}
		w.pageDown()
		if w.position != start+len("A中😀\r\n\r\n") {
			return fmt.Errorf("page skipped text after CRLF: %d", w.position)
		}
		w.pageUp()
		if w.position != start {
			return fmt.Errorf("page round trip lost bookmark")
		}
		var bounds rect
		procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&bounds)))
		monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&bounds)), 2)
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, _ := procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
			return fmt.Errorf("cannot read monitor bounds")
		}
		width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
		for _, edge := range [][2]int32{
			{info.Monitor.Left, info.Monitor.Bottom - height},
			{info.Monitor.Right - width, info.Monitor.Top},
		} {
			procSetWindowPos.Call(uintptr(w.hwnd), 0, uintptr(edge[0]), uintptr(edge[1]), 0, 0, 0x0015)
			procSendMessageW.Call(uintptr(w.hwnd), wmExitSizeMove, 0, 0)
			procGetWindowRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&bounds)))
			if bounds.Left != edge[0] || bounds.Top != edge[1] {
				return fmt.Errorf("screen-edge drag jumped from %v to %d,%d (work bottom %d)", edge, bounds.Left, bounds.Top, info.Work.Bottom)
			}
			if w.cfg.WindowLeft != float64(edge[0]) || w.cfg.WindowTop != float64(edge[1]) {
				return fmt.Errorf("edge placement not remembered")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// 最大值与归零都应重新排版，不能留下上一种字距的格式。
	for _, spacing := range []int{20, 0} {
		if err := Seek(0); err != nil {
			t.Fatal(err)
		}
		cfg.LetterSpacing = spacing
		if err := ApplySettings(cfg); err != nil {
			t.Fatal(err)
		}
		if err := operate(false, func(w *Window) error {
			extra := w.characterPoint(1).X - w.characterPoint(0).X - naturalAdvance
			if extra < int32(spacing)-1 || extra > int32(spacing)+1 {
				return fmt.Errorf("spacing %d rendered as %d", spacing, extra)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
