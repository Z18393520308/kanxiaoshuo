package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"kanxiaoshuo-go/internal/hotkey"
)

const (
	// CurrentSchemaVersion 的阅读位置以解码后 UTF-8 文本的字节偏移表示。
	CurrentSchemaVersion = 2
	MaxRecentBooks       = 30
)

// Hotkeys 使用 RegisterHotKey 可识别的组合键；翻页默认带 Alt，避免占用裸方向键。
type Hotkeys struct {
	Up   string `json:"up"`
	Down string `json:"down"`
	Hide string `json:"hide"`
	Show string `json:"show"`
	Move string `json:"move"`
}

func (h Hotkeys) Map() map[string]string {
	return map[string]string{"up": h.Up, "down": h.Down, "hide": h.Hide, "show": h.Show, "move": h.Move}
}

// RecentBook 只保存稳定信息；文件是否存在应在展示时重新检查。
type RecentBook struct {
	Path          string    `json:"path"`
	Name          string    `json:"name"`
	PositionBytes int       `json:"position_bytes"`
	LastRead      time.Time `json:"last_read"`
}

// Settings 写入 %AppData%\摸鱼联盟\config.json。
// 旧版没有 schema_version，保留旧索引供阅读器在解码正文后进行一次迁移。
type Settings struct {
	SchemaVersion int          `json:"schema_version"`
	BookPath      string       `json:"book_path"`
	LineIndex     int          `json:"line_index"`
	CharIndex     int          `json:"char_index"`
	PositionBytes int          `json:"position_bytes"`
	WindowLeft    float64      `json:"window_left"`
	WindowTop     float64      `json:"window_top"`
	WindowWidth   int          `json:"window_width"`
	FontFamily    string       `json:"font_family"`
	FontSize      int          `json:"font_size"`
	LetterSpacing int          `json:"letter_spacing"`
	FontColor     string       `json:"font_color"`
	VisibleLines  int          `json:"visible_lines"`
	Hotkeys       Hotkeys      `json:"hotkeys"`
	RecentBooks   []RecentBook `json:"recent_books"`
}

// 所有读改写都持有同一把锁，避免设置窗口用旧快照覆盖阅读器刚保存的进度。
var fileMu sync.Mutex

// 测试替换该函数即可隔离配置，不访问用户的实际配置目录。
var userConfigDir = os.UserConfigDir

func Default() Settings {
	return Settings{
		SchemaVersion: CurrentSchemaVersion,
		FontSize:      12, FontColor: "#000000", FontFamily: "Microsoft YaHei",
		VisibleLines: 1, WindowWidth: ReaderWidth, WindowLeft: 100, WindowTop: 100,
		Hotkeys:     Hotkeys{Up: "Alt+Up", Down: "Alt+Down", Hide: "Alt+C", Show: "Alt+S", Move: "Alt+T"},
		RecentBooks: []RecentBook{},
	}
}

func Dir() (string, error) {
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "摸鱼联盟")
	return dir, os.MkdirAll(dir, 0o755)
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	return loadLocked()
}

func loadLocked() (Settings, error) {
	s := Default()
	p, err := Path()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	s.SchemaVersion = 0 // 缺失版本号代表旧格式，不能提前标记字节位置迁移完成。
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("读取配置失败: %w", err)
	}
	return Normalize(s), nil
}

// Save 整体保存配置；并发阅读进度/偏好更新应分别使用 SaveProgress/SavePreferences。
func Save(s Settings) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	return saveLocked(Normalize(s))
}

func saveLocked(s Settings) error {
	if err := hotkey.Validate(s.Hotkeys.Map()); err != nil {
		return err
	}
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// 临时文件必须与目标位于同一目录，才能原子替换；写入失败保留原配置。
	f, err := os.CreateTemp(filepath.Dir(p), ".config-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, p)
}

// PreviewPreferences 从最新配置预览合并结果，以便先验证阅读器/热键再提交。
func PreviewPreferences(s Settings) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := loadLocked()
	if err != nil {
		return current, err
	}
	return mergePreferences(current, s)
}

// SavePreferences 合并样式、热键及当前选书，保留磁盘上最新进度与窗口坐标。
func SavePreferences(s Settings) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := loadLocked()
	if err != nil {
		return current, err
	}
	current, err = mergePreferences(current, s)
	if err != nil {
		return current, err
	}
	return current, saveLocked(current)
}

