#!/usr/bin/env bash
# 发布：构建 → 打 tag → gh release 上传全部产物、manifest.json 与安装脚本 → 更新仓库 releases/manifest.json
# → 自动同步国内镜像 luluyouai.com/downloads/weilai-cli/（商拍仓库 scripts/deploy-weilai-mirror.sh，公网逐个核对 sha256）。
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
gh release create "v$V" -R yandaxia07/weilai-cli -t "weilai $V" -n "未来员工命令行 $V。安装：curl -fsSL https://luluyouai.com/downloads/weilai-cli/install.sh | bash（国内镜像，GitHub 备用）" \
  dist/*.tar.gz dist/*.zip dist/*.tgz dist/manifest.json install.sh install.ps1
echo "GitHub 已发布 v$V，同步国内镜像…"
MIRROR="${WEILAI_MIRROR_DEPLOY:-$HOME/code-hub/deploy/shangpaizhushou/scripts/deploy-weilai-mirror.sh}"
[ -f "$HOME/.servers/env.sh" ] && . "$HOME/.servers/env.sh" >/dev/null 2>&1
bash "$MIRROR" "$V" || { echo "!! 国内镜像同步失败：GitHub 已发布，但国内用户可能装不上。修好后重跑：bash $MIRROR $V" >&2; exit 1; }
echo "已发布 v$V（GitHub + 国内镜像）"
