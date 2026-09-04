//go:build darwin

// 多账户配置（profiles）检测与启动 —— macOS。
//
// Chromium 家族（Chrome/Edge/Brave/Vivaldi…）把账户配置记录在用户数据目录的
// `Local State` JSON 的 profile.info_cache 中；Firefox 记录在 profiles.ini。
// 启动指定配置必须直接执行包内二进制并传 `--profile-directory=`（Chromium）或
// `-P`（Firefox）——经 `open -b` 在浏览器已运行时这些参数会被忽略。
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

// chromiumDataDir 把 bundleID 映射到其用户数据目录（相对 ~/Library/Application Support）。
var chromiumDataDir = map[string]string{
	"com.google.Chrome":        "Google/Chrome",
	"com.google.Chrome.canary": "Google/Chrome Canary",
	"com.microsoft.edgemac":    "Microsoft Edge",
	"com.brave.Browser":        "BraveSoftware/Brave-Browser",
	"com.vivaldi.Vivaldi":      "Vivaldi",
	"com.operasoftware.Opera":  "com.operasoftware.Opera",
}

// incognitoFlag 各 Chromium 浏览器的无痕参数（Edge 用 inprivate）。
func incognitoFlag(bundleID string) string {
	if bundleID == "com.microsoft.edgemac" {
		return "--inprivate"
	}
	return "--incognito"
}

// DetectProfiles 返回某浏览器的可用配置档；不支持多账户则返回 nil。
//
// Chromium 系与 Firefox 总在末尾附一个合成的"无痕"配置 —— 即便只检测到默认
// 这一个账户。绝大多数用户就只有默认账户，此前"少于 2 个配置就不展开"的规则
// 会让他们在选择器里根本选不到无痕。
func DetectProfiles(b Browser) []Profile {
	bundleID := b.BundleID()
	var profiles []Profile
	switch {
	case chromiumDataDir[bundleID] != "":
		profiles = detectChromiumProfiles(chromiumDataDir[bundleID])
	case bundleID == "org.mozilla.firefox":
		profiles = detectFirefoxProfiles()
	default:
		return nil // 不支持多账户/无痕（如 Safari）
	}
	profiles = append(profiles, Profile{ID: IncognitoProfileID, Name: i18n.T("picker.incognito"), Kind: "incognito"})
	return profiles
}

// profilesNeedChoice 判断打开前是否必须让用户挑账户：只有存在多个**真实**配置档
// 时才必须选。无痕是合成项，不构成"必须选"的理由 —— 只有一个账户的用户点卡片
// 应该直接打开，无痕走右键菜单。
func profilesNeedChoice(profiles []Profile) bool {
	n := 0
	for _, p := range profiles {
		if p.Kind != "incognito" {
			n++
		}
	}
	return n > 1
}

// BundleID 返回浏览器的 bundle id（darwin 上即 Exec/Icon 字段）。
func (b Browser) BundleID() string {
	if looksLikeBundleID(b.Exec) {
		return b.Exec
	}
	return b.Icon
}

type chromiumLocalState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name       string `json:"name"`
			UserName   string `json:"user_name"`
			GAIAName   string `json:"gaia_name"`
			IsConsumer bool   `json:"is_consented_primary_account"`
		} `json:"info_cache"`
	} `json:"profile"`
}

func detectChromiumProfiles(relDir string) []Profile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	base := filepath.Join(home, "Library", "Application Support", relDir)
	raw, err := os.ReadFile(filepath.Join(base, "Local State"))
	if err != nil {
		return nil
	}
	var ls chromiumLocalState
	if err := json.Unmarshal(raw, &ls); err != nil {
		return nil
	}
	var profiles []Profile
	for dir, info := range ls.Profile.InfoCache {
		name := info.Name
		if name == "" {
			name = info.GAIAName
		}
		if name == "" {
			name = info.UserName
		}
		if name == "" {
			name = dir
		}
		kind := "profile"
		if dir == "Default" {
			kind = "default"
		}
		profiles = append(profiles, Profile{ID: dir, Name: name, Kind: kind})
	}
	// 默认配置排首位，其余按名称稳定排序。
	sort.SliceStable(profiles, func(i, j int) bool {
		if (profiles[i].Kind == "default") != (profiles[j].Kind == "default") {
			return profiles[i].Kind == "default"
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
}

func detectFirefoxProfiles() []Profile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(home, "Library", "Application Support", "Firefox", "profiles.ini"))
	if err != nil {
		return nil
	}
	var profiles []Profile
	var cur struct {
		name    string
		isDef   bool
		hasName bool
	}
	flush := func() {
		if cur.hasName {
			kind := "profile"
			if cur.isDef {
				kind = "default"
			}
			profiles = append(profiles, Profile{ID: cur.name, Name: cur.name, Kind: kind})
		}
		cur.name, cur.isDef, cur.hasName = "", false, false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "[Profile"):
			flush()
		case strings.HasPrefix(line, "Name="):
			cur.name = strings.TrimPrefix(line, "Name=")
			cur.hasName = true
		case strings.HasPrefix(line, "Default=") && line != "Default=":
			v := strings.TrimPrefix(line, "Default=")
			cur.isDef = v == "1" || strings.Contains(v, "/")
		case strings.HasPrefix(line, "["): // 进入非 Profile 段
			flush()
		}
	}
	flush()
	return profiles
}

