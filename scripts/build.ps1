param([string]$OutputDirectory = "")
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$module = Join-Path $repo "kanxiaoshuo-go"
$version = (Get-Content (Join-Path $repo "VERSION") -Raw).Trim()
if ($version -notmatch '^\d+\.\d+\.\d+$') { throw "VERSION 格式错误" }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "请先安装 Go 1.21 或更高版本。" }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repo "dist" }
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
$name = "moyu-$version-windows-amd64"
$stage = Join-Path $OutputDirectory $name
New-Item -ItemType Directory -Force $stage | Out-Null

# 不运行 go mod tidy，不在打包时改动依赖锁文件。
Push-Location $module
$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    & go mod download
    if ($LASTEXITCODE -ne 0) { throw "依赖下载失败" }
    & go mod verify
    if ($LASTEXITCODE -ne 0) { throw "依赖校验失败" }
    & go test -mod=readonly ./...
    if ($LASTEXITCODE -ne 0) { throw "测试失败" }
    & go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "静态检查失败" }
    & go build -mod=readonly -trimpath -ldflags "-s -w -H windowsgui -X kanxiaoshuo-go/internal/version.Version=$version" -o (Join-Path $stage "摸鱼联盟.exe") .
    if ($LASTEXITCODE -ne 0) { throw "编译失败" }
} finally {
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
    $env:CGO_ENABLED = $oldCGO
    Pop-Location
}
Copy-Item (Join-Path $repo "docs/使用说明.md") (Join-Path $stage "使用说明.md")
Copy-Item (Join-Path $repo "docs/Windows验收.md") $stage
Copy-Item (Join-Path $repo "docs/验证记录.md") $stage
Copy-Item (Join-Path $repo "CHANGELOG.md") $stage
Copy-Item (Join-Path $repo "THIRD_PARTY_NOTICES.txt") $stage
Copy-Item (Join-Path $repo "VERSION") $stage
$exeHash = (Get-FileHash (Join-Path $stage "摸鱼联盟.exe") -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText((Join-Path $stage "SHA256SUMS.txt"), "$exeHash  摸鱼联盟.exe`n", (New-Object Text.UTF8Encoding $false))
$zipPath = Join-Path $OutputDirectory "$name.zip"
Compress-Archive -Path "$stage/*" -DestinationPath $zipPath -Force
$zipHash = (Get-FileHash $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText((Join-Path $OutputDirectory "$name.sha256"), "$zipHash  $name.zip`n", (New-Object Text.UTF8Encoding $false))
Write-Host "完成：$zipPath"
