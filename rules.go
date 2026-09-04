package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/kklinan/browser-switch/i18n"
)

// MatchResult represents the result of matching a URL against rules
type MatchResult struct {
	Matched  bool
	Rule     *Rule
	Browser  *Browser
	MatchLog string // human-readable description of the match
}

// MatchURL checks a URL against all enabled rules, returning the best match
func MatchURL(rawURL string, cfg *Config) MatchResult {
	// Parse the URL to extract domain
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return MatchResult{MatchLog: fmt.Sprintf("failed to parse URL: %v", err)}
	}

	// 保留原始 host，同时预备一个剥离 www. 的归一化 host。
	// 匹配时两者都尝试，从而：既能命中用户显式写的 www.example.com 规则，
	// 也能让 www.example.com 访问命中"记住选择"写入的 example.com 规则。
	host := strings.ToLower(parsed.Hostname())
	hostNoWWW := strings.TrimPrefix(host, "www.")
	fullURL := strings.ToLower(rawURL)

	// Sort rules by priority (higher priority first)
	sortedRules := make([]Rule, len(cfg.Rules))
	copy(sortedRules, cfg.Rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		return sortedRules[i].Priority > sortedRules[j].Priority
	})

	for i := range sortedRules {
		rule := &sortedRules[i]
		if !rule.Enabled {
			continue
		}

		matched := matchPattern(host, fullURL, rule.Pattern, rule.Mode)
		if !matched && hostNoWWW != host {
			matched = matchPattern(hostNoWWW, fullURL, rule.Pattern, rule.Mode)
		}
		if matched {
			// Find the browser for this rule
			browser := findBrowserByID(rule.Browser, cfg)
			if browser != nil {
				return MatchResult{
					Matched:  true,
					Rule:     rule,
					Browser:  browser,
					MatchLog: fmt.Sprintf("Rule '%s' (%s: %s) → %s", rule.Pattern, rule.Mode, host, browser.Name),
				}
			}
		}
	}

	return MatchResult{
		MatchLog: fmt.Sprintf("no rule matched for host: %s", host),
	}
}

// matchPattern checks if a host/URL matches a pattern using the specified mode.
//
// contains / prefix / suffix 是 wildcard 的"快捷方式"：除 host 外也会对完整 URL
// 尝试，这样小白用户写 ".pdf"、"https://"、"login" 这类非域名内容也能命中
// （只看 host 的话这些规则永远不生效）。
func matchPattern(host, fullURL, pattern string, mode MatchMode) bool {
	// 统一小写：调用方（MatchURL）通常已归一化，这里再做一次是幂等的，
	// 可避免新的调用点忘记归一化时静默匹配失败。
	host = strings.ToLower(host)
	fullURL = strings.ToLower(fullURL)
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}

	switch mode {
	case MatchExact:
		return host == pattern

	case MatchURLEqual:
		// 完整 URL 完全相等：包含 scheme/path/query/fragment。
		return fullURL == pattern

	case MatchWildcard:
		// 同时对 host 与完整 URL 尝试：只匹配 host 的话，
		// 像 "*github.com/*"、"https://*"、"*/settings" 这类含路径的写法永远命中不了。
		return matchWildcard(host, pattern) || matchWildcard(fullURL, pattern)

	case MatchRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		// Try matching against both host and full URL
		return re.MatchString(host) || re.MatchString(fullURL)

	case MatchContains:
		return strings.Contains(host, pattern) || strings.Contains(fullURL, pattern)

	case MatchPrefix:
		return strings.HasPrefix(host, pattern) || strings.HasPrefix(fullURL, pattern)

	case MatchSuffix:
		return strings.HasSuffix(host, pattern) || strings.HasSuffix(fullURL, pattern)

	default:
		return false
	}
}

// matchWildcard implements wildcard pattern matching against a single target.
// Supports: * (any sequence), ? (single char)
func matchWildcard(target, pattern string) bool {
	re, err := wildcardRegexp(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(target)
}

// wildcardCache 缓存 pattern → 编译后的正则。规则匹配在每次打开链接时都会跑，
// 逐条重新编译属于纯浪费，尤其正则/通配稍多时。
var wildcardCache sync.Map // map[string]*regexp.Regexp

// wildcardRegexp 把通配 pattern 编译成正则（带缓存）。
func wildcardRegexp(pattern string) (*regexp.Regexp, error) {
	if re, ok := wildcardCache.Load(pattern); ok {
		return re.(*regexp.Regexp), nil
	}
	// Convert wildcard pattern to regex
	regexStr := "^"
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '*':
			regexStr += ".*"
		case '?':
			regexStr += "."
		case '.', '+', '(', ')', '[', ']', '{', '}', '\\', '^', '$', '|':
			regexStr += "\\" + string(ch)
		default:
			regexStr += string(ch)
		}
	}
	regexStr += "$"

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, err
	}
	wildcardCache.Store(pattern, re)
	return re, nil
}

