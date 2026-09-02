package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Browser represents a detected browser on the system
type Browser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Exec     string `json:"exec"`
	Desktop  string `json:"desktop"`
	Icon     string `json:"icon"`
	IsCustom bool   `json:"is_custom,omitempty"`
}

// Profile 表示某浏览器的一个登录配置（多账户 / 无痕）。
type Profile struct {
	ID   string `json:"id"`   // 在浏览器内的标识（如 Chrome 的 "Profile 1"）
	Name string `json:"name"` // 展示名
	Kind string `json:"kind"` // "default" / "incognito" / "profile"
}

// MatchMode defines how domain patterns are matched.
//
// 分两类：
//   - 基础模式：exact / urlequal —— 精确比较，语义最直白。
//   - 通配家族：wildcard 是通用形态；contains / prefix / suffix 是它的三个
//     "快捷方式"，供不熟悉 glob/正则语法的用户直接选，避免手写 * 号。
//     三者在匹配时会展开为对应的通配语义（*p* / p* / *p），并对 host 与
//     完整 URL 同时尝试，因此 ".pdf"、"https://" 这类写法也能正常工作。
type MatchMode string

const (
	MatchExact    MatchMode = "exact"    // 仅域名完全相等（host == pattern）
	MatchURLEqual MatchMode = "urlequal" // 完整 URL 完全相等（包括 scheme/path/query/fragment）
	MatchContains MatchMode = "contains" // 快捷方式：包含（≈ 通配 *pattern*）
	MatchPrefix   MatchMode = "prefix"   // 快捷方式：以前缀开头（≈ 通配 pattern*）
	MatchSuffix   MatchMode = "suffix"   // 快捷方式：以后缀结尾（≈ 通配 *pattern）
	MatchWildcard MatchMode = "wildcard" // *.example.com, example.*  (glob: * 与 ?)
	MatchRegex    MatchMode = "regex"    // 完整正则
)

// Rule maps a URL/domain pattern to a browser
type Rule struct {
	ID              string    `json:"id"`
	Pattern         string    `json:"pattern"`
	Mode            MatchMode `json:"mode"`
	Browser         string    `json:"browser"` // browser ID
	Priority        int       `json:"priority"`
	Enabled         bool      `json:"enabled"`
	Comment         string    `json:"comment,omitempty"`
	OpenInNewWindow bool      `json:"open_in_new_window,omitempty"` // true 时强制新窗口打开
}

// Config holds all application settings
type Config struct {
	DefaultBrowser   string    `json:"default_browser"`
	Browsers         []Browser `json:"browsers"`
	Favorites        []string  `json:"favorites"` // 收藏的浏览器 ID，按弹窗显示顺序（决定 ⌘ 编号）
	Hidden           []string  `json:"hidden"`    // 在弹窗/列表中隐藏的浏览器 ID（如误报的非浏览器）
	Rules            []Rule    `json:"rules"`
	AutoCloseDelay   int       `json:"auto_close_delay"` // seconds, 0 = no auto close
	ShowPickerOnMiss bool      `json:"show_picker_on_miss"`
	Language         string    `json:"language"` // 用户选择的语言，空字符串 = 跟随系统
	// PrevDefaultBrowser 记录安装 Switch 前系统的默认浏览器 bundleID，
	// 供卸载时还原。空字符串表示尚未记录（未安装过或安装前就是 Switch 自己）。
	PrevDefaultBrowser string `json:"prev_default_browser,omitempty"`
}

// FavoriteItem 表示收藏列表中的一个可收藏单元：整个浏览器，或浏览器的某个账户（profile）。
// 收藏项在 Config.Favorites 中以复合字符串 key 存储（见 encodeFavKey / decodeFavKey）：
//
//	bundleID                 → 整浏览器（默认 profile），Profile 为 nil
//	bundleID#<profileID>     → 指定账户，Profile 指向具体配置档
type FavoriteItem struct {
	Browser Browser
	Profile *Profile // nil = 整浏览器 / 默认 profile
}

// Key 返回该收藏项在 Config.Favorites 中的存储 key。
func (it FavoriteItem) Key() string {
	if it.Profile == nil {
		return it.Browser.ID
	}
	return encodeFavKey(it.Browser.ID, it.Profile.ID)
}

// encodeFavKey 把浏览器 ID 与 profile ID 编码为复合收藏 key。
// bundle ID 不含 '#'，故用首个 '#' 作为分隔符即可无歧义还原。
func encodeFavKey(browserID, profileID string) string {
	if profileID == "" {
		return browserID
	}
	return browserID + "#" + profileID
}

// decodeFavKey 把复合收藏 key 拆回浏览器 ID 与 profile ID（profileID 为空表示整浏览器）。
func decodeFavKey(key string) (browserID, profileID string) {
	if i := strings.Index(key, "#"); i != -1 {
		return key[:i], key[i+1:]
	}
	return key, ""
}

