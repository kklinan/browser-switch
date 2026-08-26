package main

import "testing"

// decideAction 是从 handleURL 中提取的纯决策逻辑，便于测试（不触发 GUI）。
// 返回值：action ∈ {"launch", "picker"}，以及 launch 时选定的浏览器 ID。
func decideAction(url string, cfg *Config) (action string, browserID string) {
	result := MatchURL(url, cfg)
	if result.Matched && result.Browser != nil {
		return "launch", result.Browser.ID
	}
	if !cfg.ShowPickerOnMiss {
		def := findBrowserByID(cfg.DefaultBrowser, cfg)
		if def == nil {
			if list := cfg.FavoriteBrowsers(); len(list) > 0 {
				def = &list[0]
			}
		}
		if def != nil {
			return "launch", def.ID
		}
	}
	return "picker", ""
}

func testCfg() *Config {
	return &Config{
		DefaultBrowser: "com.google.Chrome",
		Browsers: []Browser{
			{ID: "com.google.Chrome", Name: "Google Chrome", Exec: "com.google.Chrome"},
			{ID: "com.apple.Safari", Name: "Safari", Exec: "com.apple.Safari"},
		},
	}
}

// TestShowPickerOnMiss_True：无规则匹配 + 开启弹窗 → 弹选择器。
func TestShowPickerOnMiss_True(t *testing.T) {
	cfg := testCfg()
	cfg.ShowPickerOnMiss = true
	action, _ := decideAction("https://nomatch.example.com/", cfg)
	if action != "picker" {
		t.Errorf("开启 ShowPickerOnMiss 且无匹配时应弹选择器，实际 action=%q", action)
	}
}

// TestShowPickerOnMiss_False：无规则匹配 + 关闭弹窗 → 直接用默认浏览器。
func TestShowPickerOnMiss_False(t *testing.T) {
	cfg := testCfg()
	cfg.ShowPickerOnMiss = false
	action, id := decideAction("https://nomatch.example.com/", cfg)
	if action != "launch" || id != "com.google.Chrome" {
		t.Errorf("关闭 ShowPickerOnMiss 应直接用默认浏览器，实际 action=%q id=%q", action, id)
	}
}

// TestShowPickerOnMiss_False_InvalidDefault：关闭弹窗但默认浏览器无效 → 回退首个可用浏览器。
func TestShowPickerOnMiss_False_InvalidDefault(t *testing.T) {
	cfg := testCfg()
	cfg.ShowPickerOnMiss = false
	cfg.DefaultBrowser = "com.nonexistent.browser"
	action, id := decideAction("https://nomatch.example.com/", cfg)
	if action != "launch" || id == "" {
		t.Errorf("默认浏览器无效时应回退首个可用浏览器，实际 action=%q id=%q", action, id)
	}
}

// TestRuleMatchAlwaysLaunches：规则命中时无论 ShowPickerOnMiss 如何都直接打开。
func TestRuleMatchAlwaysLaunches(t *testing.T) {
	for _, show := range []bool{true, false} {
		cfg := testCfg()
		cfg.ShowPickerOnMiss = show
		cfg.Rules = []Rule{{
			ID: "r", Pattern: "match.example.com", Mode: MatchExact,
			Browser: "com.apple.Safari", Priority: 100, Enabled: true,
		}}
		action, id := decideAction("https://match.example.com/", cfg)
		if action != "launch" || id != "com.apple.Safari" {
			t.Errorf("ShowPickerOnMiss=%v 时规则命中应直接打开 Safari，实际 action=%q id=%q", show, action, id)
		}
	}
}
