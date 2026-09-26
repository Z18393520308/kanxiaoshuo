@echo off
chcp 65001 >nul
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0..\scripts\build.ps1"
if errorlevel 1 (
  echo 构建失败，请检查上方错误。
  pause
  exit /b 1
)
echo 安装包已生成到仓库根目录 dist。
pause
