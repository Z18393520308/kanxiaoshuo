# 摸鱼联盟（Go 版）

原 WPF 项目的 Go 重写：**单个 exe、无需安装 .NET**。配置保存在 `%AppData%\摸鱼联盟\config.json`，分发时只需拷贝可执行文件。

## 功能

- 配置窗：选择 TXT、字号（3–24）、五种预设颜色
- 阅读条：透明置顶细条窗口，可拖动
- 全局热键：
  - `↑` / `↓` 翻行（记忆进度）
  - `Alt+C` 隐藏阅读条
  - `Alt+S` 显示阅读条
  - `Alt+T` 窗口移到 (100, 100)

## 环境要求

- Windows 10/11
- [Go 1.21+](https://go.dev/dl/)（已可通过 `winget install GoLang.Go` 安装）

## 编译

```powershell
cd kanxiaoshuo-go
go mod tidy
go build -ldflags="-s -w -H windowsgui" -o 摸鱼联盟.exe .
```

或直接双击 `build.bat`。

生成单个 `摸鱼联盟.exe`（约 7MB，含 WebView2 壳），**无需 .NET**，双击即可运行。需系统已安装 [WebView2 运行时](https://developer.microsoft.com/microsoft-edge/webview2/)（Win10/11 通常已自带）。

## 与 C# 版的差异

| 项目 | C# 版 | Go 版 |
|------|-------|-------|
| 运行时 | .NET Framework | 无，原生 exe |
| 配置 | 程序目录 `Config.ini` | `%AppData%\摸鱼联盟\config.json` |
| 字体颜色 | 未应用到阅读条 | 已应用到阅读条 |
| 依赖文件 | 多文件 | 单 exe |

## 目录结构

```
kanxiaoshuo-go/
  main.go
  internal/
    config/   # JSON 配置
    book/     # 读取 TXT（UTF-8 / GBK）
    reader/   # Win32 阅读条 + 热键
    ui/webui/ # 配置界面 HTML/CSS（极简风格）
    ui/       # WebView2 壳 + 文件对话框
```
