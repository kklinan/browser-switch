package main

import "testing"

// TestRememberChoiceFlow 复现"记住选择"→ 下次直接匹配的完整链路。
func TestRememberChoiceFlow(t *testing.T) {
	cases := []string{
		"https://qiye.aliyun.com/",
		"https://www.qiye.aliyun.com/",
		"https://qiye.aliyun.com/path?q=1",
	}
	for _, url := range cases {
		cfg := &Config{
			Browsers: []Browser{{ID: "com.google.Chrome", Name: "Google Chrome", Exec: "com.google.Chrome"}},
		}
		browser := cfg.Browsers[0]

		// 写入规则（复刻 addRuleForURL 的核心，不触发 SaveConfig 磁盘写）
		host := extractDomain(url)
		cfg.Rules = append(cfg.Rules, Rule{
			ID: "test", Pattern: host, Mode: MatchExact,
			Browser: browser.ID, Priority: 100, Enabled: true,
		})

		// 下次打开同一 URL 应命中
		result := MatchURL(url, cfg)
		if !result.Matched {
			t.Errorf("URL %q: pattern=%q 写入后未匹配自身！MatchLog=%s", url, host, result.MatchLog)
		} else {
			t.Logf("URL %q: pattern=%q → 匹配成功 (%s)", url, host, result.Browser.Name)
		}
	}
}

// TestWWWConsistency 专门验证 www 前缀在写入侧与匹配侧是否一致。
func TestWWWConsistency(t *testing.T) {
	writeSide := extractDomain("https://www.qiye.aliyun.com/")
	cfg := &Config{Browsers: []Browser{{ID: "b", Name: "B", Exec: "b"}}}
	cfg.Rules = append(cfg.Rules, Rule{ID: "r", Pattern: writeSide, Mode: MatchExact, Browser: "b", Priority: 100, Enabled: true})

	// 用户下次可能输入带 www 或不带 www 的同站 URL
	for _, u := range []string{"https://www.qiye.aliyun.com/", "https://qiye.aliyun.com/"} {
		r := MatchURL(u, cfg)
		t.Logf("写入pattern=%q, 访问%q → Matched=%v", writeSide, u, r.Matched)
		if !r.Matched {
			t.Errorf("www 不一致：pattern=%q 无法匹配 %q", writeSide, u)
		}
	}
}
