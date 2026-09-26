# 摸鱼联盟开发说明

应用入口为 `main.go`，目标系统 Windows x64。设置界面由 WebView2 承载，阅读条使用原生 Win32 EDIT 控件，页面资源通过 `go:embed` 放入单个 EXE。

## 模块

| 路径 | 职责 |
| --- | --- |
| `internal/book` | 文本解码、统一文本与进度换算 |
| `internal/config` | 配置迁移、原子保存、独立书签、最近阅读 |
| `internal/hotkey` | 热键语法与重复检查 |
| `internal/reader` | 阅读条、精确分页、热键注册、窗口恢复 |
| `internal/ui` | 托盘、设置窗口、文件选择与前后端桥接 |
| `internal/version` | 应用版本 |

## 本地检查

```sh
go mod download
go mod verify
go test ./...
go vet ./...
```

前端测试不依赖 npm 包，安装 Node.js 18 或更高版本后执行：

```sh
node --test internal/ui/test/app.test.cjs
```

浏览器布局预览可运行 `node internal/ui/test/preview.cjs` 并访问 `http://127.0.0.1:9781`。
预览使用模拟数据，不会修改真实配置；托盘和阅读条仍需 Windows 程序验收。

在 macOS / Linux 上可以运行平台无关的核心测试；Windows 专用文件不会在这些平台执行。竞态检查可在支持 CGO 的环境运行：

```sh
go test -race ./...
```

Windows 交叉编译：

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags='-s -w -H windowsgui' -o ../dist/摸鱼联盟.exe .
```

交叉编译只验证代码能生成 Windows 二进制，不能代替真实 Windows 的托盘、WebView2、全局热键和多显示器验收。见 [Windows 验收清单](../docs/Windows验收.md)。

## 配置

路径为 `%AppData%\摸鱼联盟\config.json`。保存时在同一目录写临时文件并原子替换。配置损坏时应先保留原文件排查，不要自动覆盖。

- `schema_version`：当前配置结构为 2。
- `book_path` / `position_bytes`：当前书与规范化文本的 UTF-8 字节偏移。
- `recent_books`：每本书的路径、独立位置及最近阅读时间。
- `font_family` / `font_size` / `font_color` / `visible_lines` / `window_width`：阅读样式。
- `window_left` / `window_top`：阅读条窗口位置。
- `hotkeys`：`up`、`down`、`hide`、`show`、`move`。
- 旧 `char_index` / `line_index` 仅用于兼容迁移，成功保存新进度后清零。

新旧版本同时运行、手工改写正在阅读的 TXT 内容都可能使书签所指文字改变。阅读位置基于文本，不跟踪外部编辑。

## 发布准备

以根目录 `VERSION` 和 `internal/version/version.go` 为版本入口，更新 `CHANGELOG.md` 后运行 `scripts/build.ps1`。版本采用 `0.x.y`，标签格式 `v0.x.y`。

工作流对提交与 PR 执行检查，上传 ZIP 和校验文件；标签必须与 `VERSION` 一致。推送标签或创建公开 Release 需要单独确认发布目标。本仓库未包含代码签名证书。
