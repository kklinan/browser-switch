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

	// 模拟用户在选择器勾选"记住"并选择 Safari。
	addRuleForURL(url, browser, cfg)

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
