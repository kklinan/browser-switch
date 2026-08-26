<!--
Browser Switch — 무료 오픈소스 macOS 기본 브라우저 선택기 및 사이트별 URL 라우터.
키워드: macOS 기본 브라우저, 브라우저 선택기, 브라우저 선택, 사이트별 브라우저 규칙,
URL 라우팅, 링크를 다른 브라우저로 열기, 멀티 프로필 실행기, Chrome 프로필 전환,
Finicky 대안, Velja 대안, Browserosaurus 대안, Choosy 대안.
-->

# Browser Switch — macOS 기본 브라우저 선택기 & 사이트별 URL 라우터 🌐

**Browser Switch** 는 무료 오픈소스 **macOS 기본 브라우저 선택기**입니다. 기본 브라우저로 설정하면 클릭하는 모든 링크가 사용자가 정한 규칙에 따라 라우팅됩니다. 업무 링크는 Edge, 개인 링크는 Chrome, 개발 링크는 Firefox로 자동으로 열립니다. 일치하는 규칙이 없으면 키보드 조작에 최적화된 **브라우저 선택 창**이 나타나 즉시 선택할 수 있습니다.

<p>
<a href="README.md">English</a> ·
<a href="README.zh-CN.md">简体中文</a> ·
<a href="README.ja.md">日本語</a> ·
<a href="README.ko.md"><b>한국어</b></a>
</p>

