package main

import "testing"

// TestFavKeyCodec 覆盖复合收藏 key 的编码与解码互逆性。
func TestFavKeyCodec(t *testing.T) {
	cases := []struct {
		browserID string
		profileID string
		wantKey   string
	}{
		{"com.google.Chrome", "", "com.google.Chrome"},
		{"com.google.Chrome", "Profile 1", "com.google.Chrome#Profile 1"},
		{"com.google.Chrome", "__incognito__", "com.google.Chrome#__incognito__"},
		{"org.mozilla.firefox", "default-release", "org.mozilla.firefox#default-release"},
	}
	for _, c := range cases {
		got := encodeFavKey(c.browserID, c.profileID)
		if got != c.wantKey {
			t.Errorf("encodeFavKey(%q,%q) = %q, want %q", c.browserID, c.profileID, got, c.wantKey)
		}
		bid, pid := decodeFavKey(got)
		if bid != c.browserID || pid != c.profileID {
			t.Errorf("decodeFavKey(%q) = (%q,%q), want (%q,%q)", got, bid, pid, c.browserID, c.profileID)
		}
	}
}

// TestDecodeFavKeyFirstHash 验证按首个 '#' 切分：profile ID 内若含 '#' 也能正确归入 profile 段。
func TestDecodeFavKeyFirstHash(t *testing.T) {
	bid, pid := decodeFavKey("com.google.Chrome#weird#name")
	if bid != "com.google.Chrome" || pid != "weird#name" {
		t.Errorf("decodeFavKey 首个 '#' 切分错误：bid=%q pid=%q", bid, pid)
	}
}

// TestFavoriteItemsBackwardCompat 验证旧配置（favorites 为纯 bundleID 数组）行为不变。
func TestFavoriteItemsBackwardCompat(t *testing.T) {
	cfg := &Config{
		Browsers: []Browser{
			{ID: "com.google.Chrome", Name: "Google Chrome", Exec: "com.google.Chrome"},
			{ID: "com.apple.Safari", Name: "Safari", Exec: "com.apple.Safari"},
		},
		Favorites: []string{"com.apple.Safari", "com.google.Chrome"},
	}
	items := cfg.FavoriteItems()
	if len(items) != 2 {
		t.Fatalf("应返回 2 个收藏项，实际 %d", len(items))
	}
	if items[0].Browser.ID != "com.apple.Safari" || items[0].Profile != nil {
		t.Errorf("首项应为整浏览器 Safari，实际 id=%s profile=%v", items[0].Browser.ID, items[0].Profile)
	}
	// FavoriteBrowsers 兼容包装应保持顺序且只含浏览器维度
	bs := cfg.FavoriteBrowsers()
	if len(bs) != 2 || bs[0].ID != "com.apple.Safari" || bs[1].ID != "com.google.Chrome" {
		t.Errorf("FavoriteBrowsers 兼容包装结果错误：%+v", bs)
	}
}

// TestFavoriteItemsDanglingBrowserSkipped 验证浏览器被卸载后，其收藏 key 被静默跳过。
func TestFavoriteItemsDanglingBrowserSkipped(t *testing.T) {
	cfg := &Config{
		Browsers:  []Browser{{ID: "com.apple.Safari", Name: "Safari", Exec: "com.apple.Safari"}},
		Favorites: []string{"com.uninstalled.Browser", "com.apple.Safari"},
	}
	items := cfg.FavoriteItems()
	if len(items) != 1 || items[0].Browser.ID != "com.apple.Safari" {
		t.Errorf("悬空浏览器收藏应被跳过，仅剩 Safari，实际 %+v", items)
	}
}

// TestFavoriteItemsHiddenExcluded 验证被隐藏的浏览器不出现在收藏项中。
func TestFavoriteItemsHiddenExcluded(t *testing.T) {
	cfg := &Config{
		Browsers: []Browser{
			{ID: "com.google.Chrome", Name: "Google Chrome", Exec: "com.google.Chrome"},
			{ID: "com.apple.Safari", Name: "Safari", Exec: "com.apple.Safari"},
		},
		Favorites: []string{"com.google.Chrome", "com.apple.Safari"},
		Hidden:    []string{"com.google.Chrome"},
	}
	items := cfg.FavoriteItems()
	if len(items) != 1 || items[0].Browser.ID != "com.apple.Safari" {
		t.Errorf("隐藏的 Chrome 应被排除，仅剩 Safari，实际 %+v", items)
	}
}

// TestFavoriteItemKey 验证 FavoriteItem.Key 与 encodeFavKey 一致。
func TestFavoriteItemKey(t *testing.T) {
	b := Browser{ID: "com.google.Chrome", Name: "Google Chrome"}
	whole := FavoriteItem{Browser: b}
	if whole.Key() != "com.google.Chrome" {
		t.Errorf("整浏览器 Key 错误：%q", whole.Key())
	}
	withProfile := FavoriteItem{Browser: b, Profile: &Profile{ID: "Profile 1", Name: "Work"}}
	if withProfile.Key() != "com.google.Chrome#Profile 1" {
		t.Errorf("账户 Key 错误：%q", withProfile.Key())
	}
}
