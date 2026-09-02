<!--
Browser Switch — 無料・オープンソースの macOS デフォルトブラウザピッカー兼サイト別 URL ルーター。
キーワード：macOS デフォルトブラウザ、ブラウザピッカー、ブラウザ選択、サイト別ブラウザ振り分け、
URL ルーティング、リンクを別のブラウザで開く、マルチプロファイル起動、Chrome プロファイル切り替え、
Finicky 代替、Velja 代替、Browserosaurus 代替、Choosy 代替。
-->

# Browser Switch — macOS デフォルトブラウザピッカー & サイト別 URL ルーター 🌐

**Browser Switch** は、無料・オープンソースの **macOS デフォルトブラウザピッカー**です。デフォルトブラウザに設定すると、クリックしたすべてのリンクが自分のルールに従って振り分けられます。仕事用リンクは Edge、個人用は Chrome、開発用は Firefox で自動的に開けます。ルールに一致しない場合は、キーボード操作に優れた**ブラウザ選択画面**が表示され、その場で選べます。

<p>
<a href="README.md">English</a> ·
<a href="README.zh-CN.md">简体中文</a> ·
<a href="README.ja.md"><b>日本語</b></a> ·
<a href="README.ko.md">한국어</a>
</p>

![platform](https://img.shields.io/badge/platform-macOS%2010.14%2B-black)
![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)
![GUI](https://img.shields.io/badge/GUI-Fyne%20v2.7-blue)
![license](https://img.shields.io/badge/license-Apache_2.0-green)

> **macOS 専用。** デフォルトブラウザ登録は CoreServices（cgo）、URL 受信は Carbon Apple Event API を使用しており、いずれも macOS 固有です。このリポジトリに Linux / Windows 向けのプラットフォームファイルはありません。

---

## 目次

- [なぜ Browser Switch？](#なぜ-browser-switch)
- [機能](#機能)
- [仕組み](#仕組み)
- [インストール](#インストール)
- [使い方](#使い方)
- [ルールマッチング](#ルールマッチング)
- [マルチプロファイルとアカウント別お気に入り](#マルチプロファイルとアカウント別お気に入り)
- [設定](#設定)
- [アーキテクチャ](#アーキテクチャ)
- [ソースからのビルド](#ソースからのビルド)
- [アンインストール](#アンインストール)
- [類似ツールとの比較](#類似ツールとの比較)
- [FAQ](#faq)
- [既知の制限](#既知の制限)
- [ドキュメント](#ドキュメント)
- [ライセンス](#ライセンス)

---

## なぜ Browser Switch？

複数のブラウザや複数のブラウザアカウントを毎日使い分けるなら、macOS は 1 つのデフォルトブラウザ・1 つのワークフローしか許しません。Browser Switch はこれを解決します。

- **サイト別にリンクを振り分け。** ルールエンジンがドメインに基づき各 URL を適切なブラウザへ送ります。ブラウザ間でのリンクのコピペはもう不要です。
- **アカウント別にリンクを振り分け。** macOS のリンクルーターの中でも独自の機能として、**特定の Chrome/Edge/Firefox プロファイル**（仕事用 Google アカウントと個人用など）へリンクを送れます。
- **リンクを取りこぼさない。** ルールに一致しない場合はネイティブのピッカーがカウントダウン付きで表示され、リンクが失われることはありません。
- **ネイティブで軽量。** システム標準の AppKit を使う単一の Go バイナリ。~150 MB の Electron アプリではありません。

---

## 機能

| 機能 | 説明 |
| ---- | ---- |
| 🎯 **URL インターセプト** | macOS の `http`/`https` ハンドラーとして登録し、Carbon Apple Event で URL を直接受信 |
| 📋 **7 つのルールモード** | ドメイン一致 / URL 完全一致 / ワイルドカード / 正規表現 / 部分一致 / 前方一致 / 後方一致。優先度の降順で評価 |
| 🖱️ **カード型ピッカー** | ブラウザアイコンのグリッド。4 個を超えると「もっと見る(⌘R)」カードに折りたたみ |
| ⌨️ **キーボード優先** | `⌘1`〜`⌘9` または数字キーで N 番目のブラウザを起動。`⌘R` で全リストを展開。`Enter` でデフォルト、`Esc` でキャンセル |
| 🪟 **新規ウィンドウルール** | 任意のルールで、既存ウィンドウの再利用ではなく常に新しいウィンドウで開くよう指定可能 |
| 📝 **ルールの編集** | ルールは追加・削除だけでなく、その場で編集可能 |
| ⏱️ **カウントダウンフォールバック** | 設定可能な時間（既定 5 秒）経過後に**デフォルトブラウザ**で自動的に開き、リンクが止まらない |
| 💾 **選択を記憶** | チェックすると、そのドメインの完全一致ルールを自動生成（優先度 100） |
| 👥 **マルチプロファイル対応** | Chromium（Chrome/Edge/Brave/Vivaldi/Opera）と Firefox のプロファイルを自動検出。シークレットも付属 |
| ⭐ **お気に入りと並び順** | ピッカーに表示するブラウザ（およびアカウント）と順序を自由に設定（⌘N 番号を決定） |
| 🌍 **7 言語** | 簡体字/繁体字中国語、英語、日本語、韓国語、ポルトガル語、ヒンディー語。ビルド時に埋め込み |
| ♻️ **クリーンなアンインストール** | インストール前に有効だったデフォルトブラウザを復元 |

---

## 仕組み

```
リンクをクリック
    ↓
macOS LaunchServices が Browser Switch.app に GetURL Apple Event を配信
    ↓
アプリが URL をルールと（優先度順に）照合
    ├── 一致  → マッピングされたブラウザで直接開き（UI なし）終了
    └── 不一致
        ├── show_picker_on_miss = false → デフォルトブラウザで開いて終了
        └── show_picker_on_miss = true  → ピッカーを表示
            ├── カードをクリック / ⌘N / Enter → 選んだブラウザ（またはプロファイル）で開く
            ├── Esc                          → キャンセル
            └── カウントダウンが 0 に         → デフォルトブラウザで開く
```

Browser Switch は**単一アプリ**です。自身をシステムの `http`/`https` ハンドラーとして登録し、Carbon Apple Event ハンドラー（`kInternetEventClass` / `kAEGetURL`）をインストールして URL を直接受信します。AppleScript フォワーダーは不要です。

---

## インストール

### 必要環境

ビルドには Xcode Command Line Tools のみ必要です（cgo 用の CoreServices / Carbon ヘッダーを提供）。

```bash
xcode-select --install
```

実行時の依存はすべて macOS 標準コマンドです：`plutil`、`sips`、`open`、`codesign`、`xattr`、`lsregister`。

### ビルドとインストール

```bash
# 1. ビルド（CGO は必須）
make build
# 等価:
CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

# 2. デフォルトブラウザとしてインストール
./browser-switch --install       # ~/Applications/Browser Switch.app を作成し登録
./browser-switch --check-default # 確認
```

`--install` の処理：

1. 現在の実行ファイルを `~/Applications/Browser Switch.app/Contents/MacOS/browser-switch` にコピー
2. `http`/`https` URL スキームを宣言する `Info.plist` を書き込み
3. アドホックコード署名を行い LaunchServices（`lsregister`）に登録
4. 現在のデフォルトブラウザを記録（アンインストール時の復元用）
5. `LSSetDefaultHandlerForURLScheme` を呼び出し。反映されない場合は**システム設定 → 一般**を開く

> macOS のセキュリティポリシーにより、デフォルトブラウザ変更をシステム設定で一度確認するよう求められる場合があります。正常な動作です。

### 配布用 DMG のパッケージング

```bash
# 現在のアーキテクチャの .app をビルド
make app
# またはアーキテクチャを指定（amd64 = Intel Mac x64、arm64 = Apple Silicon）
make app-x64
make app-arm64
make app-universal     # lipo で統合したユニバーサルバイナリ（両アーキテクチャ対応）

# ドラッグ＆ドロップでインストールできる DMG をパッケージング
make dmg              # 現在のアーキテクチャ
make dmg-x64          # Intel (Mac x64)
make dmg-arm64        # Apple Silicon
make dmg-universal    # ユニバーサル
make all              # 3 種類の DMG を一括ビルド

VERSION=1.2.0 make all  # バージョン指定
```

出力先は `dist/`：

| ファイル | 対象 |
|------|------|
| `dist/BrowserSwitch-<バージョン>-amd64.dmg` | Intel Mac (x64) |
| `dist/BrowserSwitch-<バージョン>-arm64.dmg` | Apple Silicon |
| `dist/BrowserSwitch-<バージョン>-universal.dmg` | 両アーキテクチャ対応 |

---

## 使い方

### コマンドライン

```bash
browser-switch https://example.com   # ルール照合 / ピッカー表示
browser-switch --settings            # 設定ウィンドウを開く
browser-switch --installer           # インストールウィザード UI を開く
browser-switch --list-browsers       # 検出したブラウザを一覧（⭐ はデフォルト）
browser-switch --list-profiles       # 各ブラウザのプロファイルを一覧
browser-switch --test https://github.com  # ブラウザを開かずルール照合をテスト
browser-switch --check-default       # システムデフォルトか確認
browser-switch --install             # インストールしデフォルトに登録
browser-switch --uninstall           # アンインストールし以前のデフォルトを復元
browser-switch --version             # バージョン情報
```

### ピッカー操作

| 入力 | 動作 |
| ---- | ---- |
| カードを左クリック | そのブラウザで開く。**複数プロファイルがある場合はアカウントメニューを表示** |
| カードを右クリック | アカウントメニューを表示（マルチプロファイルのみ） |
| `⌘1`〜`⌘9` / `1`〜`9` | N 番目のブラウザを直接起動（既定プロファイル） |
| `⌘R` | 全ブラウザリストを展開 — 「もっと見る」カードのクリックと同じ |
| `Enter` | デフォルトブラウザで開く |
| `Esc` | 何も開かずキャンセル |
| 上部 URL バーをクリック | 完全な URL をクリップボードにコピー |
| 「このドメインを記憶」 | 今回の選択を `exact` ルールとして書き込み |
| 歯車 / コピーボタン | 設定を開く / URL をクリップボードにコピー |

カウントダウンが 0 になると、ハイライト中のカードではなく**デフォルトブラウザ**（設定の `default_browser`）が使われます。歯車ボタンから設定を開くと**カウントダウンは一時停止**するため、ルール編集中にピッカーが自動で開くことはありません。

### 設定ウィンドウ

3 つのタブ：

- **ブラウザ**——左：お気に入り一覧（並べ替え / 削除、順序 = ⌘N 番号）、右：全ブラウザ（お気に入り ♥ / 非表示 👁 / アカウント展開 / 再スキャン）
- **ルール**——優先度の降順で全ルールを表示。追加・編集（✎）・削除。各ルールは**新しいウィンドウで開く**よう設定可能
- **一般**——言語、デフォルトブラウザ、自動オープン秒数、ルール不一致時の動作（ピッカーを表示、または指定ブラウザで直接開く）、インストール/アンインストール、「別のブラウザをシステムデフォルトに設定」

---

## ルールマッチング

| モード | 照合対象 | 例 |
| ------ | -------- | -- |
| `exact` | host が一致 | `github.com` → github.com のみ、sub.github.com は不可 |
| `urlequal` | URL 全体が一致 | `https://github.com/a/b` → その URL のみ、`?x=1` 付きは不可 |
| `wildcard` | host **または**完全 URL、`*` `?` 対応 | `*.google.com` → mail.google.com；`*/settings` → 任意の設定ページ |
| `regex` | host **または**完全 URL | `.*\.(test\|staging)\..*` |
| `contains` | host **または**完全 URL の部分文字列 | `login` → example.com/login |
| `prefix` | host **または**完全 URL の前方 | `dev.` → dev.example.com；`https://` → すべての安全なリンク |
| `suffix` | host **または**完全 URL の後方 | `.cn` → example.cn；`.pdf` → すべての PDF リンク |

- ルールは `priority` の**降順**で評価され、最初に一致したものが採用されます。
- 照合では元の host と `www.` を除いた host の両方を試すため、`example.com` のルールは `www.example.com` にも一致します。
- `contains` / `prefix` / `suffix` はワイルドカードの**ショートカット**（`*p*` / `p*` / `*p` と等価）です。glob や正規表現を書きたくないユーザー向けで、`*` と `?` は**解釈されず**そのままの文字として扱われます。そのようなルールは決して一致しないため、保存時にブロックされます。
- `exact` / `urlequal` 以外のモードは host と完全 URL の**両方**を試すため、パス（`*/settings`）やスキーム（`https://`）にしか現れない内容でも正しく一致します。
- 「選択を記憶」で生成されるルールの優先度は `100` 固定、手動追加は既定 `50` です。
- 任意のルールを**新しいウィンドウで開く**よう設定できます。Chrome / Edge / Safari は単一インスタンスで `open -n` が黙って無視されるため、`--new-window`（Chromium/Firefox）または AppleScript（Safari）を使用します。

ブラウザを開かずに任意の URL をテスト：

```bash
browser-switch --test https://mail.google.com/u/1/inbox
```

---

## マルチプロファイルとアカウント別お気に入り

Browser Switch は各ブラウザで設定済みのプロファイルを読み取ります。

- **Chromium 系**（Chrome、Edge、Brave、Vivaldi、Opera）：`~/Library/Application Support/<app>/Local State` から
- **Firefox**：`~/Library/Application Support/Firefox/profiles.ini` から
- マルチプロファイルのブラウザには合成の**シークレット / プライベート**項目も付きます。

**アカウント別お気に入り。** 「ブラウザ」タブでマルチアカウントのブラウザを展開し、各アカウント行の ♥ をクリックします。お気に入りにしたアカウントはピッカーに**独立したカード**（「ブラウザ · アカウント」というタイトル）として表示され、独自の ⌘N 番号を持ち、クリックするとそのプロファイルで即座に起動します（サブメニューなし）。これは複合お気に入りキーとして保存されます：ブラウザ全体は `bundleID`、特定アカウントは `bundleID#profileID`。プロファイルを削除すると、対応する無効なお気に入りは自動的にスキップされます。

プロファイルの起動はブラウザバイナリを直接実行し `--profile-directory=`（Chromium）または `-P`（Firefox）を渡します。ブラウザが起動済みの場合 `open -b` はこれらの引数を無視するためです。

---

## 設定

設定ファイル：`~/.config/browser-switch/config.json`（初回起動時に検出済みブラウザとともに自動生成）。

```json
{
  "default_browser": "com.google.Chrome",
  "browsers": [
    {
      "id": "com.google.Chrome",
      "name": "Google Chrome",
      "exec": "com.google.Chrome",
      "desktop": "/Applications/Google Chrome.app",
      "icon": "com.google.Chrome"
    }
  ],
  "favorites": ["com.google.Chrome", "com.google.Chrome#Profile 1", "com.apple.Safari"],
  "hidden": [],
  "rules": [
    {
      "id": "work",
      "pattern": "*.company.com",
      "mode": "wildcard",
      "browser": "com.microsoft.edgemac",
      "priority": 100,
      "enabled": true,
      "comment": "仕事用サイトは Edge で",
      "open_in_new_window": false
    }
  ],
  "auto_close_delay": 5,
  "show_picker_on_miss": true,
  "language": "",
  "prev_default_browser": "com.apple.safari"
}
```

| フィールド | 説明 |
| ---------- | ---- |
| `default_browser` | ブラウザ ID（macOS では bundle ID）。ルール不一致かつピッカー無効時に使用。カウントダウンの復帰先でもある |
| `favorites` | ピッカーの順序。単純な bundle ID = ブラウザ全体、`bundleID#profileID` = 特定アカウント。空なら全件（`hidden` を除く）へ |
| `hidden` | ピッカーと一覧から隠すブラウザ ID（誤検出の非ブラウザアプリを抑制） |
| `auto_close_delay` | カウントダウン秒数。`0` で自動オープン無効 |
| `show_picker_on_miss` | `false` で不一致時にピッカーを出さずデフォルトブラウザで開く |
| `language` | 空でシステムに追従。それ以外は `zh-CN` / `zh-TW` / `en` / `ja` / `ko` / `pt` / `hi` |
| `prev_default_browser` | インストール時に記録、アンインストール時に復元。未設定なら Safari へ |

---

## アーキテクチャ

```
main.go            → CLI ディスパッチ、コマンドライン URL 経路（handleURL）、インストーラー UI
config.go          → Config / Browser / Rule / Profile 型 + JSON 永続化
rules.go           → MatchURL ルールエンジン、ValidatePattern、SuggestMatchMode
picker.go          → ピッカーウィンドウ、カウントダウン、ショートカット、「選択を記憶」
settings.go        → 設定ウィンドウ（3 タブ）
gui.go             → 共有 Fyne コンポーネント（card、progressLine、アイコン/テキスト補助）
constants.go       → アプリ名と bundle ID
browsers_darwin.go → .app + CFBundleURLTypes でブラウザ検出、open -b で起動
install_darwin.go  → .app 生成、アドホック署名、LaunchServices デフォルトハンドラー（cgo）
urlhandler_darwin.go → Carbon Apple Event で URL 受信（cgo）、単一アプリのメインループ
profiles_darwin.go → Chromium Local State / Firefox profiles.ini の検出と起動
icons_darwin.go    → .icns → PNG の抽出とキャッシュ
i18n/              → 7 つの埋め込みロケール + T/Tf 翻訳関数
```

**常駐しない設計。** クリックごとに新規のコールドスタートで、処理後に終了します。Fyne の glfw ドライバーは Dock の「Reopen」イベントを Carbon ハンドラーへ転送しないため、常駐プロセスは「再クリック」を認識できません。コールドスタートにより動作が予測可能になります：Dock アイコンをクリックすれば必ず設定が開き、リンクをクリックすれば必ずピッカーが出ます。

製品仕様の全文は [PRD.md](PRD.md)、アーキテクチャ制約は [CLAUDE.md](CLAUDE.md) を参照してください。

---

## ソースからのビルド

```bash
git clone https://github.com/kklinan/browser-switch.git
cd browser-switch
make build        # CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

go test ./...     # 純関数テストスイートを実行
go vet ./...
```

> `Makefile` のターゲットは `build` / `test` / `vet` / `app` / `dmg` / `clean` です。`make app` と `make dmg` で配布用の `.app` と DMG を生成します（macOS 専用）。

---

## アンインストール

```bash
./browser-switch --uninstall     # 以前のデフォルトブラウザを復元 + ~/Applications/Browser Switch.app を削除
rm -rf ~/.config/browser-switch  # 設定を削除
rm -rf /tmp/browser-switch-icons # アイコンキャッシュを削除
```

---

## 類似ツールとの比較

| | **Browser Switch** | Velja | Browserosaurus | Finicky | Choosy |
| --- | --- | --- | --- | --- | --- |
| 価格 | **無料・OSS** | 無料 / アプリ内課金 | 無料・OSS | 無料・OSS | 有料 |
| ルールエンジン | **6 モード + 優先度** | あり | なし（ピッカーのみ） | あり（JS 設定） | あり |
| GUI ルールエディター | **あり** | あり | — | なし（JS 編集） | あり |
| **アカウント / プロファイル別振り分け** | **✅ あり** | なし | なし | なし | なし |
| ピッカーウィンドウ | **あり** | あり | あり | なし | あり |
| ネイティブバイナリ | **あり（Go/AppKit）** | あり | なし（~150 MB Electron） | あり | あり |
| カウントダウンフォールバック | **あり** | なし | なし | なし | なし |

Browser Switch のニッチ：**Finicky のルール性能 + Browserosaurus の GUI の手軽さ + どこにもないアカウント別振り分け。**

---

## FAQ

**macOS でサイトごとに別のデフォルトブラウザを設定するには？**
Browser Switch をデフォルトブラウザとしてインストールし、ドメインをブラウザに対応付けるルールを追加します（設定 → ルール、または JSON を編集）。クリックごとに自動で振り分けられます。

**特定の Chrome や Firefox プロファイルで自動的にリンクを開けますか？**
はい。Browser Switch はブラウザのプロファイルを検出し、個別アカウントをピッカーの独立カードとしてお気に入り登録できます。プロファイル別の自動**ルール**はロードマップにあります（[ROADMAP.md](docs/ROADMAP.md) §3.1）。

**Velja / Finicky / Choosy の代替として良いですか？**
無料・オープンソース・ネイティブの代替です。特徴はアカウント（プロファイル）別振り分けで、他ツールにはありません。Finicky は JS 設定ファイルが必要ですが、Browser Switch は GUI を備えます。

**Linux や Windows で動きますか？**
いいえ。設計上 macOS 専用です。デフォルトブラウザ登録と URL 受信が macOS 固有 API に依存します。

**なぜターミナルや WebView アプリがブラウザ一覧に出るのですか？**
`Info.plist` に `http`/`https` ハンドラーを宣言したアプリはすべて候補として検出されます（macOS の「デフォルトブラウザ」候補と同じ基準）。誤検出は設定 → ブラウザで非表示にできます。

**リンクを失うことはありますか？**
ありません。ルールに一致せず離席しても、カウントダウンが自動的にデフォルトブラウザで開きます。

**設定はどこに保存されますか？**
`~/.config/browser-switch/config.json`。編集可能でマシン間同期もできるプレーンな JSON です。

---

## 既知の制限

1. WebView を持つ一部の非ブラウザアプリが `http` ハンドラーを宣言して検出されます。設定 → ブラウザで非表示にしてください。
2. デフォルトブラウザ変更時に macOS がシステム設定での手動確認を求める場合があります。
3. 新しくインストールしたブラウザは自動表示されません。設定で「再スキャン」をクリックしてください。
4. ルール UI は追加と削除のみ対応。編集・有効/無効切替・優先度変更は JSON を直接編集してください。
5. アプリはアドホック署名で Apple の公証を受けていないため、初回起動時に Gatekeeper が確認を求める場合があります。
6. 問題の全リストは [docs/ISSUES.md](docs/ISSUES.md) を参照してください。

---

## ドキュメント

| ドキュメント | 目的 |
| --- | --- |
| [English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [한국어](README.ko.md) | ユーザーガイド（本ファイル） |
| [PRD.md](PRD.md) | 実装に照合済みの製品要求仕様 |
| [CLAUDE.md](CLAUDE.md) | AI コーディングアシスタント向けアーキテクチャ制約（信頼できる唯一の情報源） |
| [docs/ISSUES.md](docs/ISSUES.md) | コード位置付きの既知の問題一覧 |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 製品ロードマップ |

---

## ライセンス

[Apache-2.0](LICENSE)
