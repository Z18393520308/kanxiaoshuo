//go:build windows

package ui

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
	"golang.org/x/sys/windows"

	"kanxiaoshuo-go/internal/config"
	"kanxiaoshuo-go/internal/hotkey"
	"kanxiaoshuo-go/internal/reader"
	"kanxiaoshuo-go/internal/version"
)

//go:embed webui/*
var webuiFS embed.FS

type webApp struct {
	w        webview2.WebView
	handle   uintptr
	tray     *trayIcon
	actionMu sync.Mutex // 文件读取、切书、保存、退出严格串行；不阻塞 WebView 消息线程。
	started  bool
	quitting atomic.Bool
	closed   atomic.Bool
}

type nativeRequest struct {
	ID      int             `json:"id"`
	Action  string          `json:"action"`
	Payload json.RawMessage `json:"payload"`
}

type configView struct {
	Settings    config.Settings  `json:"settings"`
	RecentBooks []recentBookView `json:"recent_books"`
	Started     bool             `json:"started"`
	Version     string           `json:"version"`
}

// Run 的窗口创建、WebView2 COM 对象、消息循环始终位于同一个 Windows 线程。
func Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	runtimeVersion, err := webviewloader.GetInstalledVersion()
	if err != nil || runtimeVersion == "" {
		return errors.New("未找到可用的 Microsoft Edge WebView2 运行时。\n请安装 WebView2 Evergreen 运行时后重试。\n下载地址：https://go.microsoft.com/fwlink/p/?LinkId=2124703")
	}

	sub, err := fs.Sub(webuiFS, "webui")
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: http.FileServer(http.FS(sub))}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus:     true,
		WindowOptions: webview2.WindowOptions{Title: "摸鱼联盟", Width: 840, Height: 600, Center: true},
	})
	if w == nil {
		return errors.New("无法打开设置窗口。请安装 Microsoft Edge WebView2 Evergreen 运行时后重试。\n下载地址：https://go.microsoft.com/fwlink/p/?LinkId=2124703")
	}
	app := &webApp{w: w, handle: uintptr(w.Window())}
	// Destroy 只发送 WM_CLOSE；循环结束后直接销毁本线程窗口，避免留下无消息循环的窗口。
	defer func() {
		app.closed.Store(true)
		_ = reader.Close()
		if app.tray != nil {
			app.tray.dispose()
		}
		user32.NewProc("DestroyWindow").Call(app.handle)
	}()

	if err := w.Bind("nativeCommand", app.command); err != nil {
		return fmt.Errorf("初始化设置窗口通信失败：%w", err)
	}
	w.SetSize(840, 600, webview2.HintNone)
	w.SetSize(760, 560, webview2.HintMin)
	w.SetTitle("摸鱼联盟 · 阅读设置")
	app.tray, err = newTrayIcon(app)
	if err != nil {
		return err // 没有托盘不能隐藏设置，否则用户无法再次进入。
	}
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port))
	w.Run()
	return nil
}

// WebView 的 Bind 回调运行在 COM 界面线程。立即返回，由后台工作结束后通过 Dispatch 发送结果。
func (a *webApp) command(raw string) {
	var req nativeRequest
	if json.Unmarshal([]byte(raw), &req) != nil || req.ID < 1 || a.quitting.Load() {
		return
	}
	switch req.Action {
	case "pickFile":
		a.dispatch(func() {
			path, err := PickFile(a.handle)
			a.reply(req.ID, path, err)
		})
	case "windowClose":
		go a.closeSettings()
		a.reply(req.ID, nil, nil)
	case "exit":
		a.requestExit()
		a.reply(req.ID, nil, nil)
	default:
		go func() {
			a.actionMu.Lock()
			defer a.actionMu.Unlock()
			if a.quitting.Load() {
				a.reply(req.ID, nil, errors.New("程序正在退出"))
				return
			}
			var value any
			var err error
			switch req.Action {
			case "getConfig":
				value, err = a.getConfig()
			case "saveConfig":
				var s config.Settings
				if err = json.Unmarshal(req.Payload, &s); err == nil {
					err = a.saveConfig(s)
				}
			case "relocateBook":
				var paths struct {
					OldPath string `json:"old_path"`
					NewPath string `json:"new_path"`
				}
				if err = json.Unmarshal(req.Payload, &paths); err == nil {
					value, err = a.relocateBook(paths.OldPath, paths.NewPath)
				}
			case "showReader":
				err = a.showReader()
			default:
				err = errors.New("未知窗口操作")
			}
			a.reply(req.ID, value, err)
		}()
	}
}

func (a *webApp) dispatch(fn func()) {
	if !a.closed.Load() {
		a.w.Dispatch(func() {
			if !a.closed.Load() {
				fn()
			}
		})
	}
}

func (a *webApp) reply(id int, value any, err error) {
	response := struct {
		ID    int    `json:"id"`
		Value any    `json:"value"`
		Error string `json:"error,omitempty"`
	}{ID: id, Value: value}
	if err != nil {
		response.Error = err.Error()
	}
	data, _ := json.Marshal(response)
	a.dispatch(func() { a.w.Eval("window.nativeReply && window.nativeReply(" + string(data) + ")") })
}

