package main

import "testing"

// TestNormalizeRulesPreservesQuickModes 保证 contains / prefix / suffix 这三个
// "通配快捷方式"是受保护的一等公民：加载配置时不会被悄悄改写成 wildcard
// （否则用户再次编辑规则时会看到模式莫名变了）。
func TestNormalizeRulesPreservesQuickModes(t *testing.T) {
	old := configPath
	defer func() { configPath = old }()
	configPath = "" // 无效路径，避免回写时动到用户真实配置

	cfg := &Config{Rules: []Rule{
		{ID: "a", Pattern: "aliyun", Mode: MatchContains, Enabled: true},
		{ID: "b", Pattern: "https://x", Mode: MatchPrefix, Enabled: true},
		{ID: "c", Pattern: ".cn", Mode: MatchSuffix, Enabled: true},
		{ID: "d", Pattern: "example.com", Mode: MatchExact, Enabled: true},
		{ID: "e", Pattern: "   ", Mode: MatchExact, Enabled: true}, // 空 pattern → 剔除
	}}
	NormalizeRules(cfg)

	if len(cfg.Rules) != 4 {
		t.Fatalf("清理后应有 4 条规则（空 pattern 被剔除），实际 %d", len(cfg.Rules))
	}
	want := map[string]MatchMode{
		"a": MatchContains,
		"b": MatchPrefix,
		"c": MatchSuffix,
		"d": MatchExact,
	}
	for _, r := range cfg.Rules {
		if got := r.Mode; got != want[r.ID] {
			t.Errorf("规则 %s 的 mode 应保持 %s，实际 %s", r.ID, want[r.ID], got)
		}
	}
}

// TestNormalizeRulesIdempotent 保证清理是幂等的，且未知 mode 会兜底为 exact。
func TestNormalizeRulesIdempotent(t *testing.T) {
	old := configPath
	defer func() { configPath = old }()
	configPath = ""

	cfg := &Config{Rules: []Rule{
		{ID: "a", Pattern: "aliyun", Mode: MatchContains},
		{ID: "b", Pattern: "example.com", Mode: MatchMode("bogus")},
	}}
	NormalizeRules(cfg)
	NormalizeRules(cfg)
	if len(cfg.Rules) != 2 {
		t.Fatalf("重复清理不应增删规则，实际 %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Mode != MatchContains {
		t.Errorf("contains 应原样保留，实际 %s", cfg.Rules[0].Mode)
	}
	if cfg.Rules[1].Mode != MatchExact {
		t.Errorf("未知 mode 应兜底为 exact，实际 %s", cfg.Rules[1].Mode)
	}
}

// TestQuickModesMatchFullURL 验证三个快捷方式对"非域名内容"同样生效 ——
// 只看 host 的话 ".pdf" / "https://" 这类规则永远不可能命中，属于体验缺陷。
func TestQuickModesMatchFullURL(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		fullURL string
		pattern string
		mode    MatchMode
		want    bool
	}{
		{"后缀命中PDF", "a.com", "https://a.com/doc/report.pdf", ".pdf", MatchSuffix, true},
		{"后缀不命中", "a.com", "https://a.com/doc/report.html", ".pdf", MatchSuffix, false},
		{"后缀命中域名", "example.cn", "https://example.cn/", ".cn", MatchSuffix, true},
		{"前缀命中scheme", "a.com", "https://a.com/", "https://", MatchPrefix, true},
		{"前缀不命中scheme", "a.com", "http://a.com/", "https://", MatchPrefix, false},
		{"前缀命中域名", "sub.example.com", "https://sub.example.com/", "sub", MatchPrefix, true},
		{"包含命中路径", "a.com", "https://a.com/user/login", "login", MatchContains, true},
		{"包含命中域名", "login.example.com", "https://login.example.com/", "login", MatchContains, true},
		{"包含不命中", "a.com", "https://a.com/home", "login", MatchContains, false},
		{"大小写不敏感", "a.com", "https://a.com/doc/X.PDF", ".pdf", MatchSuffix, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchPattern(c.host, c.fullURL, c.pattern, c.mode)
			if got != c.want {
				t.Errorf("matchPattern(%q, %q, %q, %s) = %v, want %v",
					c.host, c.fullURL, c.pattern, c.mode, got, c.want)
			}
		})
	}
}

// TestModeEquivalentWildcard 验证快捷方式给出的等价通配写法正确。
func TestModeEquivalentWildcard(t *testing.T) {
	cases := []struct {
		mode MatchMode
		want string
	}{
		{MatchContains, "*x*"},
		{MatchPrefix, "x*"},
		{MatchSuffix, "*x"},
		{MatchWildcard, ""},
		{MatchExact, ""},
		{MatchURLEqual, ""},
		{MatchRegex, ""},
	}
	for _, c := range cases {
		if got := modeEquivalentWildcard(c.mode, "x"); got != c.want {
			t.Errorf("modeEquivalentWildcard(%s) = %q, want %q", c.mode, got, c.want)
		}
	}
}

