#!/bin/bash
# build-app.sh —— 构建期打包脚本
# 产出可分发的 `dist/Browser Switch.app`（编译二进制 + Info.plist + 图标 + ad-hoc 签名）。
# 与运行时自建 bundle 复用同一份打包逻辑（--build-bundle），保证结构一致。
set -euo pipefail

# 定位仓库根目录（脚本位于 scripts/ 下）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="Browser Switch"
DIST_DIR="$ROOT_DIR/dist"
APP_PATH="$DIST_DIR/$APP_NAME.app"
BUILD_BIN="$DIST_DIR/browser-switch"
ICON="$ROOT_DIR/assets/BrowserSwitch.icns"

echo "==> 清理旧产物"
rm -rf "$APP_PATH" "$BUILD_BIN"
mkdir -p "$DIST_DIR"

echo "==> 编译发布二进制（CGO + 去符号）"
CGO_ENABLED=1 go build -ldflags="-s -w" -o "$BUILD_BIN" .

echo "==> 打包 .app bundle"
# 用刚编译出的二进制自身构建 bundle：--build-bundle 无头模式，
# 会把该二进制、指定图标、Info.plist 组装成完整 .app 并 ad-hoc 签名。
BP_BUNDLE_ICON="$ICON" "$BUILD_BIN" --build-bundle "$APP_PATH"

echo "==> 校验签名"
codesign --verify --deep --strict "$APP_PATH" 2>&1 || {
  echo "!! 签名校验失败" >&2; exit 1;
}

echo "==> 完成: $APP_PATH"
du -sh "$APP_PATH" | awk '{print "    体积:", $1}'
