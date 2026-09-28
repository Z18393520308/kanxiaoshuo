package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func isolatedConfig(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	previous := userConfigDir
	userConfigDir = func() (string, error) { return base, nil }
	t.Cleanup(func() { userConfigDir = previous })
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mustSave(t *testing.T, s Settings) {
	t.Helper()
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
}

func mustLoad(t *testing.T) Settings {
	t.Helper()
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLegacyProgressSurvivesPreferencesUntilMigrated(t *testing.T) {
	p := isolatedConfig(t)
	legacy := []byte(`{"book_path":"旧书.txt","char_index":137,"line_index":8,"font_size":22,"visible_lines":12}`)
	if err := os.WriteFile(p, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	s := mustLoad(t)
	if s.SchemaVersion != 0 || s.CharIndex != 137 || s.LineIndex != 8 || s.FontSize != 22 {
		t.Fatalf("legacy fields lost: %+v", s)
	}
	if s.WindowWidth != 480 || s.Hotkeys.Up != "Alt+Up" {
		t.Fatalf("new defaults missing: %+v", s)
	}
	s.FontSize = 30
	s, err := SavePreferences(s)
	if err != nil {
		t.Fatal(err)
	}
	if s.SchemaVersion != 0 || s.CharIndex != 137 || len(s.RecentBooks) != 0 {
		t.Fatalf("preferences incorrectly migrated legacy position: %+v", s)
	}
	if err := SaveProgress(s.BookPath, 411, 120, 130); err != nil {
		t.Fatal(err)
	}
	s = mustLoad(t)
	if s.SchemaVersion != 2 || s.PositionBytes != 411 || s.CharIndex != 0 || s.LineIndex != 0 || s.RecentBooks[0].PositionBytes != 411 {
		t.Fatalf("migration was not persisted: %+v", s)
	}
}

func TestPreferencesPreserveFreshProgressAndLateOldBookSave(t *testing.T) {
	isolatedConfig(t)
	s := Default()
	s.BookPath = "第一本.txt"
	mustSave(t, s)
	stale := s
	if err := SaveProgress(s.BookPath, 900, -900, 250); err != nil {
		t.Fatal(err)
	}
	stale.FontSize = 24
	merged, err := SavePreferences(stale)
	if err != nil {
		t.Fatal(err)
	}
	if merged.PositionBytes != 900 || merged.WindowLeft != -900 || merged.FontSize != 24 {
		t.Fatalf("stale preferences overwrote live state: %+v", merged)
	}
	stale.BookPath = "第二本.txt"
	merged, err = SavePreferences(stale)
	if err != nil {
		t.Fatal(err)
	}
	if merged.PositionBytes != 0 || len(merged.RecentBooks) != 2 {
		t.Fatalf("new book state: %+v", merged)
	}
	if err := SaveProgress("第一本.txt", 930, 300, 400); err != nil {
		t.Fatal(err)
	}
	loaded := mustLoad(t)
	if loaded.BookPath != "第二本.txt" || loaded.PositionBytes != 0 || loaded.FontSize != 24 {
		t.Fatalf("late save replaced selection/style: %+v", loaded)
	}
	stale.BookPath = "第一本.txt"
	merged, err = SavePreferences(stale)
	if err != nil {
		t.Fatal(err)
	}
	if merged.PositionBytes != 930 || merged.SchemaVersion != 2 {
		t.Fatalf("resume did not use history: %+v", merged)
	}
}

func TestPreviewDoesNotWriteOrCreateHistory(t *testing.T) {
	p := isolatedConfig(t)
	s := Default()
	s.BookPath = "原书.txt"
	mustSave(t, s)
	before, _ := os.ReadFile(p)
	s.BookPath = "新书.txt"
	s.FontColor = "#ffffff"
	preview, err := PreviewPreferences(s)
	if err != nil {
		t.Fatal(err)
	}
	if preview.BookPath != s.BookPath || preview.FontColor != "#FFFFFF" || len(preview.RecentBooks) != 1 {
		t.Fatalf("incorrect preview: %+v", preview)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("preview modified config")
	}
}

func TestConcurrentProgressAndPreferencesKeepAllBooks(t *testing.T) {
	isolatedConfig(t)
	s := Default()
	s.BookPath = "current.txt"
	mustSave(t, s)
	var wg sync.WaitGroup
	errors := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			errors <- SaveProgress(fmt.Sprintf("book-%02d.txt", i), i*3, float64(i), 10)
		}(i)
		go func() {
			defer wg.Done()
			_, err := SavePreferences(s)
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := mustLoad(t)
	if len(got.RecentBooks) != 21 {
		t.Fatalf("lost concurrent updates: %d books", len(got.RecentBooks))
	}
	for i := 0; i < 20; i++ {
		entry, ok := findRecent(got.RecentBooks, fmt.Sprintf("book-%02d.txt", i))
		if !ok || entry.PositionBytes != i*3 {
			t.Fatalf("lost progress for book %d: %+v", i, entry)
		}
	}
}

func TestRecentBookLimitAndNormalizeDoesNotMutateInput(t *testing.T) {
	s := Default()
	now := time.Now()
	for i := 0; i < 35; i++ {
		s.RecentBooks = append(s.RecentBooks, RecentBook{Path: fmt.Sprintf("%d.txt", i), PositionBytes: i, LastRead: now.Add(time.Duration(i) * time.Second)})
	}
	s.RecentBooks = append(s.RecentBooks, RecentBook{Path: "34.txt", PositionBytes: 100, LastRead: now.Add(time.Hour)})
	n := Normalize(s)
	if len(n.RecentBooks) != 30 || n.RecentBooks[0].PositionBytes != 100 || n.RecentBooks[29].Path != "5.txt" {
		t.Fatalf("history cap/dedup/order incorrect: %+v", n.RecentBooks)
	}
	if s.RecentBooks[0].Path != "0.txt" || s.RecentBooks[35].Name != "" {
		t.Fatal("normalization mutated caller slice")
	}
}

func TestRelocatePreservesProgressAndMergesExistingPath(t *testing.T) {
	p := isolatedConfig(t)
	dir := t.TempDir()
	newPath := filepath.Join(dir, "新位置.txt")
	if err := os.WriteFile(newPath, []byte("中文小说"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.BookPath = "missing.txt"
	s.PositionBytes = 120
	s.RecentBooks = []RecentBook{{Path: s.BookPath, PositionBytes: 120}, {Path: newPath, PositionBytes: 30}}
	mustSave(t, s)
	before, _ := os.ReadFile(p)
	preview, err := PreviewRelocateBook(s.BookPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("relocation preview wrote file")
	}
	if len(preview.RecentBooks) != 1 || preview.PositionBytes != 120 || preview.BookPath != newPath {
		t.Fatalf("bad relocation preview: %+v", preview)
	}
	got, err := RelocateBook(s.BookPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.RecentBooks[0].Name != "新位置.txt" || got.RecentBooks[0].LastRead.IsZero() {
		t.Fatalf("bad history: %+v", got)
	}
	if _, err := RelocateBook(newPath, dir); err == nil {
		t.Fatal("accepted directory as book")
	}
	if _, err := RelocateBook(newPath, filepath.Join(dir, "missing.txt")); err == nil {
		t.Fatal("accepted nonexistent file")
	}
	if mustLoad(t).BookPath != newPath {
		t.Fatal("failed relocation changed selected book")
	}
}

func TestRelocateLegacyPreservesRunePosition(t *testing.T) {
	isolatedConfig(t)
	newPath := filepath.Join(t.TempDir(), "book.txt")
	if err := os.WriteFile(newPath, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.SchemaVersion, s.CharIndex, s.LineIndex = 0, 100, 10
	s.BookPath = "missing.txt"
	mustSave(t, s)
	got, err := RelocateBook(s.BookPath, newPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 0 || got.CharIndex != 100 || got.LineIndex != 10 || len(got.RecentBooks) != 0 {
		t.Fatalf("prematurely migrated legacy position: %+v", got)
	}
}

func TestRelocateHistoryOntoCurrentBookUsesOriginalProgress(t *testing.T) {
	isolatedConfig(t)
	newPath := filepath.Join(t.TempDir(), "current.txt")
	if err := os.WriteFile(newPath, []byte("book contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Default()
	s.BookPath = newPath
	s.PositionBytes, s.CharIndex, s.LineIndex = 30, 10, 2
	s.RecentBooks = []RecentBook{
		{Path: "missing.txt", PositionBytes: 120},
		{Path: newPath, PositionBytes: 30},
	}
	mustSave(t, s)
	preview, err := PreviewRelocateBook("missing.txt", newPath)
	if err != nil {
		t.Fatal(err)
	}
	if preview.BookPath != newPath || preview.PositionBytes != 120 || preview.SchemaVersion != 2 || preview.CharIndex != 0 || preview.LineIndex != 0 {
		t.Fatalf("collision did not update active book position: %+v", preview)
	}
	if len(preview.RecentBooks) != 1 || preview.RecentBooks[0].PositionBytes != 120 {
		t.Fatalf("collision did not preserve original history position: %+v", preview.RecentBooks)
	}
	if _, err := RelocateBook("missing.txt", newPath); err != nil {
		t.Fatal(err)
	}
	stored := mustLoad(t)
	if stored.PositionBytes != 120 || stored.CharIndex != 0 || stored.LineIndex != 0 || len(stored.RecentBooks) != 1 {
		t.Fatalf("commit differs from relocation preview: %+v", stored)
	}
}

func TestNormalizeBoundsAndWhiteTextColor(t *testing.T) {
	s := Default()
	s.FontSize, s.VisibleLines, s.WindowWidth = 99, 20, 9000
	s.LetterSpacing = 99
	s.FontColor = "#ffffff"
	s.WindowLeft, s.WindowTop = math.NaN(), math.Inf(1)
	s.Hotkeys.Up = "shift + ctrl + pageup"
	n := Normalize(s)
	if n.FontSize != 48 || n.VisibleLines != 15 || n.WindowWidth != 1600 || n.LetterSpacing != 20 || n.FontColor != "#FFFFFF" || n.Hotkeys.Up != "Ctrl+Shift+PgUp" {
		t.Fatalf("bad normalization: %+v", n)
	}
	if n.WindowLeft != 100 || n.WindowTop != 100 {
		t.Fatalf("nonfinite position persisted: %+v", n)
	}
	s.FontSize, s.VisibleLines, s.WindowWidth = -1, -1, -1
	s.LetterSpacing = -1
	n = Normalize(s)
	if n.FontSize != 3 || n.VisibleLines != 1 || n.WindowWidth != 240 || n.LetterSpacing != 0 {
		t.Fatalf("bad lower bounds: %+v", n)
	}
}

func TestLetterSpacingMigratesAndSurvivesProgress(t *testing.T) {
	p := isolatedConfig(t)
	if err := os.WriteFile(p, []byte(`{"schema_version":2,"book_path":"字距.txt","font_size":18}`), 0600); err != nil {
		t.Fatal(err)
	}
	settings := mustLoad(t)
	if settings.LetterSpacing != 0 {
		t.Fatal("old configuration must use natural spacing")
	}
	if err := SaveProgress(settings.BookPath, 300, 100, 1060); err != nil {
		t.Fatal(err)
	}
	settings.LetterSpacing = 8
	if _, err := SavePreferences(settings); err != nil {
		t.Fatal(err)
	}
	loaded := mustLoad(t)
	if loaded.LetterSpacing != 8 || loaded.PositionBytes != 300 || loaded.WindowTop != 1060 {
		t.Fatalf("letter spacing changed progress/placement: %+v", loaded)
	}
	if err := SaveProgress(settings.BookPath, 600, 100, 1060); err != nil {
		t.Fatal(err)
	}
	if loaded := mustLoad(t); loaded.LetterSpacing != 8 || loaded.PositionBytes != 600 {
		t.Fatalf("progress save lost spacing: %+v", loaded)
	}
}

func TestInvalidSaveKeepsExistingFileAndNoTempFiles(t *testing.T) {
	p := isolatedConfig(t)
	s := Default()
	mustSave(t, s)
	before, _ := os.ReadFile(p)
	s.Hotkeys.Up = s.Hotkeys.Down
	if err := Save(s); err == nil {
		t.Fatal("accepted duplicate hotkey")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid save damaged existing config")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(p), ".config-*.json"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files leaked: %v %v", files, err)
	}
	var decoded Settings
	if err := json.Unmarshal(after, &decoded); err != nil {
		t.Fatal(err)
	}
}

func TestCorruptConfigIsNotSilentlyOverwritten(t *testing.T) {
	p := isolatedConfig(t)
	broken := []byte(`{"book_path":`)
	if err := os.WriteFile(p, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("missing parse error")
	}
	if _, err := SavePreferences(Default()); err == nil {
		t.Fatal("preferences ignored corrupt config")
	}
	if err := SaveProgress("book.txt", 100, 10, 10); err == nil {
		t.Fatal("progress ignored corrupt config")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(broken, after) {
		t.Fatal("corrupt config overwritten without repair")
	}
}