![platform](https://img.shields.io/badge/platform-macOS%2010.14%2B-black)
![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8)
![GUI](https://img.shields.io/badge/GUI-Fyne%20v2.7-blue)
![license](https://img.shields.io/badge/license-Apache_2.0-green)

> **macOS 전용.** 기본 브라우저 등록은 CoreServices(cgo), URL 수신은 Carbon Apple Event API를 사용하며 모두 macOS 고유 기능입니다. 이 저장소에는 Linux / Windows 플랫폼 파일이 없습니다.

---

## 목차

- [왜 Browser Switch인가?](#왜-browser-switch인가)
- [기능](#기능)
- [작동 방식](#작동-방식)
- [설치](#설치)
- [사용법](#사용법)
- [규칙 매칭](#규칙-매칭)
- [멀티 프로필 및 계정별 즐겨찾기](#멀티-프로필-및-계정별-즐겨찾기)
- [설정](#설정)
- [아키텍처](#아키텍처)
- [소스에서 빌드](#소스에서-빌드)
- [제거](#제거)
- [유사 도구 비교](#유사-도구-비교)
- [FAQ](#faq)
- [알려진 제한](#알려진-제한)
- [문서](#문서)
- [라이선스](#라이선스)

---

## 왜 Browser Switch인가?

여러 브라우저나 여러 브라우저 계정을 매일 오간다면, macOS는 하나의 기본 브라우저와 하나의 워크플로만 허용합니다. Browser Switch가 이를 해결합니다.

- **사이트별로 링크 라우팅.** 규칙 엔진이 도메인에 따라 각 URL을 알맞은 브라우저로 보냅니다. 더 이상 브라우저 간에 링크를 복사·붙여넣기할 필요가 없습니다.
- **계정별로 링크 라우팅.** macOS 링크 라우터 중 유일하게, **특정 Chrome/Edge/Firefox 프로필**(업무용 Google 계정 vs 개인 계정)로 링크를 보낼 수 있습니다.
- **링크를 놓치지 않음.** 일치하는 규칙이 없으면 카운트다운 폴백이 있는 네이티브 선택기가 나타나 링크가 사라지지 않습니다.
- **네이티브하고 가벼움.** 시스템 자체 AppKit을 사용하는 단일 Go 바이너리. ~150 MB Electron 앱이 아닙니다.

---

## 기능

| 기능 | 설명 |
| ---- | ---- |
| 🎯 **URL 인터셉트** | macOS `http`/`https` 핸들러로 등록되어 Carbon Apple Event로 URL을 직접 수신 |
| 📋 **6가지 규칙 모드** | 정확 일치 / 와일드카드 / 정규식 / 포함 / 접두 / 접미. 우선순위 내림차순으로 평가 |
| 🖱️ **카드형 선택기** | 브라우저 아이콘 그리드. 4개를 넘으면 "더 보기" 카드로 접힘 |
| ⌨️ **키보드 우선** | `⌘1`~`⌘9` 또는 숫자 키로 N번째 브라우저 실행. `Enter`는 기본, `Esc`는 취소 |
| ⏱️ **카운트다운 폴백** | 설정 가능한 시간(기본 5초) 후 **기본 브라우저**로 자동으로 열려 링크가 멈추지 않음 |
| 💾 **선택 기억** | 체크하면 해당 도메인의 정확 일치 규칙을 자동 생성(우선순위 100) |
| 👥 **멀티 프로필 지원** | Chromium(Chrome/Edge/Brave/Vivaldi/Opera)과 Firefox 프로필을 자동 감지. 시크릿 포함 |
| ⭐ **즐겨찾기 및 정렬** | 선택기에 표시할 브라우저(및 계정)와 순서를 직접 설정(⌘N 번호 결정) |
| 🌍 **7개 언어** | 간체/번체 중국어, 영어, 일본어, 한국어, 포르투갈어, 힌디어. 빌드 시 내장 |
| ♻️ **깔끔한 제거** | 설치 전에 활성화되어 있던 기본 브라우저를 복원 |

---

## 작동 방식

```
링크 클릭
    ↓
macOS LaunchServices가 Browser Switch.app에 GetURL Apple Event 전달
    ↓
앱이 URL을 규칙과 (우선순위대로) 대조
    ├── 일치  → 매핑된 브라우저로 바로 열고(UI 없음) 종료
    └── 불일치
        ├── show_picker_on_miss = false → 기본 브라우저로 열고 종료
        └── show_picker_on_miss = true  → 선택기 표시
            ├── 카드 클릭 / ⌘N / Enter → 선택한 브라우저(또는 프로필)로 열기
            ├── Esc                    → 취소
            └── 카운트다운 0 도달       → 기본 브라우저로 열기
```

Browser Switch는 **단일 앱**입니다. 자신을 시스템 `http`/`https` 핸들러로 등록하고 Carbon Apple Event 핸들러(`kInternetEventClass` / `kAEGetURL`)를 설치해 URL을 직접 수신합니다. AppleScript 포워더가 필요 없습니다.

---

## 설치

### 요구 사항

빌드에는 Xcode Command Line Tools만 필요합니다(cgo용 CoreServices / Carbon 헤더 제공).

```bash
xcode-select --install
```

모든 런타임 의존성은 macOS 내장 명령입니다: `plutil`, `sips`, `open`, `codesign`, `xattr`, `lsregister`.

### 빌드 및 설치

```bash
# 1. 빌드(CGO 필수)
make build
# 동일:
CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

# 2. 기본 브라우저로 설치
./browser-switch --install       # ~/Applications/Browser Switch.app 생성 및 등록
./browser-switch --check-default # 확인
```

`--install` 동작:

1. 현재 실행 파일을 `~/Applications/Browser Switch.app/Contents/MacOS/browser-switch`로 복사
2. `http`/`https` URL 스킴을 선언하는 `Info.plist` 작성
3. 애드혹 코드 서명 후 LaunchServices(`lsregister`)에 등록
4. 현재 기본 브라우저 기록(제거 시 복원용)
5. `LSSetDefaultHandlerForURLScheme` 호출. 적용되지 않으면 **시스템 설정 → 일반** 열기

> macOS 보안 정책상 기본 브라우저 변경을 시스템 설정에서 한 번 확인하라는 요청이 나올 수 있습니다. 정상적인 동작입니다.

---

## 사용법

### 명령줄

```bash
browser-switch https://example.com   # 규칙 매칭 / 선택기 표시
browser-switch --settings            # 설정 창 열기
browser-switch --installer           # 설치 마법사 UI 열기
browser-switch --list-browsers       # 감지된 브라우저 목록(⭐ 는 기본)
browser-switch --list-profiles       # 각 브라우저의 프로필 목록
browser-switch --test https://github.com  # 브라우저를 열지 않고 규칙 매칭 테스트
browser-switch --check-default       # 시스템 기본 여부 확인
browser-switch --install             # 설치 및 기본으로 등록
browser-switch --uninstall           # 제거 및 이전 기본 복원
browser-switch --version             # 버전 정보
```

### 선택기 조작

| 입력 | 동작 |
| ---- | ---- |
| 카드 좌클릭 | 해당 브라우저로 열기. **프로필이 여러 개면 계정 메뉴 표시** |
| 카드 우클릭 | 계정 메뉴 표시(멀티 프로필 전용) |
| `⌘1`~`⌘9` / `1`~`9` | N번째 브라우저를 바로 실행(기본 프로필 사용) |
| `Enter` | 기본 브라우저로 열기 |
| `Esc` | 아무것도 열지 않고 취소 |
| "이 도메인 기억" | 이번 선택을 `exact` 규칙으로 저장 |
| 톱니바퀴 / 복사 버튼 | 설정 열기 / URL을 클립보드에 복사 |

카운트다운이 0이 되면 강조된 카드가 아니라 **기본 브라우저**(설정의 `default_browser`)가 사용됩니다.

### 설정 창

3개 탭:

- **브라우저**——왼쪽: 즐겨찾기 목록(순서 변경 / 제거, 순서 = ⌘N 번호), 오른쪽: 전체 브라우저(즐겨찾기 ♥ / 숨기기 👁 / 계정 펼치기 / 다시 스캔)
- **규칙**——우선순위 내림차순으로 전체 규칙 표시. 추가 및 삭제
- **일반**——언어, 기본 브라우저, 자동 열기 초, 규칙 불일치 시 동작(선택기 표시 또는 지정 브라우저로 바로 열기), 설치/제거, "다른 브라우저를 시스템 기본으로 설정"

---

## 규칙 매칭

| 모드 | 대조 대상 | 예 |
| ---- | --------- | -- |
| `exact` | host 완전 일치 | `github.com` → github.com만, sub.github.com 제외 |
| `wildcard` | host, `*` `?` 지원 | `*.google.com` → mail.google.com |
| `regex` | host **또는** 전체 URL | `.*\.(test\|staging)\..*` |
| `contains` | host **또는** 전체 URL 부분 문자열 | `login` → example.com/login |
| `prefix` | host 접두 | `dev.` → dev.example.com |
| `suffix` | host 접미 | `.cn` → example.cn |

- 규칙은 `priority` **내림차순**으로 평가되며 처음 일치한 것이 채택됩니다.
- 매칭 시 원본 host와 `www.`를 제거한 host를 모두 시도하므로, `example.com` 규칙은 `www.example.com`에도 일치합니다.
- "선택 기억"으로 생성된 규칙의 우선순위는 `100` 고정, 수동 추가는 기본 `50`입니다.

브라우저를 열지 않고 임의의 URL 테스트:

```bash
browser-switch --test https://mail.google.com/u/1/inbox
```

---

## 멀티 프로필 및 계정별 즐겨찾기

Browser Switch는 각 브라우저에 구성된 프로필을 읽어옵니다.

- **Chromium 계열**(Chrome, Edge, Brave, Vivaldi, Opera): `~/Library/Application Support/<app>/Local State`에서
- **Firefox**: `~/Library/Application Support/Firefox/profiles.ini`에서
- 멀티 프로필 브라우저에는 합성 **시크릿 / 비공개** 항목도 추가됩니다.

**계정별 즐겨찾기.** "브라우저" 탭에서 멀티 계정 브라우저를 펼치고 각 계정 행의 ♥ 를 클릭합니다. 즐겨찾기한 계정은 선택기에 **독립 카드**("브라우저 · 계정" 제목)로 나타나 고유한 ⌘N 번호를 가지며, 클릭하면 해당 프로필로 즉시 실행됩니다(하위 메뉴 없음). 이는 복합 즐겨찾기 키로 저장됩니다: 브라우저 전체는 `bundleID`, 특정 계정은 `bundleID#profileID`. 프로필을 삭제하면 대응하는 잘못된 즐겨찾기는 자동으로 건너뜁니다.

프로필 실행은 브라우저 바이너리를 직접 실행하며 `--profile-directory=`(Chromium) 또는 `-P`(Firefox)를 전달합니다. 브라우저가 이미 실행 중이면 `open -b`가 이 인자를 무시하기 때문입니다.

---

## 설정

설정 파일: `~/.config/browser-switch/config.json`(최초 실행 시 감지된 브라우저와 함께 자동 생성).

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
      "comment": "업무 사이트는 Edge로"
    }
  ],
  "auto_close_delay": 5,
  "show_picker_on_miss": true,
  "language": "",
  "prev_default_browser": "com.apple.safari"
}
```

| 필드 | 설명 |
| ---- | ---- |
| `default_browser` | 브라우저 ID(macOS에서는 bundle ID). 규칙 불일치 및 선택기 비활성 시 사용. 카운트다운 폴백 대상이기도 함 |
| `favorites` | 선택기 순서. 순수 bundle ID = 브라우저 전체, `bundleID#profileID` = 특정 계정. 비어 있으면 전체(`hidden` 제외)로 |
| `hidden` | 선택기와 목록에서 숨길 브라우저 ID(오탐된 비브라우저 앱 억제) |
| `auto_close_delay` | 카운트다운 초. `0`이면 자동 열기 비활성 |
| `show_picker_on_miss` | `false`면 불일치 시 선택기 없이 기본 브라우저로 열기 |
| `language` | 비어 있으면 시스템 따름. 그 외 `zh-CN` / `zh-TW` / `en` / `ja` / `ko` / `pt` / `hi` |
| `prev_default_browser` | 설치 시 기록, 제거 시 복원. 미설정이면 Safari로 |

---

## 아키텍처

```
main.go            → CLI 디스패치, 명령줄 URL 경로(handleURL), 인스톨러 UI
config.go          → Config / Browser / Rule / Profile 타입 + JSON 영속화
rules.go           → MatchURL 규칙 엔진, ValidatePattern, SuggestMatchMode
picker.go          → 선택기 창, 카운트다운, 단축키, "선택 기억"
settings.go        → 설정 창(3개 탭)
gui.go             → 공유 Fyne 컴포넌트(card, progressLine, 아이콘/텍스트 유틸)
constants.go       → 앱 이름과 bundle ID
browsers_darwin.go → .app + CFBundleURLTypes로 브라우저 감지, open -b로 실행
install_darwin.go  → .app 생성, 애드혹 서명, LaunchServices 기본 핸들러(cgo)
urlhandler_darwin.go → Carbon Apple Event로 URL 수신(cgo), 단일 앱 메인 루프
profiles_darwin.go → Chromium Local State / Firefox profiles.ini 감지 및 실행
icons_darwin.go    → .icns → PNG 추출 및 캐시
i18n/              → 7개 내장 로케일 + T/Tf 번역 함수
```

**비상주 설계.** 클릭할 때마다 새 콜드 스타트로 처리 후 종료합니다. Fyne의 glfw 드라이버는 Dock "Reopen" 이벤트를 Carbon 핸들러로 전달하지 않으므로 상주 프로세스는 "다시 클릭"을 인식할 수 없습니다. 콜드 스타트 덕분에 동작이 예측 가능합니다: Dock 아이콘을 클릭하면 항상 설정이 열리고, 링크를 클릭하면 항상 선택기가 나옵니다.

전체 제품 사양은 [PRD.md](PRD.md), 아키텍처 제약은 [CLAUDE.md](CLAUDE.md)를 참조하세요.

---

## 소스에서 빌드

```bash
git clone https://github.com/kklinan/browser-switch.git
cd browser-switch
make build        # CGO_ENABLED=1 go build -ldflags="-s -w" -o browser-switch .

go test ./...     # 순수 함수 테스트 스위트 실행
go vet ./...
```

> `Makefile` 타깃은 `build` / `test` / `vet` / `app` / `dmg` / `clean` 입니다. `make app` 과 `make dmg` 로 배포용 `.app` 과 DMG를 생성합니다(macOS 전용).

---

## 제거

```bash
./browser-switch --uninstall     # 이전 기본 브라우저 복원 + ~/Applications/Browser Switch.app 삭제
rm -rf ~/.config/browser-switch  # 설정 삭제
rm -rf /tmp/browser-switch-icons # 아이콘 캐시 삭제
```

---

## 유사 도구 비교

| | **Browser Switch** | Velja | Browserosaurus | Finicky | Choosy |
| --- | --- | --- | --- | --- | --- |
| 가격 | **무료·오픈소스** | 무료 / 인앱 결제 | 무료·오픈소스 | 무료·오픈소스 | 유료 |
| 규칙 엔진 | **6개 모드 + 우선순위** | 있음 | 없음(선택기만) | 있음(JS 설정) | 있음 |
| GUI 규칙 편집기 | **있음** | 있음 | — | 없음(JS 편집) | 있음 |
| **계정 / 프로필별 라우팅** | **✅ 있음** | 없음 | 없음 | 없음 | 없음 |
| 선택기 창 | **있음** | 있음 | 있음 | 없음 | 있음 |
| 네이티브 바이너리 | **있음(Go/AppKit)** | 있음 | 없음(~150 MB Electron) | 있음 | 있음 |
| 카운트다운 폴백 | **있음** | 없음 | 없음 | 없음 | 없음 |

Browser Switch의 틈새: **Finicky의 규칙 성능 + Browserosaurus의 GUI 간편함 + 어디에도 없는 계정별 라우팅.**

---

## FAQ

**macOS에서 웹사이트마다 다른 기본 브라우저를 설정하려면?**
Browser Switch를 기본 브라우저로 설치한 뒤 도메인을 브라우저에 매핑하는 규칙을 추가합니다(설정 → 규칙, 또는 JSON 편집). 클릭할 때마다 자동으로 라우팅됩니다.

**특정 Chrome 또는 Firefox 프로필로 링크를 자동으로 열 수 있나요?**
네. Browser Switch는 브라우저 프로필을 감지하고 개별 계정을 선택기 독립 카드로 즐겨찾기할 수 있습니다. 프로필별 자동 **규칙**은 로드맵에 있습니다([ROADMAP.md](docs/ROADMAP.md) §3.1).

**Velja / Finicky / Choosy 대안으로 좋은가요?**
무료, 오픈소스, 네이티브 대안입니다. 특징은 계정(프로필)별 라우팅으로 다른 도구에는 없습니다. Finicky는 JS 설정 파일이 필요하지만 Browser Switch는 GUI를 제공합니다.

**Linux나 Windows에서 동작하나요?**
아니요. 설계상 macOS 전용입니다. 기본 브라우저 등록과 URL 수신이 macOS 고유 API에 의존합니다.

**왜 터미널이나 WebView 앱이 브라우저 목록에 나오나요?**
`Info.plist`에 `http`/`https` 핸들러를 선언한 앱은 모두 후보로 감지됩니다(macOS "기본 브라우저" 후보와 동일 기준). 오탐은 설정 → 브라우저에서 숨길 수 있습니다.

**링크를 잃어버릴 수 있나요?**
아니요. 규칙에 일치하지 않고 자리를 비워도 카운트다운이 자동으로 기본 브라우저로 엽니다.

**설정은 어디에 저장되나요?**
`~/.config/browser-switch/config.json`. 편집 가능하고 기기 간 동기화도 되는 순수 JSON입니다.

---

## 알려진 제한

1. WebView를 가진 일부 비브라우저 앱이 `http` 핸들러를 선언해 감지됩니다. 설정 → 브라우저에서 숨기세요.
2. 기본 브라우저 변경 시 macOS가 시스템 설정에서 수동 확인을 요구할 수 있습니다.
3. 새로 설치한 브라우저는 자동으로 나타나지 않습니다. 설정에서 "다시 스캔"을 클릭하세요.
4. 규칙 UI는 추가와 삭제만 지원합니다. 편집·활성/비활성·우선순위 변경은 JSON을 직접 편집하세요.
5. 앱은 애드혹 서명되어 Apple 공증을 받지 않았으므로 최초 실행 시 Gatekeeper가 확인을 요구할 수 있습니다.
6. 전체 이슈 목록은 [docs/ISSUES.md](docs/ISSUES.md)를 참조하세요.

---

## 문서

| 문서 | 용도 |
| --- | --- |
| [English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [한국어](README.ko.md) | 사용자 가이드(본 파일) |
| [PRD.md](PRD.md) | 구현에 대조 검증된 제품 요구 사양 |
| [CLAUDE.md](CLAUDE.md) | AI 코딩 어시스턴트용 아키텍처 제약(단일 진실 공급원) |
| [docs/ISSUES.md](docs/ISSUES.md) | 코드 위치가 포함된 알려진 이슈 목록 |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 제품 로드맵 |

---

## 라이선스

[Apache-2.0](LICENSE)
