#!/usr/bin/env bash
# 未来员工命令行安装（macOS / Linux）：读 manifest.json → 下载本平台压缩包 → 强制校验 sha256 → 装到 ~/.local/bin/weilai。
#   curl -fsSL https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/install.sh | bash
# 下载源：先用国内镜像 luluyouai.com，失败再回退 GitHub；两边同一批文件，sha256 一律按 manifest 强制校验。
# 可选环境变量：WEILAI_INSTALL_DIR（默认 ~/.local/bin）、WEILAI_MANIFEST_URL（指定后只用它，不回退）
set -euo pipefail
MIRROR="https://luluyouai.com/downloads/weilai-cli"
GITHUB_MANIFEST="https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/releases/manifest.json"
MANIFEST_URL="${WEILAI_MANIFEST_URL:-$MIRROR/manifest.json}"
DIR="${WEILAI_INSTALL_DIR:-$HOME/.local/bin}"
fatal() { echo "weilai 安装失败：$*" >&2; exit 1; }
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) fatal "不支持的系统 $(uname -s)，Windows 请用 install.ps1" ;; esac
case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) fatal "不支持的架构 $(uname -m)" ;; esac
command -v curl >/dev/null || fatal "需要 curl"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL --connect-timeout 8 --max-time 30 "$MANIFEST_URL" -o "$tmp/manifest.json" \
  || { [ -z "${WEILAI_MANIFEST_URL:-}" ] && curl -fsSL --connect-timeout 8 --max-time 30 "$GITHUB_MANIFEST" -o "$tmp/manifest.json"; } \
  || fatal "下载 manifest 失败（国内镜像与 GitHub 都不可用）"
# manifest 是 scripts/build.sh 写的缩进 JSON：按文件名那一段取 url 与 sha256（不依赖 python / jq）
ver=$(sed -n 's/.*"latest": *"\([^"]*\)".*/\1/p' "$tmp/manifest.json" | head -1)
[ -n "$ver" ] || fatal "manifest 格式不对"
block=$(awk -v f="\"weilai-$ver-$os-$arch.tar.gz\"" 'index($0, f) {on=1} on {print} on && /}/ {exit}' "$tmp/manifest.json")
url=$(printf '%s\n' "$block" | sed -n 's/.*"url": *"\([^"]*\)".*/\1/p')
sum=$(printf '%s\n' "$block" | sed -n 's/.*"sha256": *"\([0-9a-f]*\)".*/\1/p')
[ -n "${sum:-}" ] && [ ${#sum} -eq 64 ] || fatal "manifest 里没有本平台（$os-$arch）的有效 sha256"
if ! curl -fsSL --connect-timeout 8 --max-time 300 "$url" -o "$tmp/w.tar.gz"; then
  # 首选地址下载不动时回退 GitHub release 同名文件（sha256 仍按 manifest 校验）
  curl -fsSL --connect-timeout 8 "https://github.com/yandaxia07/weilai-cli/releases/download/v$ver/${url##*/}" -o "$tmp/w.tar.gz" \
    || fatal "下载失败 $url"
fi
if command -v shasum >/dev/null; then got=$(shasum -a 256 "$tmp/w.tar.gz" | cut -d' ' -f1); else got=$(sha256sum "$tmp/w.tar.gz" | cut -d' ' -f1); fi
[ "$got" = "$sum" ] || fatal "sha256 校验不通过（期望 $sum，实际 $got）"
tar -xzf "$tmp/w.tar.gz" -C "$tmp"
mkdir -p "$DIR"
install -m 0755 "$tmp/weilai-$os-$arch" "$DIR/weilai"
echo "已安装 weilai $ver 到 $DIR/weilai"
case ":$PATH:" in *":$DIR:"*) ;; *) echo "提示：把 $DIR 加入 PATH，例如 echo 'export PATH=\"$DIR:\$PATH\"' >> ~/.zshrc" ;; esac
echo "下一步：weilai auth login（微信扫码领取 API key）"
