package ui

import (
	"os"
	"path/filepath"
	"strings"

	"kanxiaoshuo-go/internal/config"
)

type recentBookView struct {
	config.RecentBook
	Name    string `json:"name"`
	Missing bool   `json:"missing"`
}

func recentBooksForView(s config.Settings) []recentBookView {
	books := append([]config.RecentBook(nil), s.RecentBooks...)
	found := false
	for _, b := range books {
		if strings.EqualFold(filepath.Clean(b.Path), filepath.Clean(s.BookPath)) {
			found = true
			break
		}
	}
	// 旧版只记录当前书，没有历史列表；仍提供重新定位入口，避免文件移动后丢掉旧版书签。
	if s.BookPath != "" && !found {
		books = append([]config.RecentBook{{Path: s.BookPath, PositionBytes: s.PositionBytes}}, books...)
	}
	if len(books) > config.MaxRecentBooks {
		books = books[:config.MaxRecentBooks]
	}
	view := make([]recentBookView, 0, len(books))
	for _, b := range books {
		info, err := os.Stat(b.Path)
		view = append(view, recentBookView{RecentBook: b, Name: filepath.Base(b.Path), Missing: err != nil || info.IsDir()})
	}
	return view
}
