package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
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

// matchPattern checks if a host/URL matches a pattern using the specified mode
func matchPattern(host, fullURL, pattern string, mode MatchMode) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	switch mode {
	case MatchExact:
		return host == pattern

	case MatchWildcard:
		return matchWildcard(host, pattern)

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
		return strings.HasPrefix(host, pattern)

	case MatchSuffix:
		return strings.HasSuffix(host, pattern)

	default:
		return false
	}
}

// matchWildcard implements wildcard pattern matching
// Supports: * (any sequence), ? (single char)
func matchWildcard(host, pattern string) bool {
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
		return false
	}
	return re.MatchString(host)
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
		// Wildcards are always valid syntactically
	case MatchExact, MatchContains, MatchPrefix, MatchSuffix:
		// All valid
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
	if strings.ContainsAny(pattern, "*?") {
		return MatchWildcard
	}
	return MatchExact
}