// findProfileByID 在浏览器的可用配置档中按 ID 查找（含合成的无痕项）。
// 找不到返回 nil —— 用于把收藏 key 里的 profileID 反查回具体 Profile 对象。
func findProfileByID(b Browser, profileID string) *Profile {
	for _, p := range DetectProfiles(b) {
		if p.ID == profileID {
			pp := p
			return &pp
		}
	}
	return nil
}

// LaunchBrowserProfile 用指定配置档打开 URL。空配置或合成无痕走相应分支。
// newWindow 为 true 时在原 profile 中打开到新窗口（不影响 profile 选择）。
func LaunchBrowserProfile(b Browser, p Profile, url string, newWindow bool) error {
	bundleID := b.BundleID()

	// 无痕：直接执行二进制 + 无痕参数最稳，失败回退 open。
	if p.Kind == "incognito" {
		if exe := appExecPath(b.Desktop); exe != "" {
			if _, ok := chromiumDataDir[bundleID]; ok {
				args := []string{incognitoFlag(bundleID)}
				if newWindow {
					args = append(args, "--new-window")
				}
				return runDetached(exe, append(args, url)...)
			}
			if bundleID == "org.mozilla.firefox" {
				args := []string{"--private-window"}
				if newWindow {
					args = append(args, "--new-window")
				}
				return runDetached(exe, append(args, url)...)
			}
		}
		return LaunchBrowser(b, url, newWindow)
	}

	exe := appExecPath(b.Desktop)
	if exe == "" {
		return LaunchBrowser(b, url, newWindow) // 兜底
	}
	if _, ok := chromiumDataDir[bundleID]; ok {
		args := []string{"--profile-directory=" + p.ID}
		if newWindow {
			args = append(args, "--new-window")
		}
		return runDetached(exe, append(args, url)...)
	}
	if bundleID == "org.mozilla.firefox" {
		args := []string{"-P", p.ID}
		if newWindow {
			args = append(args, "--new-window")
		}
		return runDetached(exe, append(args, url)...)
	}
	return LaunchBrowser(b, url, newWindow)
}

// launchForRule 按规则匹配结果启动浏览器 —— 两条 URL 入口（命令行 handleURL 与
// Apple Event 的 openPicker）共用，避免"记住了账户却没用上"两边各漏一处。
//
// 规则指定了账户时走 LaunchBrowserProfile；未指定、或账户已被删除/浏览器不再支持
// 多账户时退回普通打开 —— 宁可用默认账户打开，也不能什么都不开。
// newWindow 由规则的"新窗口打开"开关决定，与账户选择互不干扰。
func launchForRule(u string, result MatchResult) error {
	if result.Browser == nil {
		return fmt.Errorf("no browser to launch")
	}
	newWin := result.Rule != nil && result.Rule.OpenInNewWindow
	if result.Rule != nil && result.Rule.Profile != "" {
		if p := findProfileByID(*result.Browser, result.Rule.Profile); p != nil {
			return LaunchBrowserProfile(*result.Browser, *p, u, newWin)
		}
	}
	return LaunchBrowser(*result.Browser, u, newWin)
}

// appExecPath 读取 .app 的 CFBundleExecutable，返回包内二进制完整路径。
func appExecPath(appPath string) string {
	if appPath == "" || !strings.HasSuffix(appPath, ".app") {
		return ""
	}
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	out, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleExecutable", "raw", "-o", "-", plist).Output()
	exe := strings.TrimSpace(string(out))
	if err != nil || exe == "" {
		// 兜底：取 MacOS 目录下唯一可执行文件
		macOS := filepath.Join(appPath, "Contents", "MacOS")
		entries, e := os.ReadDir(macOS)
		if e != nil || len(entries) == 0 {
			return ""
		}
		return filepath.Join(macOS, entries[0].Name())
	}
	return filepath.Join(appPath, "Contents", "MacOS", exe)
}

// runDetached 启动浏览器进程但不等待其退出（选择器随后会退出）。
func runDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