// 每次重开设置都先刷新阅读位置，再读磁盘；不把第一次打开时的旧位置重新写回。
func (a *webApp) getConfig() (configView, error) {
	if err := reader.Flush(); err != nil {
		return configView{}, fmt.Errorf("保存当前阅读位置失败：%w", err)
	}
	s, err := config.Load()
	if err != nil {
		return configView{}, fmt.Errorf("读取配置失败：%w", err)
	}
	return configView{Settings: s, Started: a.started, Version: version.Version, RecentBooks: recentBooksForView(s)}, nil
}

func validateBookPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("请先选择 TXT 文件")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("找不到或无法访问这本书，请重新选择文件：%w", err)
	}
	if info.IsDir() {
		return errors.New("请选择 TXT 文件，不能选择文件夹")
	}
	return nil
}

func (a *webApp) saveConfig(s config.Settings) error {
	if err := reader.Flush(); err != nil {
		return fmt.Errorf("保存当前阅读位置失败：%w", err)
	}
	if err := validateBookPath(s.BookPath); err != nil {
		return err
	}
	s = config.Normalize(s)
	if err := hotkey.Validate(map[string]string{
		"上一页": s.Hotkeys.Up, "下一页": s.Hotkeys.Down, "隐藏": s.Hotkeys.Hide,
		"显示": s.Hotkeys.Show, "复位": s.Hotkeys.Move,
	}); err != nil {
		return err
	}
	before, err := config.Load()
	if err != nil {
		return err
	}
	next, err := config.PreviewPreferences(s)
	if err != nil {
		return err
	}
	a.started, err = commitPreferences(before, next, a.started, nativeReaderControls(), func(s config.Settings) error {
		_, saveErr := config.SavePreferences(s)
		return saveErr
	})
	if err != nil {
		return err
	}
	a.dispatch(func() { hideWindow(a.handle) })
	return nil
}

func nativeReaderControls() readerControls {
	return readerControls{
		start: func(s config.Settings) error { return reader.Start(s, nil) },
		apply: reader.ApplySettings,
		show:  reader.Show,
		seek:  reader.Seek,
		close: reader.Close,
	}
}

func (a *webApp) relocateBook(oldPath, newPath string) (configView, error) {
	if err := reader.Flush(); err != nil {
		return configView{}, err
	}
	if err := validateBookPath(newPath); err != nil {
		return configView{}, err
	}
	before, err := config.Load()
	if err != nil {
		return configView{}, err
	}
	next, err := config.PreviewRelocateBook(oldPath, newPath)
	if err != nil {
		return configView{}, err
	}
	active := a.started && (strings.EqualFold(filepath.Clean(before.BookPath), filepath.Clean(oldPath)) ||
		strings.EqualFold(filepath.Clean(before.BookPath), filepath.Clean(newPath)))
	if err := commitRelocation(before, next, active, nativeReaderControls(), func() error {
		_, saveErr := config.RelocateBook(oldPath, newPath)
		return saveErr
	}); err != nil {
		return configView{}, err
	}
	return a.getConfig()
}

func (a *webApp) showSettings(message string, chooseBook bool) {
	a.dispatch(func() {
		showWindow(a.handle, 9) // SW_RESTORE，同时恢复最小化窗口。
		user32.NewProc("SetForegroundWindow").Call(a.handle)
		data, _ := json.Marshal(map[string]any{"message": message, "choose_book": chooseBook})
		a.w.Eval("window.refreshSettings && window.refreshSettings(" + string(data) + ")")
	})
}

func (a *webApp) showReader() error {
	if !a.started {
		a.showSettings("请先选择一本书，开始阅读后可从托盘恢复。", false)
		return nil
	}
	return reader.Show()
}

func (a *webApp) closeSettings() {
	a.actionMu.Lock()
	started := a.started
	a.actionMu.Unlock()
	if started {
		a.dispatch(func() { hideWindow(a.handle) })
	} else {
		a.requestExit()
	}
}

func (a *webApp) requestExit() {
	if !a.quitting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		a.actionMu.Lock()
		defer a.actionMu.Unlock()
		// Close 负责把最终位置刷盘并关闭阅读窗口。失败时保留设置入口，避免静默丢进度。
		if err := reader.Close(); err != nil {
			a.quitting.Store(false)
			a.showSettings("退出时保存失败："+err.Error(), false)
			return
		}
		a.started = false
		a.dispatch(func() {
			a.tray.dispose()
			a.w.Terminate() // Terminate 调用 PostQuitMessage，必须在 WebView 线程执行。
		})
	}()
}

var user32 = windows.NewLazySystemDLL("user32.dll")

func hideWindow(hwnd uintptr) { showWindow(hwnd, 0) }

func showWindow(hwnd, cmd uintptr) {
	if hwnd != 0 {
		user32.NewProc("ShowWindow").Call(hwnd, cmd)
	}
}
