//go:build windows

package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"sync"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"

	"kanxiaoshuo-go/internal/config"
	"kanxiaoshuo-go/internal/reader"
)

//go:embed webui/*
var webuiFS embed.FS

type webApp struct {
	w       webview2.WebView
	mu      sync.Mutex
	started bool
}

// Run 启动 WebView2 极简配置界面
func Run() error {
	sub, err := fs.Sub(webuiFS, "webui")
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: http.FileServer(http.FS(sub))}
	go srv.Serve(ln)
	defer srv.Close()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "摸鱼联盟",
			Width:  820,
			Height: 520,
			Center: true,
		},
	})
	if w == nil {
		return fmt.Errorf("无法创建 WebView2 窗口，请安装 Edge WebView2 运行时")
	}
	defer w.Destroy()

	app := &webApp{w: w}

	_ = w.Bind("getConfig", app.getConfig)
	_ = w.Bind("pickFile", app.pickFile)
	_ = w.Bind("saveConfig", app.saveConfig)
	_ = w.Bind("windowDrag", app.windowDrag)
	_ = w.Bind("windowMinimize", app.windowMinimize)
	_ = w.Bind("windowClose", app.windowClose)

	w.SetSize(820, 520, webview2.HintFixed)
	stripNativeChrome(uintptr(w.Window()))
	w.SetTitle("")
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d/", port))
	w.Run()
	return nil
}

func (a *webApp) getConfig() string {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	b, _ := json.Marshal(cfg)
	return string(b)
}

func (a *webApp) pickFile() string {
	return PickFile()
}

func (a *webApp) saveConfig(jsonStr string) string {
	var s config.Settings
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return "配置格式错误"
	}
	if s.BookPath == "" {
		return "请先选择 TXT 文件"
	}
	s = config.Normalize(s)
	if err := config.Save(s); err != nil {
		return err.Error()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started {
		a.started = true
		_ = reader.Start(s, nil)
	} else {
		reader.ApplySettings(s)
	}
	hideWindow(a.hwnd())
	return ""
}

func (a *webApp) windowDrag() {
	hwnd := a.hwnd()
	if hwnd == 0 {
		return
	}
	u := windows.NewLazySystemDLL("user32.dll")
	u.NewProc("ReleaseCapture").Call()
	u.NewProc("SendMessageW").Call(hwnd, 0x00A1, 2, 0)
}

func (a *webApp) windowMinimize() {
	showWindow(a.hwnd(), 6) // SW_MINIMIZE
}

func (a *webApp) windowClose() {
	a.mu.Lock()
	started := a.started
	a.mu.Unlock()
	if started {
		hideWindow(a.hwnd())
		return
	}
	a.w.Destroy()
}

func (a *webApp) hwnd() uintptr {
	return uintptr(a.w.Window())
}

func hideWindow(hwnd uintptr) { showWindow(hwnd, 0) }

func showWindow(hwnd uintptr, cmd uintptr) {
	if hwnd == 0 {
		return
	}
	windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow").Call(hwnd, cmd)
}

func stripNativeChrome(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	u := windows.NewLazySystemDLL("user32.dll")
	getLong := u.NewProc("GetWindowLongPtrW")
	setLong := u.NewProc("SetWindowLongPtrW")
	setPos := u.NewProc("SetWindowPos")
	gwlStyle := uintptr(0xFFFFFFFFFFFFFFF0) // GWL_STYLE = -16
	style, _, _ := getLong.Call(hwnd, gwlStyle)
	style &^= 0x00C00000 // WS_CAPTION
	style &^= 0x00010000 // WS_MAXIMIZEBOX
	setLong.Call(hwnd, gwlStyle, style)
	setPos.Call(hwnd, 0, 0, 0, 0, 0, 0x0027) // SWP_NOMOVE|SWP_NOSIZE|SWP_NOZORDER|SWP_FRAMECHANGED
}
