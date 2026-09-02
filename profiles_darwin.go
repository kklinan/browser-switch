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

// DetectProfiles 返回某浏览器的可用配置档；不支持/无配置则返回 nil。
// 总在末尾附一个合成的"无痕"配置。
func DetectProfiles(b Browser) []Profile {
	var profiles []Profile
	if dir, ok := chromiumDataDir[b.BundleID()]; ok {
		profiles = detectChromiumProfiles(dir)
	} else if b.BundleID() == "org.mozilla.firefox" {
		profiles = detectFirefoxProfiles()
	}
	// 只有 1 个默认配置时不值得展开（等同直接打开），返回空。
	if len(profiles) <= 1 {
		return nil
	}
	// 末尾加无痕
	profiles = append(profiles, Profile{ID: "__incognito__", Name: i18n.T("picker.incognito"), Kind: "incognito"})
	return profiles
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