// findBrowserByID finds a browser in the config by its ID
func findBrowserByID(id string, cfg *Config) *Browser {
	for i := range cfg.Browsers {
		if cfg.Browsers[i].ID == id {
			return &cfg.Browsers[i]
		}
	}
	return nil
}

// ValidatePattern checks if a pattern is valid for the given mode
func ValidatePattern(pattern string, mode MatchMode) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	switch mode {
	case MatchRegex:
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	case MatchWildcard:
		// 通配语法本身没有非法形式，但仍编译一次以确保转换后的正则可用。
		if _, err := wildcardRegexp(pattern); err != nil {
			return fmt.Errorf("invalid wildcard: %w", err)
		}
	case MatchExact, MatchURLEqual, MatchContains, MatchPrefix, MatchSuffix:
		// All valid
	}

	return nil
}

// ValidateRuleInput 在保存规则前做一次"面向用户"的语义校验，返回可直接展示的错误。
// 它与 ValidatePattern 的分工：后者只保证语法可编译，这里额外捕捉
// "语法没问题但语义明显写错、保存后一定不生效"的情况。
//
// browserID / profileID 参与重复检测（目标是"浏览器 + 账户"，同一个域名用 Chrome
// 的不同账号打开是两条不同规则）；excludeID 用于编辑场景排除规则自身，避免把自己
// 判成重复。
func ValidateRuleInput(pattern string, mode MatchMode, browserID, profileID string, rules []Rule, excludeID string) error {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return fmt.Errorf(i18n.T("settings.add_rule.error.empty_pattern"))
	}

	if err := ValidatePattern(p, mode); err != nil {
		if mode == MatchRegex {
			return fmt.Errorf(i18n.T("settings.add_rule.error.invalid_regex"), err)
		}
		return err
	}

	switch mode {
	case MatchURLEqual:
		if !strings.Contains(p, "://") {
			return fmt.Errorf(i18n.T("settings.add_rule.error.urlequal_need_scheme"))
		}
	case MatchExact:
		// 域名里不可能出现 scheme 或路径分隔符，出现说明用户想要的是别的模式。
		if strings.Contains(p, "://") {
			return fmt.Errorf(i18n.T("settings.add_rule.error.exact_has_scheme"))
		}
		if strings.Contains(p, "/") {
			return fmt.Errorf(i18n.T("settings.add_rule.error.exact_has_path"))
		}
	case MatchContains, MatchPrefix, MatchSuffix:
		// 快捷方式里写了 * 或 ?，说明用户其实想用通配，但快捷方式不会解释它们，
		// 只会被当成普通字符 —— 结果规则永远不命中，必须拦下来。
		if strings.ContainsAny(p, "*?") {
			return fmt.Errorf(i18n.T("settings.add_rule.error.quick_has_glob"))
		}
	}

	// 完全重复的规则（同 pattern + 同 mode + 同浏览器 + 同账户）没有任何意义，
	// 且会让人困惑于"改了怎么没生效"（被优先级相同的另一条抢先命中）。
	for _, r := range rules {
		if r.ID == excludeID {
			continue
		}
		if r.Mode == mode && strings.EqualFold(strings.TrimSpace(r.Pattern), p) &&
			r.Browser == browserID && r.Profile == profileID {
			return fmt.Errorf(i18n.Tf("settings.add_rule.error.duplicate", p))
		}
	}
	return nil
}

// SuggestMatchMode analyzes a pattern and suggests the best match mode
func SuggestMatchMode(pattern string) MatchMode {
	// 正则元字符（转义、字符组、分组、择一）优先判定为 regex，
	// 即便其中也含有 * —— 例如 ".*\.test\..*" 应是 regex 而非通配符。
	if strings.ContainsAny(pattern, `\[]()|`) {
		return MatchRegex
	}
	// 含 scheme（http:// https:// ftp:// 等）→ 完整 URL 完全相等
	if strings.Contains(pattern, "://") {
		return MatchURLEqual
	}
	if strings.ContainsAny(pattern, "*?") {
		return MatchWildcard
	}
	return MatchExact
}
