#!/bin/bash
# build-app.sh —— 构建期打包脚本
# 产出可分发的 `dist/<arch>/Browser Switch.app`（编译二进制 + Info.plist + 图标 + ad-hoc 签名）。
# 支持架构: amd64 / arm64 / universal（缺省 = 当前主机架构）
# 用法:
#   ./scripts/build-app.sh            # 当前主机架构
#   ./scripts/build-app.sh amd64      # Intel (Mac x64)
#   ./scripts/build-app.sh arm64      # Apple Silicon (Mac ARM64)
#   ./scripts/build-app.sh universal  # 通用包（lipo 合并，同时兼容两种架构）
# 与运行时自建 bundle 复用同一份打包逻辑（--build-bundle），保证结构一致。
#
# 注意: 本项目依赖 CGO（CoreServices / CoreFoundation / Carbon）。
#   - 在 Apple Silicon 上交叉编译 amd64 需安装 Rosetta 2（softwareupdate --install-rosetta）
#   - 在 Intel Mac 上交叉编译 arm64 需较新 Xcode（含通用 SDK）
set -euo pipefail

# 定位仓库根目录（脚本位于 scripts/ 下）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# ---- 解析架构 ----
ARCH_ARG="${1:-auto}"
case "$ARCH_ARG" in
  auto)
    case "$(uname -m)" in
      x86_64) GOARCH="amd64" ;;
      arm64)  GOARCH="arm64" ;;
      *) echo "不支持的主机架构: $(uname -m)"; exit 1 ;;
    esac
    ;;
  x64|amd64|x86_64) GOARCH="amd64" ;;
  arm64|aarch64)    GOARCH="arm64" ;;
  universal|fat|both) GOARCH="universal" ;;
  *) echo "未知架构: $ARCH_ARG（支持: amd64 / arm64 / universal）"; exit 1 ;;
esac
echo "==> 目标架构: $GOARCH"

APP_NAME="Browser Switch"
DIST_DIR="$ROOT_DIR/dist"
ARCH_DIR="$DIST_DIR/$GOARCH"
APP_PATH="$ARCH_DIR/$APP_NAME.app"
BUILD_BIN="$ARCH_DIR/browser-switch"
ICON="$ROOT_DIR/assets/BrowserSwitch.icns"

echo "==> 清理旧产物"
rm -rf "$APP_PATH"
mkdir -p "$ARCH_DIR"

# 编译指定架构的发布二进制（CGO + 去符号）
build_single() {
  local arch="$1" out="$2"
  echo "==> 编译 $arch 二进制"
  CGO_ENABLED=1 GOARCH="$arch" go build -ldflags="-s -w" -o "$out" .
}

if [[ "$GOARCH" == "universal" ]]; then
  # ---- 通用包：分别编译两种架构后 lipo 合并 ----
  TMP_AMD64="$DIST_DIR/.browser-switch-amd64-$$"
  TMP_ARM64="$DIST_DIR/.browser-switch-arm64-$$"
  trap 'rm -f "$TMP_AMD64" "$TMP_ARM64"' EXIT
  build_single amd64 "$TMP_AMD64"
  build_single arm64 "$TMP_ARM64"
  echo "==> lipo 合并为通用二进制"
  lipo -create "$TMP_AMD64" "$TMP_ARM64" -output "$BUILD_BIN"
  rm -f "$TMP_AMD64" "$TMP_ARM64"
  trap - EXIT
  file "$BUILD_BIN"
else
  build_single "$GOARCH" "$BUILD_BIN"
fi

# ---- 打包 .app bundle ----
# 用二进制自身执行 --build-bundle 无头模式（组装 bundle + Info.plist + 图标 + ad-hoc 签名）。
# 目标架构二进制在本机可运行时直接用它打包；
# 交叉编译时（如 Intel 主机打 arm64 包）目标二进制无法执行，改用本机架构的
# 临时打包器 + BP_BUNDLE_BIN 指向目标二进制完成打包，保证 bundle 内是目标架构二进制。
HOST_GOARCH="$(uname -m)"
[[ "$HOST_GOARCH" == "x86_64" ]] && HOST_GOARCH="amd64"
PACKAGER=""
pack_bundle() {
  BP_BUNDLE_ICON="$ICON" BP_BUNDLE_BIN="${BP_BUNDLE_BIN:-}" "$1" --build-bundle "$APP_PATH"
}

if [[ "$GOARCH" == "$HOST_GOARCH" || "$GOARCH" == "universal" ]]; then
  echo "==> 打包 .app bundle（使用目标二进制）"
  unset BP_BUNDLE_BIN
  pack_bundle "$BUILD_BIN"
else
  echo "==> 交叉编译场景：编译本机($HOST_GOARCH)打包器"
  PACKAGER="$DIST_DIR/.browser-switch-packager-$HOST_GOARCH-$$"
  CGO_ENABLED=1 GOARCH="$HOST_GOARCH" go build -ldflags="-s -w" -o "$PACKAGER" .
  trap 'rm -f "$PACKAGER"' EXIT
  BP_BUNDLE_BIN="$BUILD_BIN" pack_bundle "$PACKAGER"
  rm -f "$PACKAGER"
  trap - EXIT
fi

echo "==> 校验签名"
# 等待签名 seal 落盘，避免紧随签名后校验的偶发时序失败；失败时自动重试一次
sleep 1
if ! codesign --verify --deep --strict "$APP_PATH" 2>&1; then
  echo "!! 首次校验失败，2s 后重试" >&2
  sleep 2
  codesign --verify --deep --strict "$APP_PATH" 2>&1 || {
    echo "!! 签名校验失败" >&2; exit 1;
  }
fi

echo "==> 完成: $APP_PATH"
du -sh "$APP_PATH" | awk '{print "    体积:", $1}'
