#!/bin/bash
# build-dmg.sh —— 生成可分发的 macOS 安装镜像（DMG）
# 产出 `dist/BrowserSwitch-<version>.dmg`：打开后拖拽 App 到 Applications 即安装，
# 与 Chrome / VS Code 等正式软件一致的体验。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

APP_NAME="Browser Switch"
VOL_NAME="Browser Switch"
VERSION="${VERSION:-1.0.0}"
DIST_DIR="$ROOT_DIR/dist"
APP_PATH="$DIST_DIR/$APP_NAME.app"
DMG_PATH="$DIST_DIR/BrowserSwitch-$VERSION.dmg"
STAGE_DIR="$DIST_DIR/dmg-stage"
ICON="$ROOT_DIR/assets/BrowserSwitch.icns"

# 1) 确保 .app 已构建
if [[ ! -d "$APP_PATH" ]]; then
  echo "==> 未找到 ${APP_PATH}，先执行 build-app.sh"
  "$SCRIPT_DIR/build-app.sh"
fi

echo "==> 准备暂存目录"
rm -rf "$STAGE_DIR" "$DMG_PATH"
mkdir -p "$STAGE_DIR"
# 复制 App 并建立指向 /Applications 的软链，供用户拖拽安装
cp -R "$APP_PATH" "$STAGE_DIR/"
ln -s /Applications "$STAGE_DIR/Applications"

# 2) 生成可读写的临时 DMG（便于设置窗口布局），随后转为压缩只读发布版
TMP_DMG="$DIST_DIR/.tmp-rw.dmg"
rm -f "$TMP_DMG"
echo "==> 创建可读写镜像"
hdiutil create -srcfolder "$STAGE_DIR" -volname "$VOL_NAME" \
  -fs HFS+ -format UDRW -ov "$TMP_DMG" >/dev/null

echo "==> 挂载镜像布局窗口"
# 先卸载可能残留的同名卷，避免 /Volumes 命名冲突
hdiutil detach "/Volumes/$VOL_NAME" >/dev/null 2>&1 || true
# 默认挂到 /Volumes/<卷名>，Finder 才能按卷名寻址布局
ATTACH_OUT="$(hdiutil attach "$TMP_DMG" -readwrite -noverify -noautoopen)"
MOUNT_DIR="$(echo "$ATTACH_OUT" | grep -oE '/Volumes/[^[:cntrl:]]+' | tail -1)"
if [[ -z "$MOUNT_DIR" || ! -d "$MOUNT_DIR" ]]; then
  MOUNT_DIR="/Volumes/$VOL_NAME"
fi
echo "    挂载于: $MOUNT_DIR"
sleep 1

# 3) 用 Finder 设置图标位置、窗口大小（拖拽引导视觉）
#    图标：左侧 App、右侧 Applications，中间形成"拖过去"的直觉。
osascript <<APPLESCRIPT || true
tell application "Finder"
  tell disk "$VOL_NAME"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set the bounds of container window to {200, 120, 760, 520}
    set theViewOptions to the icon view options of container window
    set arrangement of theViewOptions to not arranged
    set icon size of theViewOptions to 128
    set position of item "$APP_NAME.app" of container window to {150, 200}
    set position of item "Applications" of container window to {410, 200}
    update without registering applications
    delay 1
    close
  end tell
end tell
APPLESCRIPT

# 4) 设置卷图标（可选，失败不阻断）
if [[ -f "$ICON" ]]; then
  cp "$ICON" "$MOUNT_DIR/.VolumeIcon.icns" 2>/dev/null || true
  SetFile -a C "$MOUNT_DIR" 2>/dev/null || true
fi

sync
echo "==> 卸载镜像"
hdiutil detach "$MOUNT_DIR" >/dev/null 2>&1 || hdiutil detach "$MOUNT_DIR" -force >/dev/null 2>&1 || true

# 5) 转换为压缩只读发布镜像
echo "==> 生成压缩发布镜像"
hdiutil convert "$TMP_DMG" -format UDZO -imagekey zlib-level=9 -ov -o "$DMG_PATH" >/dev/null
rm -f "$TMP_DMG"
rm -rf "$STAGE_DIR"

echo "==> 完成: $DMG_PATH"
du -sh "$DMG_PATH" | awk '{print "    体积:", $1}'
