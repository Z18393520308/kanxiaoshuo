//go:build windows

package reader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"kanxiaoshuo-go/internal/config"
	"kanxiaoshuo-go/internal/hotkey"
)

// 在 Windows CI 上验证真正的 EDIT 排版和 RegisterHotKey，而非复刻生产算法。
// 本地跨平台测试只覆盖编码与位置转换，不能代替本组系统集成测试。
func TestReaderWindowsLifecycle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	cfg := config.Default()
	cfg.Hotkeys = config.Hotkeys{Up: "Ctrl+Alt+Shift+F6", Down: "Ctrl+Alt+Shift+F7", Hide: "Ctrl+Alt+Shift+F8", Show: "Ctrl+Alt+Shift+F9", Move: "Ctrl+Alt+Shift+F10"}
	cfg.VisibleLines = 3
	cfg.FontSize = 14
	cfg.WindowWidth = 480
	cfg.BookPath = filepath.Join(dir, "小说.txt")
	text := strings.Repeat("第十章 中文阅读😀emoji keeps surrogate pairs intact. A-long-English-word-to-wrap。\r\n下一段继续，不能漏掉结尾。\r\n\r\n", 900)
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
			t.Errorf("cleanup Close: %v", err)
		}
	})

	var saved int
	if err := operate(false, func(w *Window) error {
		// 先走到正文中间，检查上一页/下一页往返与真实行索引保存。
		for i := 0; i < 25; i++ {
			before := w.position
			w.pageDown()
			if w.position <= before {
				return fmt.Errorf("page %d did not advance", i)
			}
		}
		saved = w.position
		w.pageUp()
		w.pageDown()
		if w.position != saved {
			return fmt.Errorf("page round trip: %d != %d", w.position, saved)
		}
		if !utf8.ValidString(w.text[:saved]) {
			return fmt.Errorf("position split UTF-8")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Flush(); err != nil {
		t.Fatal(err)
	}
	stored, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.PositionBytes != saved {
		t.Fatalf("persisted %d, visible %d", stored.PositionBytes, saved)
	}
	emoji := strings.Index(text, "😀")
	if err := Seek(emoji + 2); err != nil {
		t.Fatal(err)
	}
	if err := operate(false, func(w *Window) error {
		if w.position != emoji || len(w.previous) != 0 || len(w.forward) != 0 {
			return fmt.Errorf("Seek did not align or reset navigation: %d", w.position)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Seek(saved); err != nil {
		t.Fatal(err)
	}

	cfg.WindowWidth = 320
	cfg.VisibleLines = 5
	cfg.FontSize = 18
	cfg.FontColor = "#FFFFFF"
	if err := ApplySettings(cfg); err != nil {
		t.Fatal(err)
	}
	if err := operate(false, func(w *Window) error {
		if w.position != saved {
			return fmt.Errorf("layout lost position: %d != %d", w.position, saved)
		}
		if w.colorKey == 0xFFFFFF {
			return fmt.Errorf("white text collides with transparency")
		}
		// 重排后的上一页走无历史路径，下一页必须回到原字节位置。
		w.pageUp()
		w.pageDown()
		if w.position != saved {
			return fmt.Errorf("reflow round trip lost position: %d", w.position)
		}
		// 超过旧版分块末页，并一直翻到全文结尾，不能在中间块的末页卡住。
		for n := 0; n < 20000; n++ {
			before := w.position
			w.pageDown()
			if w.position == before {
				if w.chunkEnd != len(w.text) {
					return fmt.Errorf("stalled before EOF: %d/%d", w.chunkEnd, len(w.text))
				}
				if n == 0 {
					return fmt.Errorf("never advanced after reflow")
				}
				return nil
			}
			if w.position < before || !utf8.ValidString(w.text[:w.position]) {
				return fmt.Errorf("invalid page position %d", w.position)
			}
		}
		return fmt.Errorf("pagination never reached final page")
	}); err != nil {
		t.Fatal(err)
	}

	if err := Hide(); err != nil {
		t.Fatal(err)
	}
	if !probeHotkey(t, cfg.Hotkeys.Up) {
		t.Fatal("hidden reader did not release page-up hotkey")
	}
	if err := Show(); err != nil {
		t.Fatal(err)
	}
	if probeHotkey(t, cfg.Hotkeys.Up) {
		t.Fatal("visible reader did not reacquire page-up hotkey")
	}
	// 无效设置不能替换正在阅读的书、热键和位置。
	invalid := cfg
	invalid.Hotkeys.Down = invalid.Hotkeys.Up
	if err := ApplySettings(invalid); err == nil {
		t.Fatal("duplicate hotkey accepted")
	}
	if err := Flush(); err != nil {
		t.Fatal(err)
	}
	// 保存失败应保留运行中的阅读条，恢复配置文件后允许重试关闭。
	configPath, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	backup := configPath + ".test-backup"
	if err := os.Rename(configPath, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(configPath, 0700); err != nil {
		t.Fatal(err)
	}
	closeErr := Close()
	showErr := Show()
	if err := os.Remove(configPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, configPath); err != nil {
		t.Fatal(err)
	}
	if closeErr == nil || showErr != nil {
		t.Fatalf("Close did not preserve window on save failure: close=%v show=%v", closeErr, showErr)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	// 连续 Start/Close 不应受上一窗口的 WM_QUIT 或已注册窗口类影响。
	stored, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := Start(stored, nil); err != nil {
		t.Fatal(err)
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	// 启动失败也必须完整释放线程和热键，允许随后重试。
	if err := Start(invalid, nil); err == nil {
		t.Fatal("invalid Start unexpectedly succeeded")
	}
	if err := Start(stored, nil); err != nil {
		t.Fatal(err)
	}
}

// 在独立 OS 线程注册同一组合键，测量系统中的实际占用情况。
func probeHotkey(t *testing.T, value string) bool {
	t.Helper()
	b, err := hotkey.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan bool, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		ok, _, _ := procRegisterHotKey.Call(0, 991, uintptr(b.Modifiers|0x4000), uintptr(b.Key))
		if ok != 0 {
			procUnregisterHotKey.Call(0, 991)
		}
		result <- ok != 0
	}()
	return <-result
}
