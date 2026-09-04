//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#cgo CFLAGS: -Wno-deprecated-declarations
#include <CoreServices/CoreServices.h>
#include <stdlib.h>

static int bp_setDefaultHandler(const char* scheme, const char* bundleID) {
    CFStringRef s = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
    CFStringRef b = CFStringCreateWithCString(NULL, bundleID, kCFStringEncodingUTF8);
    OSStatus st = LSSetDefaultHandlerForURLScheme(s, b);
    CFRelease(s);
    CFRelease(b);
    return (int)st;
}

static char* bp_copyDefaultHandler(const char* scheme) {
    CFStringRef s = CFStringCreateWithCString(NULL, scheme, kCFStringEncodingUTF8);
    CFStringRef h = LSCopyDefaultHandlerForURLScheme(s);
    CFRelease(s);
    if (h == NULL) return NULL;
    CFIndex maxLen = CFStringGetMaximumSizeForEncoding(CFStringGetLength(h), kCFStringEncodingUTF8) + 1;
    char* buf = (char*)malloc(maxLen);
    if (!CFStringGetCString(h, buf, maxLen, kCFStringEncodingUTF8)) {
        free(buf); CFRelease(h); return NULL;
    }
    CFRelease(h);
    return buf;
}
*/
import "C"

const (
	appName        = AppName
	appBundleID    = AppBundleID
	helperName     = HelperName
	helperBundleID = HelperBundleID
	helperExecName = HelperExecName
)

type InstallInfo struct {
	ExecPath  string
	MainApp   string // 单 App：既是 URL 处理器也是 GUI
	Installed bool
}

func userApplicationsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Applications")
}

func GetInstallInfo() (*InstallInfo, error) {
	execPath, err := os.Executable()
	if err != nil {
		execPath, _ = filepath.Abs(os.Args[0])
	}
	appsDir := userApplicationsDir()
	info := &InstallInfo{
		ExecPath: execPath,
		MainApp:  filepath.Join(appsDir, appName+".app"),
	}
	if _, err := os.Stat(info.MainApp); err == nil {
		info.Installed = true
	}
	return info, nil
}

// cleanupLegacyApps 清理旧的双 App 架构残留（升级到单 App 方案时）。
func cleanupLegacyApps() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacyHelper := filepath.Join(home, "Applications", helperName+".app")
	_ = os.RemoveAll(legacyHelper)
}

func Install() error {
	// 清理旧的双 App 残留（升级场景）。
	cleanupLegacyApps()

	// 若当前进程本身就运行在一个正式打包的 .app 内（如 DMG 拖拽安装到
	// /Applications 后启动），只做注册与默认浏览器设置，绝不再自建副本。
	if bundle := runningAppBundlePath(); bundle != "" {
		// 移除可能残留的 ~/Applications 旧自建副本，避免同 bundleID 冲突。
		stale := filepath.Join(userApplicationsDir(), appName+".app")
		if !samePath(stale, bundle) {
			_ = os.RemoveAll(stale)
		}
		return registerAsDefault(bundle)
	}

	// 开发 / 裸二进制模式：把当前可执行文件打包进 ~/Applications。
	info, err := GetInstallInfo()
	if err != nil {
		return fmt.Errorf("get install info: %w", err)
	}
	if err := os.MkdirAll(userApplicationsDir(), 0755); err != nil {
		return fmt.Errorf("create Applications: %w", err)
	}
	if err := buildMainApp(info.MainApp, info.ExecPath); err != nil {
		return fmt.Errorf("build main app: %w", err)
	}
	return registerAsDefault(info.MainApp)
}

