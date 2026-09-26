package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanxiaoshuo-go/internal/config"
)

type readerStub struct {
	book                  config.Settings
	running               bool
	showError, closeError error
	refusePath            string
	seekCalls             []int
}

func (s *readerStub) controls() readerControls {
	return readerControls{
		start: func(cfg config.Settings) error { s.book = cfg; s.running = true; return nil },
		apply: func(cfg config.Settings) error {
			if cfg.BookPath == s.refusePath {
				return errors.New("快捷键已被占用")
			}
			position := s.book.PositionBytes
			sameBook := cfg.BookPath == s.book.BookPath
			s.book = cfg
			if sameBook {
				s.book.PositionBytes = position
			} // 与真实阅读器一致：普通改样式不改变当前位置。
			return nil
		},
		show: func() error { return s.showError },
		seek: func(position int) error {
			s.book.PositionBytes = position
			s.seekCalls = append(s.seekCalls, position)
			return nil
		},
		close: func() error {
			if s.closeError != nil {
				return s.closeError
			}
			s.running = false
			return nil
		},
	}
}

func TestPreferencesRegisterFailureDoesNotCommit(t *testing.T) {
	before := config.Settings{BookPath: "before.txt", PositionBytes: 120, FontSize: 12}
	next := config.Settings{BookPath: "next.txt", PositionBytes: 800, FontSize: 24}
	stub := &readerStub{book: before, running: true, refusePath: next.BookPath}
	committed := false
	running, err := commitPreferences(before, next, true, stub.controls(), func(config.Settings) error { committed = true; return nil })
	if err == nil || committed || !running || stub.book.BookPath != before.BookPath || stub.book.PositionBytes != 120 {
		t.Fatalf("failed hotkeys changed live/persisted state: running=%v err=%v committed=%v book=%+v", running, err, committed, stub.book)
	}
}

func TestPreferenceSaveFailureRestoresBookAndExactPosition(t *testing.T) {
	before := config.Settings{BookPath: "before.txt", PositionBytes: 125, FontSize: 12}
	next := config.Settings{BookPath: "next.txt", PositionBytes: 888, FontSize: 24}
	stub := &readerStub{book: before, running: true}
	running, err := commitPreferences(before, next, true, stub.controls(), func(config.Settings) error { return errors.New("磁盘写入失败") })
	if err == nil || !running || stub.book.BookPath != before.BookPath || stub.book.PositionBytes != before.PositionBytes || stub.book.FontSize != 12 {
		t.Fatalf("failed save did not restore reading session: %+v, %v", stub, err)
	}
	if len(stub.seekCalls) != 1 || stub.seekCalls[0] != 125 {
		t.Fatalf("exact position not restored: %+v", stub.seekCalls)
	}
}

func TestShowFailureDoesNotPersistNewHotkeys(t *testing.T) {
	before := config.Settings{BookPath: "before.txt", PositionBytes: 500}
	stub := &readerStub{book: before, running: true, showError: errors.New("显示时翻页键占用")}
	committed := false
	_, err := commitPreferences(before, config.Settings{BookPath: "next.txt"}, true, stub.controls(), func(config.Settings) error { committed = true; return nil })
	if err == nil || committed || stub.book.BookPath != before.BookPath || stub.book.PositionBytes != 500 {
		t.Fatalf("show failure was committed: %+v err=%v", stub, err)
	}
}

func TestFailedRollbackCloseKeepsRunningState(t *testing.T) {
	stub := &readerStub{closeError: errors.New("无法刷盘")}
	running, err := commitPreferences(config.Settings{}, config.Settings{BookPath: "new.txt"}, false, stub.controls(), func(config.Settings) error { return errors.New("配置保存失败") })
	if !running || !stub.running || err == nil || !strings.Contains(err.Error(), "无法刷盘") {
		t.Fatalf("reader remains alive but UI forgot it: running=%v stub=%+v err=%v", running, stub, err)
	}
}

func TestRelocationToCurrentBookAppliesOriginalBookmarkBeforeCommit(t *testing.T) {
	before := config.Settings{BookPath: "current.txt", PositionBytes: 900}
	next := config.Settings{BookPath: "current.txt", PositionBytes: 300}
	stub := &readerStub{book: before, running: true}
	committed := false
	err := commitRelocation(before, next, true, stub.controls(), func() error {
		if stub.book.PositionBytes != 300 {
			t.Fatalf("old bookmark not restored before committing: %+v", stub.book)
		}
		committed = true
		return nil
	})
	if err != nil || !committed || stub.book.PositionBytes != 300 {
		t.Fatalf("relocation failed: %+v, %v", stub, err)
	}
}

func TestRelocationCommitFailureRestoresSameBookPosition(t *testing.T) {
	before := config.Settings{BookPath: "current.txt", PositionBytes: 900}
	next := config.Settings{BookPath: "current.txt", PositionBytes: 300}
	stub := &readerStub{book: before, running: true}
	err := commitRelocation(before, next, true, stub.controls(), func() error { return errors.New("配置只读") })
	if err == nil || stub.book.PositionBytes != 900 || len(stub.seekCalls) != 2 {
		t.Fatalf("rollback lost bookmark: %+v, %v", stub, err)
	}
}

func TestInactiveRelocationDoesNotTouchReader(t *testing.T) {
	called := false
	err := commitRelocation(config.Settings{}, config.Settings{}, false, readerControls{}, func() error { called = true; return nil })
	if err != nil || !called {
		t.Fatalf("inactive relocation failed: %v", err)
	}
}

func TestMissingLegacyBookStillHasRelocationEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "移动过的旧书.txt")
	view := recentBooksForView(config.Settings{BookPath: path, CharIndex: 300, SchemaVersion: 0})
	if len(view) != 1 || !view[0].Missing || view[0].Path != path {
		t.Fatalf("missing legacy book omitted: %+v", view)
	}
}

func TestRecentBooksRecheckFileAndAvoidDuplicateCurrentBook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "故事.txt")
	if err := os.WriteFile(path, []byte("故事"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Settings{BookPath: path, RecentBooks: []config.RecentBook{{Path: path, PositionBytes: 3}}}
	view := recentBooksForView(cfg)
	if len(view) != 1 || view[0].Missing || view[0].Name != "故事.txt" {
		t.Fatalf("unexpected existing book: %+v", view)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if !recentBooksForView(cfg)[0].Missing {
		t.Fatal("file status was cached after deletion")
	}
}
