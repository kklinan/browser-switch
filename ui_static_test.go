package main

import (
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 本文件是静态检查，不跑业务代码：
//  1. 所有出现的 i18n key 必须在 7 个语言包里都存在（漏翻会在界面上直接显示 key）
//  2. 语言包里不能有代码不再引用的 key（留着会让人以为它还在用）
//  3. 界面层不得出现 emoji（各平台字形不同，会立刻暴露非原生观感）

func goFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(os.DirFS("."), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (p == ".git" || strings.HasPrefix(p, "scripts")) {
			return fs.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

var i18nKeyRe = regexp.MustCompile(`i18n\.Tf?\("([^"]+)"`)

func TestI18nKeysCovered(t *testing.T) {
	var used []string
	seen := map[string]bool{}
	for _, p := range goFiles(t) {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range i18nKeyRe.FindAllStringSubmatch(string(b), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				used = append(used, m[1])
			}
		}
	}
	if len(used) < 100 {
		t.Fatalf("只解析出 %d 个 key，正则可能失效", len(used))
	}
	sort.Strings(used)

	entries, err := os.ReadDir("i18n/locales")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile("i18n/locales/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		// (?m) 不可省：Go 的 ^ 默认只匹配文本开头，不加就只会拿到第一个 key。
		have := map[string]bool{}
		for _, m := range regexp.MustCompile(`(?m)^\s*"([^"]+)":`).FindAllStringSubmatch(string(b), -1) {
			have[m[1]] = true
		}
		var missing, extra []string
		for _, k := range used {
			if !have[k] {
				missing = append(missing, k)
			}
		}
		for k := range have {
			if !seen[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		if len(missing) > 0 {
			t.Errorf("%s 缺失 %d 个 key: %v", e.Name(), len(missing), missing)
		}
		if len(extra) > 0 {
			t.Errorf("%s 有 %d 个代码未引用的 key: %v", e.Name(), len(extra), extra)
		}
	}
}

// emojiRe 匹配彩色 emoji 与常见符号字母。
// 用反引号字符串：Go 的双引号字符串不支持 \x{...} 转义。
var emojiRe = regexp.MustCompile(`[\x{1F000}-\x{1FAFF}\x{2600}-\x{27BF}\x{FE0F}\x{2B00}-\x{2BFF}]`)

func TestNoEmojiInUI(t *testing.T) {
	for _, p := range goFiles(t) {
		if strings.HasSuffix(p, "_test.go") || p == "icons_darwin.go" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			// 注释里提到 emoji 是允许的（本文档就在解释为什么不用）
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, m := range emojiRe.FindAllString(line, -1) {
				// 「→」等 CJK 语境里的常规符号不算 emoji，这里只拦截彩色 emoji 段
				if strings.ContainsAny(m, "→←↑↓") {
					continue
				}
				t.Errorf("%s:%d 界面代码含 emoji %q: %s", p, i+1, m, strings.TrimSpace(line))
			}
		}
	}
}

// TestGofmtClean 保证提交前格式化（与 CLAUDE.md 的 `make check` 一致，
// 但只检查本次改动涉及的界面文件，避免历史文件噪音）。
func TestGofmtClean(t *testing.T) {
	files := []string{
		"gui.go", "picker.go", "main.go",
		"settings.go", "settings_browsers.go", "settings_rules.go", "settings_general.go",
		"urlhandler_darwin.go", "ui_smoke_test.go",
	}
	var bad []string
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		out, _ := exec.Command("gofmt", "-l", f).Output()
		if len(strings.TrimSpace(string(out))) > 0 {
			bad = append(bad, f)
		}
	}
	if len(bad) > 0 {
		t.Errorf("未 gofmt: %v（运行 gofmt -w %s）", bad, strings.Join(bad, " "))
	}
}