func mergePreferences(current, s Settings) (Settings, error) {
	s = Normalize(s)
	if err := hotkey.Validate(s.Hotkeys.Map()); err != nil {
		return current, err
	}
	current.FontSize = s.FontSize
	current.LetterSpacing = s.LetterSpacing
	current.FontColor = s.FontColor
	current.FontFamily = s.FontFamily
	current.VisibleLines = s.VisibleLines
	current.WindowWidth = s.WindowWidth
	current.Hotkeys = s.Hotkeys
	oldPath := current.BookPath
	current.BookPath = s.BookPath
	if entry, ok := findRecent(current.RecentBooks, s.BookPath); ok {
		current.PositionBytes = entry.PositionBytes
		current.CharIndex, current.LineIndex = 0, 0
		current.SchemaVersion = CurrentSchemaVersion
		current.RecentBooks = updateRecent(current.RecentBooks, s.BookPath, entry.PositionBytes, time.Now())
	} else if !samePath(oldPath, s.BookPath) {
		current.PositionBytes, current.CharIndex, current.LineIndex = 0, 0, 0
		current.SchemaVersion = CurrentSchemaVersion
		current.RecentBooks = updateRecent(current.RecentBooks, s.BookPath, 0, time.Now())
	} else if current.SchemaVersion >= CurrentSchemaVersion {
		current.RecentBooks = updateRecent(current.RecentBooks, s.BookPath, current.PositionBytes, time.Now())
	}
	return Normalize(current), nil
}

// SaveProgress 更新指定书的进度。旧阅读窗口迟到的保存不会把新选书切回去。
func SaveProgress(path string, position int, left, top float64) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := loadLocked()
	if err != nil {
		return err
	}
	if position < 0 {
		position = 0
	}
	current.RecentBooks = updateRecent(current.RecentBooks, path, position, time.Now())
	if path != "" && samePath(current.BookPath, path) {
		current.PositionBytes = position
		current.CharIndex, current.LineIndex = 0, 0
		current.SchemaVersion = CurrentSchemaVersion
	}
	current.WindowLeft, current.WindowTop = left, top
	return saveLocked(Normalize(current))
}

// PreviewRelocateBook 预览重新定位，不修改磁盘。
func PreviewRelocateBook(oldPath, newPath string) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := loadLocked()
	if err != nil {
		return current, err
	}
	return mergeRelocation(current, oldPath, newPath)
}

// RelocateBook 将历史条目关联到用户重新选择的文件，不丢失原阅读位置。
func RelocateBook(oldPath, newPath string) (Settings, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	current, err := loadLocked()
	if err != nil {
		return current, err
	}
	current, err = mergeRelocation(current, oldPath, newPath)
	if err != nil {
		return current, err
	}
	return current, saveLocked(current)
}

func mergeRelocation(current Settings, oldPath, newPath string) (Settings, error) {
	if oldPath == "" || newPath == "" {
		return current, fmt.Errorf("原书籍路径和新路径不能为空")
	}
	info, err := os.Stat(newPath)
	if err != nil {
		return current, fmt.Errorf("无法打开重新选择的书籍: %w", err)
	}
	if !info.Mode().IsRegular() {
		return current, fmt.Errorf("重新选择的路径不是普通文件")
	}
	entry, found := findRecent(current.RecentBooks, oldPath)
	isCurrent := samePath(current.BookPath, oldPath)
	if !found && !isCurrent {
		return current, fmt.Errorf("没有找到待重新定位的阅读记录")
	}
	if !found {
		entry = RecentBook{Path: oldPath, PositionBytes: current.PositionBytes}
	}
	books := make([]RecentBook, 0, len(current.RecentBooks))
	for _, book := range current.RecentBooks {
		if !samePath(book.Path, oldPath) && !samePath(book.Path, newPath) {
			books = append(books, book)
		}
	}
	// 未迁移的旧配置仍使用 CharIndex/LineIndex，不提前建立字节位置为 0 的历史。
	if found || !isCurrent || current.SchemaVersion >= CurrentSchemaVersion {
		books = updateRecent(books, newPath, entry.PositionBytes, time.Now())
	}
	current.RecentBooks = books
	if isCurrent || samePath(current.BookPath, newPath) {
		current.BookPath = newPath
		if found {
			current.PositionBytes = entry.PositionBytes
			current.CharIndex, current.LineIndex = 0, 0
			current.SchemaVersion = CurrentSchemaVersion
		}
	}
	return Normalize(current), nil
}

func pathKey(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func samePath(a, b string) bool { return pathKey(a) == pathKey(b) }

func findRecent(books []RecentBook, path string) (RecentBook, bool) {
	if path != "" {
		for _, book := range books {
			if samePath(book.Path, path) {
				return book, true
			}
		}
	}
	return RecentBook{}, false
}

func updateRecent(books []RecentBook, path string, position int, now time.Time) []RecentBook {
	if path == "" {
		return books
	}
	updated := make([]RecentBook, 0, len(books)+1)
	updated = append(updated, RecentBook{Path: path, Name: filepath.Base(path), PositionBytes: position, LastRead: now})
	for _, book := range books {
		if !samePath(book.Path, path) {
			updated = append(updated, book)
		}
	}
	return normalizeRecent(updated)
}
