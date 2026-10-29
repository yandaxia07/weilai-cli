#!/usr/bin/env bash
# 构建与打包：五个平台的二进制 + 各自压缩包 + manifest.json（带 sha256）+ npm 包（WorkBuddy 托管 Node 运行时安装用）。
# 用法：bash scripts/build.sh <版本号，如 0.1.0>   产物在 dist/；发布见 scripts/release.sh。
set -euo pipefail
cd "$(dirname "$0")/.."
V="${1:?用法: build.sh <版本号>}"
GO="${GO:-go}"
rm -rf dist && mkdir -p dist/bin dist/npm/vendor
$GO vet ./...
$GO test ./...
targets="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"
for t in $targets; do
  os="${t%/*}"; arch="${t#*/}"; ext=""; [ "$os" = windows ] && ext=".exe"
  out="dist/bin/weilai-$os-$arch$ext"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch $GO build -trimpath -ldflags "-s -w -X main.version=$V" -o "$out" .
  cp "$out" "dist/npm/vendor/"
  if [ "$os" = windows ]; then
    (cd dist/bin && zip -q -j "weilai-$V-$os-$arch.zip" "weilai-$os-$arch$ext")
  else
    tar -C dist/bin -czf "dist/weilai-$V-$os-$arch.tar.gz" "weilai-$os-$arch"
  fi
done
mv dist/bin/*.zip dist/
# npm 包：bin/weilai.js 按平台挑 vendor 里的二进制
cp npm/bin/weilai.js dist/npm/
mkdir -p dist/npm/bin && mv dist/npm/weilai.js dist/npm/bin/
sed "s/__VERSION__/$V/" npm/package.json > dist/npm/package.json
cp README.md dist/npm/
(cd dist/npm && npm pack --silent >/dev/null && mv weilai-cli-$V.tgz ..)
# manifest：贝壳同款，安装脚本按它下载并强制校验 sha256
python3 - "$V" <<'PY'
import hashlib, json, os, sys
v = sys.argv[1]
base = f"https://github.com/yandaxia07/weilai-cli/releases/download/v{v}/"
files = {}
for f in sorted(os.listdir("dist")):
    if f.endswith((".tar.gz", ".zip", ".tgz")):
        files[f] = {"url": base + f, "sha256": hashlib.sha256(open(f"dist/{f}", "rb").read()).hexdigest(),
                    "size": os.path.getsize(f"dist/{f}")}
json.dump({"latest": v, "files": files}, open("dist/manifest.json", "w"), indent=2)
print(json.dumps({k: x["sha256"][:12] for k, x in files.items()}, indent=1))
PY
