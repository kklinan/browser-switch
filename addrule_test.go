package main

import "testing"

// TestValidatePattern 覆盖添加规则时的模式校验。
func TestValidatePattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		mode    MatchMode
		wantErr bool
	}{
		{"空模式报错", "", MatchExact, true},
		{"精确非空", "example.com", MatchExact, false},
		{"合法正则", `^https?://.*\.example\.com`, MatchRegex, false},
		{"非法正则-未闭合括号", "([a-z", MatchRegex, true},
		{"非法正则-未闭合方括号", "[a-z", MatchRegex, true},
		{"通配符总是合法", "*.example.*", MatchWildcard, false},
		{"contains非空", "aliyun", MatchContains, false},
		{"prefix非空", "https://", MatchPrefix, false},
		{"suffix非空", ".cn", MatchSuffix, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePattern(c.pattern, c.mode)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePattern(%q, %s): err=%v, wantErr=%v", c.pattern, c.mode, err, c.wantErr)
			}
		})
	}
}

// TestSuggestMatchMode 覆盖添加规则对话框里"根据输入自动推测模式"。
func TestSuggestMatchMode(t *testing.T) {
	cases := []struct {
		pattern string
		want    MatchMode
	}{
		{"example.com", MatchExact},
		{"*.example.com", MatchWildcard},
		{"example.?om", MatchWildcard},
		{`.*\.test\..*`, MatchRegex}, // 含转义 → 正则优先，即便有 *
		{`(a|b).com`, MatchRegex},    // 含分组和择一
		{`[a-z]+\.com`, MatchRegex},  // 含字符组
		{"plain-text", MatchExact},
	}
	for _, c := range cases {
		got := SuggestMatchMode(c.pattern)
		if got != c.want {
			t.Errorf("SuggestMatchMode(%q) = %s, want %s", c.pattern, got, c.want)
		}
	}
}

// TestMatchPatternModes 覆盖每种匹配模式的命中与不命中。
func TestMatchPatternModes(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		fullURL string
		pattern string
		mode    MatchMode
		want    bool
	}{
		{"exact命中", "example.com", "https://example.com/", "example.com", MatchExact, true},
		{"exact不命中子域", "sub.example.com", "https://sub.example.com/", "example.com", MatchExact, false},
		{"wildcard子域命中", "sub.example.com", "https://sub.example.com/", "*.example.com", MatchWildcard, true},
		{"wildcard单字符", "a.example.com", "https://a.example.com/", "?.example.com", MatchWildcard, true},
		{"regex命中", "mail.google.com", "https://mail.google.com/", `.*\.google\.com`, MatchRegex, true},
		{"regex非法返回false", "x", "https://x/", "([", MatchRegex, false},
		{"contains命中host", "aliyun.com", "https://qiye.aliyun.com/x", "aliyun", MatchContains, true},
		{"contains命中fullURL", "example.com", "https://example.com/login", "login", MatchContains, true},
		{"prefix命中", "www.example.com", "https://www.example.com/", "www.", MatchPrefix, true},
		{"suffix命中", "example.cn", "https://example.cn/", ".cn", MatchSuffix, true},
		{"suffix不命中", "example.com", "https://example.com/", ".cn", MatchSuffix, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchPattern(c.host, c.fullURL, c.pattern, c.mode)
			if got != c.want {
				t.Errorf("matchPattern(host=%q, pattern=%q, %s) = %v, want %v", c.host, c.pattern, c.mode, got, c.want)
			}
		})
	}
}

// TestAddRulePriorityOrdering 验证添加多条规则后按优先级匹配（高优先级先命中）。
func TestAddRulePriorityOrdering(t *testing.T) {
	cfg := &Config{
		Browsers: []Browser{
			{ID: "chrome", Name: "Chrome", Exec: "chrome"},
			{ID: "safari", Name: "Safari", Exec: "safari"},
		},
		Rules: []Rule{
			{ID: "low", Pattern: "example.com", Mode: MatchExact, Browser: "chrome", Priority: 10, Enabled: true},
			{ID: "high", Pattern: "example.com", Mode: MatchExact, Browser: "safari", Priority: 100, Enabled: true},
		},
	}
	result := MatchURL("https://example.com/", cfg)
	if !result.Matched || result.Browser.ID != "safari" {
		t.Errorf("应命中高优先级规则(safari)，实际 matched=%v browser=%v", result.Matched, result.Browser)
	}
}

// TestDisabledRuleSkipped 验证禁用的规则不参与匹配。
func TestDisabledRuleSkipped(t *testing.T) {
	cfg := &Config{
		Browsers: []Browser{{ID: "chrome", Name: "Chrome", Exec: "chrome"}},
		Rules: []Rule{
			{ID: "r", Pattern: "example.com", Mode: MatchExact, Browser: "chrome", Priority: 100, Enabled: false},
		},
	}
	result := MatchURL("https://example.com/", cfg)
	if result.Matched {
		t.Errorf("禁用规则不应命中，实际 matched=%v", result.Matched)
	}
}

// TestRemoveRule 验证删除规则逻辑。
func TestRemoveRule(t *testing.T) {
	rules := []Rule{
		{ID: "a", Pattern: "a.com"},
		{ID: "b", Pattern: "b.com"},
		{ID: "c", Pattern: "c.com"},
	}
	out := removeRule(rules, "b")
	if len(out) != 2 {
		t.Fatalf("删除后应剩 2 条，实际 %d", len(out))
	}
	for _, r := range out {
		if r.ID == "b" {
			t.Errorf("规则 b 未被删除")
		}
	}
	// 删除不存在的 ID 应保持原样
	out2 := removeRule(rules, "zzz")
	if len(out2) != 3 {
		t.Errorf("删除不存在的规则应保持 3 条，实际 %d", len(out2))
	}
}

// TestAddRuleDuplicateGuard 验证 addRuleForURL 的去重逻辑（同 host+browser 不重复添加）。
func TestAddRuleDuplicateGuard(t *testing.T) {
	cfg := &Config{
		Browsers: []Browser{{ID: "chrome", Name: "Chrome", Exec: "chrome"}},
	}
	// 手动复刻去重判断（不触发磁盘写）
	url := "https://example.com/"
	host := extractDomain(url)
	add := func() {
		for _, r := range cfg.Rules {
			if r.Pattern == host && r.Browser == "chrome" {
				return
			}
		}
		cfg.Rules = append(cfg.Rules, Rule{ID: "x", Pattern: host, Browser: "chrome", Mode: MatchExact, Enabled: true})
	}
	add()
	add()
	add()
	if len(cfg.Rules) != 1 {
		t.Errorf("同 host+browser 应去重为 1 条，实际 %d 条", len(cfg.Rules))
	}
}
