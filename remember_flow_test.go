package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAddRuleForURLWritesRule 验证"记住选择"→ addRuleForURL 确实把规则写入 cfg 与磁盘，
// 且写入的 pattern 能被 MatchURL 命中（闭环）。
func TestAddRuleForURLWritesRule(t *testing.T) {
	// 隔离配置路径到临时目录，避免污染真实配置。
	tmp := t.TempDir()
	configPath = filepath.Join(tmp, "config.json")

	cfg := &Config{
		Browsers: []Browser{{ID: "com.apple.Safari", Name: "Safari", Exec: "com.apple.Safari"}},
	}
	browser := cfg.Browsers[0]
	url := "https://remember-flow.example.com/some/path"

	// 模拟用户在选择器勾选"记住"并选择 Safari（未选具体账户 → nil）。
	addRuleForURL(url, browser, nil, cfg)

	// 1. 内存中应有一条规则
	if len(cfg.Rules) != 1 {
		t.Fatalf("addRuleForURL 后应有 1 条规则，实际 %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Browser != "com.apple.Safari" {
		t.Errorf("规则浏览器错误：%s", cfg.Rules[0].Browser)
	}

	// 2. 磁盘文件应已写入
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("规则未写入磁盘：%v", err)
	}

	// 3. 闭环：下次打开同一 URL 应命中该规则
	result := MatchURL(url, cfg)
	if !result.Matched || result.Browser == nil || result.Browser.ID != "com.apple.Safari" {
		t.Errorf("写入规则后再匹配未命中：matched=%v", result.Matched)
	}

	// 4. 带 www 的同站也应命中（归一化）
	result2 := MatchURL("https://www.remember-flow.example.com/", cfg)
	if !result2.Matched {
		t.Errorf("带 www 的同站未命中记住的规则")
	}
}

// TestAddRuleForURLKeepsProfile 覆盖"记住了某个子账号/无痕，下次打开必须还是它"：
// 规则里必须存下账户 ID，否则命中规则后仍会用默认账户打开（历史 bug）。
func TestAddRuleForURLKeepsProfile(t *testing.T) {
	tmp := t.TempDir()
	configPath = filepath.Join(tmp, "config.json")

	cfg := &Config{
		Browsers: []Browser{{ID: "com.google.Chrome", Name: "Google Chrome", Exec: "com.google.Chrome"}},
	}
	browser := cfg.Browsers[0]
	incognito := Profile{ID: IncognitoProfileID, Name: "无痕", Kind: "incognito"}
	url := "https://profile-flow.example.com/a/b"

	// 模拟用户在选择器勾选"记住"，并从账户菜单里选了"无痕"。
	addRuleForURL(url, browser, &incognito, cfg)

	if len(cfg.Rules) != 1 {
		t.Fatalf("应有 1 条规则，实际 %d", len(cfg.Rules))
	}
	if cfg.Rules[0].Profile != IncognitoProfileID {
		t.Fatalf("规则未记录账户，Profile=%q", cfg.Rules[0].Profile)
	}

	// 闭环：下次打开同一 URL，命中规则要能把账户带出来（launchForRule 据此选择启动方式）。
	result := MatchURL(url, cfg)
	if !result.Matched || result.Rule == nil {
		t.Fatalf("未命中记住的规则")
	}
	if result.Rule.Profile != IncognitoProfileID {
		t.Errorf("匹配结果未带出账户：%q", result.Rule.Profile)
	}

	// 同一域名换一个账户记住，应是另一条规则（不覆盖、不去重）。
	work := Profile{ID: "Profile 1", Name: "工作", Kind: "profile"}
	addRuleForURL(url, browser, &work, cfg)
	if len(cfg.Rules) != 2 {
		t.Errorf("同域名不同账户应记为两条规则，实际 %d", len(cfg.Rules))
	}

	// 而"整浏览器（默认账户）"与"无痕"也是两个不同目标，不应互相去重。
	addRuleForURL(url, browser, nil, cfg)
	if len(cfg.Rules) != 3 {
		t.Errorf("整浏览器与具体账户应分别记一条，实际 %d", len(cfg.Rules))
	}

	// 重复选择同一账户不应再新增。
	addRuleForURL(url, browser, &incognito, cfg)
	if len(cfg.Rules) != 3 {
		t.Errorf("重复记住同一账户应去重，实际 %d", len(cfg.Rules))
	}
}

// TestProfilesNeedChoice 覆盖"是否必须在打开前挑账户"的判定：
// 只有多个真实账户才必须选；仅"默认 + 无痕"时点击卡片应直接打开。
func TestProfilesNeedChoice(t *testing.T) {
	cases := []struct {
		name     string
		profiles []Profile
		want     bool
	}{
		{"无配置档", nil, false},
		{"只有无痕", []Profile{{ID: IncognitoProfileID, Kind: "incognito"}}, false},
		{"默认+无痕", []Profile{{ID: "Default", Kind: "default"}, {ID: IncognitoProfileID, Kind: "incognito"}}, false},
		{"两个真实账户", []Profile{{ID: "Default", Kind: "default"}, {ID: "Profile 1", Kind: "profile"}, {ID: IncognitoProfileID, Kind: "incognito"}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := profilesNeedChoice(c.profiles); got != c.want {
				t.Errorf("profilesNeedChoice(%v) = %v, want %v", c.profiles, got, c.want)
			}
		})
	}
}
