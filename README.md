# 摸鱼联盟

Windows 桌面 TXT 阅读小工具：置顶细条窗口 + 全局热键，适合边工作边摸鱼看小说。

## 版本说明

本仓库包含两个实现：

| 目录 | 说明 | 技术栈 |
|------|------|--------|
| **根目录**（`摸鱼联盟.sln`） | 原版 | C# / WPF / .NET Framework |
| **[kanxiaoshuo-go/](kanxiaoshuo-go/)** | **推荐** | Go / WebView2 / Win32 |

### Go 版（推荐）

- **单文件 exe**，无需安装 .NET
- WebView2 配置界面，支持可见行数、字号、颜色
- 大文件分块阅读 + 块末/块首预读
- 透明背景、只读、可拖动、无边框

详见 **[kanxiaoshuo-go/README.md](kanxiaoshuo-go/README.md)**，含编译步骤、快捷键与配置说明。

### C# 版

- 使用 Visual Studio 打开 `摸鱼联盟.sln` 编译运行
- 配置保存在程序目录 `Config.ini`

## 快速使用（Go 版）

```powershell
cd kanxiaoshuo-go
go build -ldflags="-s -w -H windowsgui" -o 摸鱼联盟.exe .
.\摸鱼联盟.exe
```

或进入 `kanxiaoshuo-go` 双击 `build.bat` 后运行生成的 `摸鱼联盟.exe`。

## 仓库

https://github.com/Z18393520308/kanxiaoshuo