// FavoriteItems 返回按收藏顺序排列的收藏项（浏览器或浏览器的某账户）；
// 无收藏时回退到全部浏览器（去除隐藏），每个作为整浏览器项。
// 解析不到浏览器或 profile 的悬空 key 会被静默跳过（浏览器卸载 / 账户被删）。
func (c *Config) FavoriteItems() []FavoriteItem {
	hidden := map[string]bool{}
	for _, id := range c.Hidden {
		hidden[id] = true
	}
	if len(c.Favorites) > 0 {
		var items []FavoriteItem
		for _, key := range c.Favorites {
			bid, pid := decodeFavKey(key)
			if hidden[bid] {
				continue
			}
			b := findBrowserByID(bid, c)
			if b == nil {
				continue // 悬空：浏览器已卸载
			}
			if pid == "" {
				items = append(items, FavoriteItem{Browser: *b})
				continue
			}
			p := findProfileByID(*b, pid)
			if p == nil {
				continue // 悬空：账户已删除
			}
			items = append(items, FavoriteItem{Browser: *b, Profile: p})
		}
		if len(items) > 0 {
			return items
		}
	}
	var all []FavoriteItem
	for _, b := range c.Browsers {
		if !hidden[b.ID] {
			all = append(all, FavoriteItem{Browser: b})
		}
	}
	return all
}

// FavoriteBrowsers 返回按收藏顺序排列的浏览器；无收藏时回退到全部（去除隐藏）。
// 兼容包装：仅取收藏项的浏览器维度（忽略 profile），供仅需浏览器列表的旧调用方使用。
func (c *Config) FavoriteBrowsers() []Browser {
	items := c.FavoriteItems()
	out := make([]Browser, 0, len(items))
	for _, it := range items {
		out = append(out, it.Browser)
	}
	return out
}

var (
	configMu   sync.RWMutex
	configPath string
)

func DefaultConfig() *Config {
	return &Config{
		DefaultBrowser:   "",
		Browsers:         nil,
		Rules:            nil,
		AutoCloseDelay:   5, // 选择器 5 秒未操作则用默认浏览器打开
		ShowPickerOnMiss: true,
	}
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "browser-switch")
}

func InitConfig() (*Config, error) {
	dir := ConfigDir()
	configPath = filepath.Join(dir, "config.json")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First run: detect browsers and save defaults
			cfg.Browsers = DetectBrowsers()
			if len(cfg.Browsers) > 0 {
				cfg.DefaultBrowser = cfg.Browsers[0].ID
			}
			if err := SaveConfig(cfg); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	// 处理空文件或仅包含空白字符的文件
	if len(bytes.TrimSpace(data)) == 0 {
		// 空配置文件，重新初始化默认值
		cfg.Browsers = DetectBrowsers()
		if len(cfg.Browsers) > 0 {
			cfg.DefaultBrowser = cfg.Browsers[0].ID
		}
		if err := SaveConfig(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	NormalizeRules(cfg)

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	configMu.Lock()
	defer configMu.Unlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// NormalizeRules 对加载后的规则做最小化的就地清理：
//  1. 丢弃 pattern 为空的规则（无法匹配任何东西，且会让规则列表出现空行）。
//  2. 未知/空的 mode 兜底为 MatchExact，避免匹配时静默失效。
//
// 注意：contains / prefix / suffix 是通配的快捷方式，是一等公民，
// 这里绝不把它们改写成 wildcard —— 保留用户原始意图，便于后续编辑。
func NormalizeRules(cfg *Config) {
	if cfg == nil {
		return
	}
	changed := false
	out := cfg.Rules[:0]
	for _, r := range cfg.Rules {
		if strings.TrimSpace(r.Pattern) == "" {
			changed = true
			continue
		}
		if !isKnownMatchMode(r.Mode) {
			r.Mode = MatchExact
			changed = true
		}
		out = append(out, r)
	}
	cfg.Rules = out
	if changed {
		_ = SaveConfig(cfg)
	}
}

// isKnownMatchMode 判断 mode 是否为已知模式之一。
func isKnownMatchMode(m MatchMode) bool {
	switch m {
	case MatchExact, MatchURLEqual, MatchContains, MatchPrefix, MatchSuffix, MatchWildcard, MatchRegex:
		return true
	}
	return false
}

// RemoveConfig 删除整个配置目录及其中所有数据（供设置界面"卸载并清除数据"使用）。
// 须在 Uninstall 之后调用：Uninstall 内部会重新 SaveConfig 写回配置文件，先删会被重建。
func RemoveConfig() error {
	configMu.Lock()
	defer configMu.Unlock()
	return os.RemoveAll(ConfigDir())
}