// registerAsDefault 对指定 bundle 完成 ad-hoc 签名、LaunchServices 注册，
// 记录安装前默认浏览器，并把自己设为 http/https 默认处理器。
func registerAsDefault(bundlePath string) error {
	adhocSign(bundlePath)
	registerWithLaunchServices(bundlePath)

	// 设置自己为默认前，记录安装前的系统默认浏览器，供卸载还原。
	// 仅在尚未记录、且当前默认不是 Switch 自己时记录，避免重复安装覆盖成 Switch。
	rememberPreviousDefault()

	for _, scheme := range []string{"http", "https"} {
		_ = setDefaultHandlerForScheme(scheme, appBundleID)
	}
	if ok, _ := CheckDefaultBrowser(); !ok {
		openDefaultBrowserSettings()
	}
	return nil
}

// runningAppBundlePath 若当前可执行文件位于 `XXX.app/Contents/MacOS/` 结构内，
// 返回该 .app 的路径；否则返回空串（裸二进制运行）。
func runningAppBundlePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	macOSDir := filepath.Dir(exe)          // .../Contents/MacOS
	contentsDir := filepath.Dir(macOSDir)  // .../Contents
	appDir := filepath.Dir(contentsDir)    // .../XXX.app
	if filepath.Base(macOSDir) == "MacOS" && filepath.Base(contentsDir) == "Contents" && strings.HasSuffix(appDir, ".app") {
		return appDir
	}
	return ""
}

// samePath 比较两个路径的清理后绝对形式是否指向同一位置。
func samePath(a, b string) bool {
	ca, _ := filepath.Abs(filepath.Clean(a))
	cb, _ := filepath.Abs(filepath.Clean(b))
	return ca == cb
}

