# 未来员工命令行安装（Windows）：先用国内镜像、失败回退 GitHub；读 manifest.json → 下载 zip → 强制校验 sha256 → 装到 %USERPROFILE%\.weilai\bin\weilai.exe
#   irm https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'
$mirror = 'https://luluyouai.com/downloads/weilai-cli'
$manifestUrl = if ($env:WEILAI_MANIFEST_URL) { $env:WEILAI_MANIFEST_URL } else { "$mirror/manifest.json" }
$dir = if ($env:WEILAI_INSTALL_DIR) { $env:WEILAI_INSTALL_DIR } else { Join-Path $env:USERPROFILE '.weilai\bin' }
# 先读国内镜像，读不到回退 GitHub（同一批文件、同样的 sha256）
try { $m = Invoke-RestMethod $manifestUrl -TimeoutSec 30 }
catch { if ($env:WEILAI_MANIFEST_URL) { throw } ; $m = Invoke-RestMethod 'https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/releases/manifest.json' -TimeoutSec 30 }
$v = $m.latest
$name = "weilai-$v-windows-amd64.zip"
$f = $m.files.$name
if (-not $f -or $f.sha256.Length -ne 64) { throw "manifest 里没有 Windows 版本的有效 sha256" }
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("weilai-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
$zip = Join-Path $tmp $name
try { Invoke-WebRequest $f.url -OutFile $zip -UseBasicParsing -TimeoutSec 60 }
catch { Invoke-WebRequest "https://github.com/yandaxia07/weilai-cli/releases/download/v$v/$name" -OutFile $zip -UseBasicParsing }
$got = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
if ($got -ne $f.sha256) { throw "sha256 校验不通过（期望 $($f.sha256)，实际 $got）" }
Expand-Archive $zip -DestinationPath $tmp -Force
New-Item -ItemType Directory -Path $dir -Force | Out-Null
Copy-Item (Join-Path $tmp 'weilai-windows-amd64.exe') (Join-Path $dir 'weilai.exe') -Force
Remove-Item $tmp -Recurse -Force
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$dir*") { [Environment]::SetEnvironmentVariable('Path', "$dir;$userPath", 'User') }
Write-Host "已安装 weilai $v 到 $dir\weilai.exe（新开终端生效）。下一步：weilai auth login"
