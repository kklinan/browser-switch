//go:build darwin

// macOS 平台实现：通过扫描 .app 包并解析 Info.plist 的 CFBundleURLTypes
// 来发现声明了 http/https scheme 处理能力的浏览器；通过 `open -b <bundleID>`
// 启动浏览器（最可靠，遵循 LaunchServices）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kklinan/browser-switch/i18n"
)

// macInfoPlist 是 .app/Contents/Info.plist 中我们关心的子集。
type macInfoPlist struct {
	CFBundleIdentifier string `json:"CFBundleIdentifier"`
	CFBundleName       string `json:"CFBundleName"`
	CFBundleURLTypes   []struct {
		CFBundleURLSchemes []string `json:"CFBundleURLSchemes"`
	} `json:"CFBundleURLTypes"`
}

// knownMacBrowser 用作兜底：某些浏览器（尤其是 Safari）不会在
// CFBundleURLTypes 中显式声明 http scheme，故按已知 bundleID + 路径补充。
type knownMacBrowser struct {
	Name     string
	BundleID string
	AppPath  string
}

var knownMacBrowsers = []knownMacBrowser{
	{"Safari", "com.apple.Safari", "/Applications/Safari.app"},
	{"Google Chrome", "com.google.Chrome", "/Applications/Google Chrome.app"},
	{"Firefox", "org.mozilla.firefox", "/Applications/Firefox.app"},
	{"Microsoft Edge", "com.microsoft.edgemac", "/Applications/Microsoft Edge.app"},
	{"Brave", "com.brave.Browser", "/Applications/Brave Browser.app"},
	{"Arc", "company.thebrowser.Browser", "/Applications/Arc.app"},
	{"Opera", "com.operasoftware.Opera", "/Applications/Opera.app"},
	{"Vivaldi", "com.vivaldi.Vivaldi", "/Applications/Vivaldi.app"},
	{"Chromium", "org.chromium.Chromium", "/Applications/Chromium.app"},
	{"Microsoft Edge Dev", "com.microsoft.edgemac.Dev", "/Applications/Microsoft Edge Dev.app"},
	{"Tor Browser", "org.torproject.torbrowser", "/Applications/Tor Browser.app"},
	{"Zen", "app.zen-browser.zen", "/Applications/Zen Browser.app"},
}

// appSearchDirs 返回需要扫描 .app 的目录。
func appSearchDirs() []string {
	dirs := []string{"/Applications", "/Applications/Utilities", "/System/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append([]string{filepath.Join(home, "Applications")}, dirs...)
	}
	return dirs
}

// DetectBrowsers scans the system for installed browsers (macOS).
func DetectBrowsers() []Browser {
	var found []Browser
	seen := make(map[string]bool)

	// 方法 1：扫描 .app 并解析 Info.plist，凡声明 http/https scheme 处理者即为浏览器
	for _, dir := range appSearchDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".app") {
				continue
			}
			appPath := filepath.Join(dir, e.Name())
			info, ok := readBundleInfo(appPath)
			if !ok || info.CFBundleIdentifier == "" {
				continue
			}
			if info.CFBundleIdentifier == AppBundleID {
				continue // 跳过我们自己
			}
			if !bundleHandlesHTTP(info) {
				continue
			}
			if seen[info.CFBundleIdentifier] {
				continue
			}
			seen[info.CFBundleIdentifier] = true
			found = append(found, Browser{
				ID:      info.CFBundleIdentifier,
				Name:    macDisplayName(info, appPath),
				Exec:    info.CFBundleIdentifier, // 通过 `open -b <id>` 启动
				Desktop: appPath,                 // 包路径，作为兜底与展示
				Icon:    info.CFBundleIdentifier,
			})
		}
	}

	// 方法 2：已知浏览器兜底（如 Safari 不在 CFBundleURLTypes 中声明 http）
	for _, kb := range knownMacBrowsers {
		if seen[kb.BundleID] {
			continue
		}
		if _, err := os.Stat(kb.AppPath); err != nil {
			continue
		}
		seen[kb.BundleID] = true
		found = append(found, Browser{
			ID:      kb.BundleID,
			Name:    kb.Name,
			Exec:    kb.BundleID,
			Desktop: kb.AppPath,
			Icon:    kb.BundleID,
		})
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].Name < found[j].Name
	})

	return found
}

// readBundleInfo 用 plutil 将 Info.plist（可能是二进制格式）转为 JSON 后解析。
func readBundleInfo(appPath string) (macInfoPlist, bool) {
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	if _, err := os.Stat(plist); err != nil {
		return macInfoPlist{}, false
	}
	out, err := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", plist).Output()
	if err != nil {
		return macInfoPlist{}, false
	}
	var info macInfoPlist
	if err := json.Unmarshal(out, &info); err != nil {
		return macInfoPlist{}, false
	}
	return info, true
}

// bundleHandlesHTTP 判断该包是否声明处理 http/https scheme。
func bundleHandlesHTTP(info macInfoPlist) bool {
	for _, t := range info.CFBundleURLTypes {
		for _, s := range t.CFBundleURLSchemes {
			switch strings.ToLower(s) {
			case "http", "https":
				return true
			}
		}
	}
	return false
}

// macDisplayName 优先用包目录名（用户最熟悉），回退到 CFBundleName。
func macDisplayName(info macInfoPlist, appPath string) string {
	if base := strings.TrimSuffix(filepath.Base(appPath), ".app"); base != "" {
		return base
	}
	if info.CFBundleName != "" {
		return info.CFBundleName
	}
	return info.CFBundleIdentifier
}

// LaunchBrowser opens a URL in the specified browser (macOS).
func LaunchBrowser(browser Browser, url string) error {
	var cmd *exec.Cmd
	switch {
	case looksLikeBundleID(browser.Exec):
		// open -b 用 bundle id 启动，最可靠；不加 -n 以复用已开窗口
		cmd = exec.Command("/usr/bin/open", "-b", browser.Exec, url)
	case browser.Desktop != "":
		cmd = exec.Command("/usr/bin/open", "-a", browser.Desktop, url)
	case browser.Exec != "":
		cmd = exec.Command("/usr/bin/open", "-a", browser.Exec, url)
	default:
		return fmt.Errorf(i18n.T("browser.launch_no_target"), browser.Name)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start %s: %w: %s", browser.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// looksLikeBundleID 粗略判断字符串是否为 macOS bundle identifier
// （形如 com.vendor.app：含点、无斜杠、无空格）。
func looksLikeBundleID(s string) bool {
	return s != "" && strings.Contains(s, ".") && !strings.ContainsAny(s, "/ ")
}