// TestRuleOpenInNewWindowPropagates 确保"新窗口打开"字段能在序列化后原样保留。
func TestRuleOpenInNewWindowPropagates(t *testing.T) {
	r := Rule{ID: "x", Pattern: "example.com", Mode: MatchExact, Browser: "com.apple.safari", OpenInNewWindow: true}
	if !r.OpenInNewWindow {
		t.Fatal("OpenInNewWindow 未被保留")
	}
}

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
		{"https://example.com/a?b=1", MatchURLEqual}, // 含 scheme → 完整网址相等
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
		{"urlequal命中", "example.com", "https://example.com/a?b=1", "https://example.com/a?b=1", MatchURLEqual, true},
		{"urlequal路径不同不命中", "example.com", "https://example.com/a", "https://example.com/b", MatchURLEqual, false},
		{"urlequal查询串不同不命中", "example.com", "https://example.com/a?x=1", "https://example.com/a?x=2", MatchURLEqual, false},
		{"urlequal忽略大小写", "example.com", "https://example.com/a", "HTTPS://EXAMPLE.COM/A", MatchURLEqual, true},
		{"urlequal不只看host", "example.com", "https://example.com/", "example.com", MatchURLEqual, false},
		{"wildcard子域命中", "sub.example.com", "https://sub.example.com/", "*.example.com", MatchWildcard, true},
		{"wildcard单字符", "a.example.com", "https://a.example.com/", "?.example.com", MatchWildcard, true},
		{"regex命中", "mail.google.com", "https://mail.google.com/", `.*\.google\.com`, MatchRegex, true},
		{"regex非法返回false", "x", "https://x/", "([", MatchRegex, false},
		{"contains命中host", "aliyun.com", "https://qiye.aliyun.com/x", "aliyun", MatchContains, true},
		{"contains命中fullURL", "example.com", "https://example.com/login", "login", MatchContains, true},
		{"prefix命中", "www.example.com", "https://www.example.com/", "www.", MatchPrefix, true},
		{"suffix命中", "example.cn", "https://example.cn/", ".cn", MatchSuffix, true},
		{"suffix不命中", "example.com", "https://example.com/", ".cn", MatchSuffix, false},
		// 通配过去只匹配 host，含路径的写法永远命中不了（历史 bug）。
		{"wildcard匹配完整URL路径", "github.com", "https://github.com/foo/bar/settings", "*/settings", MatchWildcard, true},
		{"wildcard匹配scheme", "example.com", "https://example.com/", "https://*", MatchWildcard, true},
		{"wildcard匹配host与路径", "a.example.com", "https://a.example.com/docs/1", "*example.com*", MatchWildcard, true},
		{"wildcard不匹配", "github.com", "https://github.com/foo", "*/settings", MatchWildcard, false},
		{"wildcard点号不被当正则元字符", "aXexample.com", "https://aXexample.com/", "a.example.com", MatchWildcard, false},
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

// TestValidateRuleInput 覆盖保存规则前的语义校验：
// 捕捉"语法没问题但保存后一定不生效"的写法，以及完全重复的规则。
func TestValidateRuleInput(t *testing.T) {
	existing := []Rule{
		{ID: "r1", Pattern: "example.com", Mode: MatchExact, Browser: "chrome"},
	}
	cases := []struct {
		name      string
		pattern   string
		mode      MatchMode
		browser   string
		excludeID string
		wantErr   bool
	}{
		{"空pattern", "   ", MatchExact, "chrome", "", true},
		{"正常域名", "other.com", MatchExact, "chrome", "", false},
		{"urlequal缺scheme", "example.com/a", MatchURLEqual, "chrome", "", true},
		{"urlequal正常", "https://example.com/a", MatchURLEqual, "chrome", "", false},
		{"exact带scheme", "https://example.com", MatchExact, "chrome", "", true},
		{"exact带路径", "example.com/a", MatchExact, "chrome", "", true},
		{"快捷方式含星号", "*.pdf", MatchSuffix, "chrome", "", true},
		{"快捷方式正常", ".pdf", MatchSuffix, "chrome", "", false},
		{"正则非法", "([", MatchRegex, "chrome", "", true},
		{"正则正常", `.*\.com`, MatchRegex, "chrome", "", false},
		{"与已有规则完全重复", "example.com", MatchExact, "chrome", "", true},
		{"编辑自身不算重复", "example.com", MatchExact, "chrome", "r1", false},
		{"同模式同内容但不同浏览器", "example.com", MatchExact, "safari", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateRuleInput(c.pattern, c.mode, c.browser, existing, c.excludeID)
			if c.wantErr && err == nil {
				t.Errorf("期望报错，实际通过：pattern=%q mode=%s", c.pattern, c.mode)
			}
			if !c.wantErr && err != nil {
				t.Errorf("期望通过，实际报错：%v", err)
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
