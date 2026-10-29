#!/usr/bin/env bash
# 发布：构建 → 打 tag → gh release 上传全部产物与 manifest.json → 更新仓库里的 releases/manifest.json（安装脚本读它）。
# 用法：bash scripts/release.sh <版本号>（在 NAS 工作目录执行，gh 已登录 yandaxia07）
set -euo pipefail
cd "$(dirname "$0")/.."
V="${1:?用法: release.sh <版本号>}"
bash scripts/build.sh "$V"
mkdir -p releases && cp dist/manifest.json releases/manifest.json
git add releases/manifest.json && { git diff --cached --quiet || git commit -q -m "release: v$V" -- releases/manifest.json; }
git tag -f "v$V"
git push -q origin HEAD:main && git push -q -f origin "v$V"
git push -q github HEAD:main && git push -q -f github "v$V"
gh release delete "v$V" -R yandaxia07/weilai-cli -y >/dev/null 2>&1 || true
gh release create "v$V" -R yandaxia07/weilai-cli -t "weilai $V" -n "未来员工命令行 $V。安装：curl -fsSL https://raw.githubusercontent.com/yandaxia07/weilai-cli/main/install.sh | bash" \
  dist/*.tar.gz dist/*.zip dist/*.tgz dist/manifest.json
echo "已发布 v$V"
