package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings 应用配置，首次运行后写入 %AppData%\摸鱼联盟\config.json
type Settings struct {
	BookPath   string  `json:"book_path"`
	LineIndex  int     `json:"line_index"`  // 兼容旧版：行号
	CharIndex  int     `json:"char_index"`  // 阅读位置：全文字符（rune）下标
	WindowLeft float64 `json:"window_left"`
	WindowTop  float64 `json:"window_top"`
	FontSize      int     `json:"font_size"`
	FontColor     string  `json:"font_color"` // 如 #000000
	VisibleLines  int     `json:"visible_lines"`
}

func Default() Settings {
	return Settings{
		FontSize:     12,
		FontColor:    "#000000",
		VisibleLines: 1,
		LineIndex:    0,
		WindowLeft:   100,
		WindowTop:    100,
	}
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
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
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), err
	}
	return Normalize(s), nil
}

func Save(s Settings) error {
	p, err := Path()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
