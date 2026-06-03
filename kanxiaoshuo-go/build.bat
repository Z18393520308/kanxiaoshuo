@echo off
chcp 65001 >nul
cd /d "%~dp0"
where go >nul 2>&1 || (
  echo 未检测到 Go，请先安装: https://go.dev/dl/
  pause
  exit /b 1
)
go mod tidy
go build -ldflags="-s -w -H windowsgui" -o 摸鱼联盟.exe .
if %errorlevel%==0 (
  echo 编译成功: 摸鱼联盟.exe
) else (
  echo 编译失败
)
pause
