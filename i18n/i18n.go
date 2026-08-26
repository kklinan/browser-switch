// Package i18n 提供多语言国际化支持。
// 使用 embed 将 locales/*.json 编译进二进制，运行时根据系统语言自动选择翻译。
//
// 使用方式：
//
//	i18n.Init()                  // 自动检测系统语言
//	label := i18n.T("app.name")  // 翻译单个 key
//	text := i18n.Tf("cli.output.detected_browsers", 5) // 格式化翻译
//
// 支持的语言：zh-CN, zh-TW, en, ja, ko, pt, hi
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	translations map[string]map[string]string // lang → key → value
	currentLang  string
	mu           sync.RWMutex
)

// Init 检测系统语言并加载对应翻译包。
// 检测优先级：LC_ALL > LC_MESSAGES > LANG > AppleLocale(macOS) > "en"
func Init() {
	loadAllLocales()
	lang := detectLanguage()
	setLanguage(lang)
}

// loadAllLocales 从 embed 中解析全部语言包。
func loadAllLocales() {
	mu.Lock()
	defer mu.Unlock()

	translations = make(map[string]map[string]string)

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		// 文件名去掉 .json 后缀作为语言代码
		lang := strings.TrimSuffix(e.Name(), ".json")
		translations[lang] = m
	}
}

// detectLanguage 根据系统环境检测当前语言。
func detectLanguage() string {
	// 1. 按优先级检查环境变量
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(env); v != "" {
			if lang := mapLocale(v); lang != "" {
				return lang
			}
		}
	}
	// 2. macOS: defaults read -g AppleLocale（不阻塞，仅作补充）——该调用较慢，仅在 LANG 为空时尝试
	if stdLang := appleLocale(); stdLang != "" {
		return stdLang
	}
	return "en"
}

// mapLocale 将 POSIX locale 字符串映射到我们的语言代码。
func mapLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	// 去掉编码段（.UTF-8、.utf8 等）
	if idx := strings.Index(locale, "."); idx != -1 {
		locale = locale[:idx]
	}
	// 去掉 @ 修饰符
	if idx := strings.Index(locale, "@"); idx != -1 {
		locale = locale[:idx]
	}

	lang, _, _ := strings.Cut(locale, "_")

	switch lang {
	case "zh":
		// LANG=zh_CN → zh-CN, LANG=zh_TW / zh_HK → zh-TW
		if strings.Contains(locale, "tw") || strings.Contains(locale, "hk") || strings.Contains(locale, "hant") {
			return "zh-TW"
		}
		return "zh-CN"
	case "ja", "jp":
		return "ja"
	case "ko", "kr":
		return "ko"
	case "pt":
		return "pt"
	case "hi":
		return "hi"
	case "en":
		return "en"
	default:
		return ""
	}
}

// appleLocale 通过 `defaults read -g AppleLocale` 读取 macOS 用户的区域语言，
// 并映射为受支持的语言代码；读取失败或语言不受支持时返回 ""。
//
// 为何必需：经 LaunchServices 启动的 .app 不继承 shell 环境变量，LANG / LC_ALL
// 通常为空，仅靠环境变量检测会让已安装的应用永远以英文启动。AppleLocale 形如
// zh_CN / en_US，正好与 mapLocale 的 POSIX 解析兼容；此处不引入 cgo，直接调用
// 系统内置的 defaults 命令（运行期本就依赖它）。
func appleLocale() string {
	out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output()
	if err != nil {
		return ""
	}
	return mapLocale(strings.TrimSpace(string(out)))
}

// T 返回 key 在当前语言的翻译；找不到时回退到 English，再找不到返回 key 本身。
func T(key string) string {
	mu.RLock()
	defer mu.RUnlock()

	// 先查当前语言
	if m, ok := translations[currentLang]; ok {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
	}
	// 回退 English
	if currentLang != "en" {
		if m, ok := translations["en"]; ok {
			if v, ok := m[key]; ok && v != "" {
				return v
			}
		}
	}
	return key
}

// Tf 格式化翻译文本，等价于 fmt.Sprintf(T(key), args...).
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// SetLanguage 手动切换语言（如用户在设置中选择）。
func SetLanguage(lang string) {
	setLanguage(lang)
}

func setLanguage(lang string) {
	mu.Lock()
	defer mu.Unlock()
	// 检查语言是否存在
	if _, ok := translations[lang]; ok {
		currentLang = lang
	}
	// 若不存在则保持不变
}

// Language 返回当前使用的语言代码。
func Language() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// SupportedLanguages 返回所有可用语言代码列表。
func SupportedLanguages() []string {
	return []string{"zh-CN", "zh-TW", "en", "ja", "ko", "pt", "hi"}
}

// LanguageNativeName 返回语言代码对应的原生名称（用于语言选择器下拉列表）。
func LanguageNativeName(lang string) string {
	switch lang {
	case "zh-CN":
		return "简体中文"
	case "zh-TW":
		return "繁體中文"
	case "en":
		return "English"
	case "ja":
		return "日本語"
	case "ko":
		return "한국어"
	case "pt":
		return "Português"
	case "hi":
		return "हिन्दी"
	default:
		return lang
	}
}