// BuildAppBundle 在指定路径构建一个完整的、可分发的 .app（仅打包 + ad-hoc 签名，
// 不做 LaunchServices 注册，也不改动系统默认浏览器）。供构建期打包脚本调用。
func BuildAppBundle(appPath string) error {
	if err := os.MkdirAll(filepath.Dir(appPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	// 构建期交叉打包：BP_BUNDLE_BIN 可显式指定要打包进 .app 的源二进制，
	// 缺省为自身。跨架构打包时（如 Intel 主机打 arm64 包）目标二进制无法
	// 在本机运行，需由本机架构的打包器配合该变量完成打包。
	if src := os.Getenv("BP_BUNDLE_BIN"); src != "" {
		if _, statErr := os.Stat(src); statErr == nil {
			exe = src
		}
	}
	if err := buildMainApp(appPath, exe); err != nil {
		return fmt.Errorf("build bundle: %w", err)
	}
	adhocSign(appPath)
	return nil
}

// rememberPreviousDefault 在安装前把系统当前默认浏览器记入配置，供卸载时还原。
func rememberPreviousDefault() {
	cfg, err := InitConfig()
	if err != nil {
		return
	}
	if cfg.PrevDefaultBrowser != "" {
		return // 已记录过，不覆盖
	}
	current := defaultHandlerForScheme("http")
	current = strings.ToLower(current)
	// 当前默认已是 Switch 自己则不记录（否则卸载会还原成 Switch，毫无意义）。
	if current == "" || current == strings.ToLower(appBundleID) {
		return
	}
	cfg.PrevDefaultBrowser = current
	_ = SaveConfig(cfg)
}

func openDefaultBrowserSettings() {
	_ = exec.Command("/usr/bin/open", "x-apple.systempreferences:com.apple.preference.general").Run()
}

func Uninstall() error {
	info, _ := GetInstallInfo()

	// 还原安装前的默认浏览器：优先用记录值，无记录时回退 Safari。
	restore := "com.apple.safari"
	if cfg, err := InitConfig(); err == nil && cfg.PrevDefaultBrowser != "" {
		restore = cfg.PrevDefaultBrowser
	}
	for _, scheme := range []string{"http", "https"} {
		_ = setDefaultHandlerForScheme(scheme, restore)
	}

	// 清除记录，保证下次安装能重新捕获。
	if cfg, err := InitConfig(); err == nil {
		cfg.PrevDefaultBrowser = ""
		_ = SaveConfig(cfg)
	}

	// 删除主 App，并清理可能存在的旧双 App 残留。
	cleanupLegacyApps()
	var firstErr error
	if info.MainApp != "" {
		if err := os.RemoveAll(info.MainApp); err != nil {
			firstErr = err
		}
	}
	return firstErr
}

// SetSystemDefaultBrowser 将系统默认浏览器设为指定 bundleID（供设置界面"选择其他浏览器为默认"使用）。
func SetSystemDefaultBrowser(bundleID string) error {
	// 任一 scheme 失败即返回：更改默认浏览器会触发 macOS 系统确认弹窗，
	// 继续对下一个 scheme 调用会重复弹窗；且 http/https 由系统作为"默认网页浏览器"整体处理。
	for _, scheme := range []string{"http", "https"} {
		if err := setDefaultHandlerForScheme(scheme, bundleID); err != nil {
			return err
		}
	}
	return nil
}

func CheckDefaultBrowser() (bool, string) {
	current := defaultHandlerForScheme("http")
	if current == "" {
		return false, "unknown"
	}
	return current == appBundleID, current
}

func buildMainApp(appPath, srcExec string) error {
	// 关键：如果当前进程本身就在 appPath 里运行（例如从已安装的 .app 内启动），
	// 不能先 RemoveAll(appPath) 再读 srcExec —— 此时 self-bundle 已被删除，源文件句柄就失效了。
	// 必须先打开源文件拿到 reader，再删旧目录。
	src, err := os.Open(srcExec)
	if err != nil {
		return fmt.Errorf("open source executable: %w", err)
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat source executable: %w", err)
	}

	_ = os.RemoveAll(appPath)
	macOSDir := filepath.Join(appPath, "Contents", "MacOS")
	if err := os.MkdirAll(macOSDir, 0755); err != nil {
		return err
	}
	dstExec := filepath.Join(macOSDir, helperExecName)
	if err := copyReader(src, dstExec, srcInfo.Mode()&0755|0755); err != nil {
		return fmt.Errorf("copy executable: %w", err)
	}
	// 单 App：以主 App 身份注册，声明 http/https URL 类型（isBrowser=true），
	// 使 macOS 通过 Apple Event 将 URL 投递给本进程。
	// 图标（资源）仅在确实存在时写入；若无图标则完全不创建 Resources 目录，
	// 避免空资源目录被 codesign seal 后校验时报 "code has no resources"。
	iconSrc := getIconPath()
	if iconSrc != "" && filepath.Ext(iconSrc) == ".icns" {
		resourcesDir := filepath.Join(appPath, "Contents", "Resources")
		if err := os.MkdirAll(resourcesDir, 0755); err == nil {
			if err := copyFile(iconSrc, filepath.Join(resourcesDir, "AppIcon.icns"), 0644); err == nil {
				plist := buildInfoPlistWithIcon(helperExecName, appBundleID, appName, true)
				return os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), []byte(plist), 0644)
			}
		}
	}
	plist := buildInfoPlist(helperExecName, appBundleID, appName, true)
	return os.WriteFile(filepath.Join(appPath, "Contents", "Info.plist"), []byte(plist), 0644)
}

func buildInfoPlist(execName, bundleID, name string, isBrowser bool) string {
	urlTypes := ""
	if isBrowser {
		urlTypes = `	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLName</key>
			<string>Web URL</string>
			<key>CFBundleURLSchemes</key>
			<array>
				<string>http</string>
				<string>https</string>
			</array>
		</dict>
	</array>
`
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>English</string>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0.1</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>10.14</string>
	<key>NSHighResolutionCapable</key>
	<string>True</string>
%s</dict>
</plist>
`, execName, bundleID, name, urlTypes)
}

func buildInfoPlistWithIcon(execName, bundleID, name string, isBrowser bool) string {
	urlTypes := ""
	if isBrowser {
		urlTypes = `	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLName</key>
			<string>Web URL</string>
			<key>CFBundleURLSchemes</key>
			<array>
				<string>http</string>
				<string>https</string>
			</array>
		</dict>
	</array>
`
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>English</string>
	<key>CFBundleExecutable</key>
	<string>%s</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0.1</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>LSMinimumSystemVersion</key>
	<string>10.14</string>
	<key>NSHighResolutionCapable</key>
	<string>True</string>
	<key>CFBundleIconFile</key>
	<string>AppIcon.icns</string>
%s</dict>
</plist>
`, execName, bundleID, name, urlTypes)
}

