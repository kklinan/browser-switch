//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// browserIconPath 提取浏览器 .app 的图标为 PNG（缓存到临时目录），返回路径。
func browserIconPath(b Browser) string {
	appPath := b.Desktop // darwin 上 Desktop 存的是 .app 包路径
	if appPath == "" || !strings.HasSuffix(appPath, ".app") {
		return ""
	}
	cache := filepath.Join(os.TempDir(), "browser-picker-icons")
	_ = os.MkdirAll(cache, 0755)
	out := filepath.Join(cache, strings.ReplaceAll(b.ID, "/", "_")+".png")
	if fi, err := os.Stat(out); err == nil && fi.Size() > 0 {
		return out // 已缓存
	}

	plist := filepath.Join(appPath, "Contents", "Info.plist")
	raw, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleIconFile", "raw", "-o", "-", plist).Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(raw))
	if name == "" {
		return ""
	}
	if !strings.HasSuffix(name, ".icns") {
		name += ".icns"
	}
	icns := filepath.Join(appPath, "Contents", "Resources", name)
	if err := exec.Command("/usr/bin/sips", "-s", "format", "png", "-Z", "128", icns, "--out", out).Run(); err != nil {
		return ""
	}
	return out
}