func adhocSign(appPath string) {
	_ = exec.Command("/usr/bin/xattr", "-dr", "com.apple.quarantine", appPath).Run()
	// 先移除可能残留的旧签名，确保 --force 重签时 seal 一致
	_ = exec.Command("/usr/bin/codesign", "--remove-signature", appPath).Run()
	_ = exec.Command("/usr/bin/codesign", "--force", "--deep", "--sign", "-", appPath).Run()
	// 等待签名 seal 落盘，避免紧接着校验时读到不一致状态
	// （偶发的 "code has no resources but signature indicates they must be present"）
	time.Sleep(500 * time.Millisecond)
}

func registerWithLaunchServices(appPath string) {
	lsregister := "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	if _, err := os.Stat(lsregister); err != nil {
		return
	}
	_ = exec.Command(lsregister, "-f", appPath).Run()
}

func getIconPath() string {
	// 构建期打包脚本可用 BP_BUNDLE_ICON 显式指定图标路径，优先级最高。
	if p := os.Getenv("BP_BUNDLE_ICON"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// 基于可执行文件所在目录动态查找 assets，避免硬编码开发者本地路径
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	assetDir := filepath.Join(filepath.Dir(exePath), "assets")
	iconPaths := []string{
		filepath.Join(assetDir, "BrowserSwitch.icns"),
		filepath.Join(assetDir, "BrowserSwitch.icns"),
	}
	for _, path := range iconPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// copyReader 从已打开的 io.Reader 复制到 dst，等价于 copyFile 但允许调用方控制源文件生命周期。
// 用于 buildHelperApp 场景：必须先打开源文件句柄，再 RemoveAll 旧 bundle，
// 否则在已安装的 bundle 内运行时 self-bundle 会被先删除导致打开失败。
func copyReader(in io.Reader, dst string, perm os.FileMode) error {
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// osStatusPermErr 是 LaunchServices 的 permErr(-54)：更改 http/https 默认处理器需用户
// 在系统确认弹窗中批准，未获批准（用户拒绝或提示尚未完成）时即返回此码。
const osStatusPermErr = -54

// ErrDefaultChangeNeedsApproval 表示更改默认浏览器待用户在 macOS 系统弹窗中确认。
// 这不是故障，而是 macOS 的安全机制：任何 App 更改默认网页浏览器都需用户亲自批准。
var ErrDefaultChangeNeedsApproval = errors.New("default browser change needs user approval")

func setDefaultHandlerForScheme(scheme, bundleID string) error {
	cScheme := C.CString(scheme)
	cBundle := C.CString(bundleID)
	defer C.free(unsafe.Pointer(cScheme))
	defer C.free(unsafe.Pointer(cBundle))
	if st := C.bp_setDefaultHandler(cScheme, cBundle); st != 0 {
		if int(st) == osStatusPermErr {
			return ErrDefaultChangeNeedsApproval
		}
		return fmt.Errorf("LSSetDefaultHandlerForURLScheme(%s → %s) 失败: OSStatus %d", scheme, bundleID, int(st))
	}
	return nil
}

func defaultHandlerForScheme(scheme string) string {
	cScheme := C.CString(scheme)
	defer C.free(unsafe.Pointer(cScheme))
	ptr := C.bp_copyDefaultHandler(cScheme)
	if ptr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}
