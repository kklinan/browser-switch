//go:build darwin

// 单 App 方案：让本应用直接作为 macOS 的 http/https 处理器接收 URL，
// 取代原先的 AppleScript Forwarder + Helper 双 App 架构。
//
// macOS 通过 Apple Event（kInternetEventClass/kAEGetURL）向注册为 URL 处理器的
// 应用投递 URL，而非命令行参数。因此这里用 cgo 注册 GetURL 事件处理器，
// 收到的 URL 送入 Go channel，由常驻的 Fyne 事件循环弹出选择器。
package main

/*
#cgo LDFLAGS: -framework CoreServices -framework Carbon
#cgo CFLAGS: -Wno-deprecated-declarations
#include <CoreServices/CoreServices.h>
#include <Carbon/Carbon.h>
#include <stdlib.h>

extern void goHandleAppleEventURL(char* url);

// bpHandleGetURL 从 GetURL Apple Event 中取出 URL 字符串并回调 Go。
static OSErr bpHandleGetURL(const AppleEvent* ev, AppleEvent* reply, void* ctx) {
    AEDesc d;
    d.descriptorType = typeNull;
    d.dataHandle = NULL;
    OSErr err = AEGetParamDesc(ev, keyDirectObject, typeWildCard, &d);
    if (err != noErr) return err;
    Size sz = AEGetDescDataSize(&d);
    char* buf = (char*)malloc(sz + 1);
    if (buf != NULL) {
        if (AEGetDescData(&d, buf, sz) == noErr) {
            buf[sz] = '\0';
            goHandleAppleEventURL(buf);
        }
        free(buf);
    }
    AEDisposeDesc(&d);
    return noErr;
}

// bpInstallURLHandler 注册 GetURL 事件处理器（进程级，需在事件循环启动前调用）。
static void bpInstallURLHandler() {
    AEInstallEventHandler(kInternetEventClass, kAEGetURL,
        NewAEEventHandlerUPP((AEEventHandlerProcPtr)bpHandleGetURL), 0, false);
}
*/
import "C"

import (
	"sync/atomic"
	"time"

	"github.com/kklinan/browser-switch/i18n"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

// urlEventCh 承接来自 Apple Event 的 URL；带缓冲避免回调阻塞。
var urlEventCh = make(chan string, 8)

//export goHandleAppleEventURL
func goHandleAppleEventURL(curl *C.char) {
	u := C.GoString(curl)
	select {
	case urlEventCh <- u:
	default:
	}
}

func installURLHandler() {
	C.bpInstallURLHandler()
}

// runAppModeGUI 单 App 主循环（非常驻）：
//   - 注册 URL 事件处理器
//   - 冷启动后短时间内收到 URL → 弹出选择器；选择/取消后进程退出
//   - 未收到 URL（用户主动点击 App）→ 显示设置窗口；关闭后进程退出
//
// 采用"用完即退"而非常驻：macOS 上 glfw 驱动不会把 Reopen(点击 Dock) 事件
// 转发给 Carbon Apple Event 处理器，常驻进程收不到"再次点击"通知。非常驻下每次
// 点击 Dock / 打开链接都是全新冷启动，行为可预测：点击必显示设置、链接必弹选择器。
func runAppModeGUI(cfg *Config) {
	installURLHandler()

	a := app.New()

	// master 窗口在做出"URL 还是设置"决策前维持事件循环；决策后按用途复用或关闭。
	master := a.NewWindow(i18n.T("app.full_name"))
	master.SetContent(widget.NewLabel(""))
	master.Resize(fyne.NewSize(1, 1))

	var decided int32
	showSettingsInMaster := func() {
		if !atomic.CompareAndSwapInt32(&decided, 0, 1) {
			return
		}
		master.SetContent(buildSettingsContent(a, cfg, master))
		master.SetTitle(i18n.T("settings.window_title"))
		master.Resize(fyne.NewSize(760, 560))
		master.CenterOnScreen()
		master.Show() // 关闭即退出进程（master 是唯一窗口）
	}
	openPicker := func(u string) {
		// 关键：URL 经系统 handler（Apple Event）进来时，必须先走规则匹配 ——
		// 命中"记住"写入的规则则直接用对应浏览器打开，不弹选择器；否则才展示选择器。
		result := MatchURL(u, cfg)
		if result.Matched && result.Browser != nil {
			if err := LaunchBrowser(*result.Browser, u); err == nil {
				if atomic.CompareAndSwapInt32(&decided, 0, 1) {
					a.Quit() // 首个 URL 已命中规则并打开，无需 GUI，退出进程
				}
				return
			}
			// 启动失败则继续走选择器兜底。
		}
		// 无匹配且用户关闭了"无匹配弹选择器"：直接用默认/首个浏览器打开。
		if !cfg.ShowPickerOnMiss {
			def := findBrowserByID(cfg.DefaultBrowser, cfg)
			if def == nil {
				if list := cfg.FavoriteBrowsers(); len(list) > 0 {
					def = &list[0]
				}
			}
			if def != nil {
				_ = LaunchBrowser(*def, u)
				if atomic.CompareAndSwapInt32(&decided, 0, 1) {
					a.Quit()
				}
				return
			}
		}

		if !atomic.CompareAndSwapInt32(&decided, 0, 1) {
			// 已在处理其它 URL：再开一个独立选择器窗口即可。
			w := a.NewWindow(i18n.T("picker.window_title"))
			setupPickerWindow(a, w, u, cfg, func() { w.Close() })
			w.Show()
			return
		}
		// 首个 URL：用 picker 取代隐藏的 master 作为主窗口，取消/选择后退出进程。
		w := a.NewWindow(i18n.T("picker.window_title"))
		setupPickerWindow(a, w, u, cfg, func() { a.Quit() })
		w.Show()
	}

	// 消费 URL 事件（冷启动早期到达）。
	go func() {
		for u := range urlEventCh {
			uu := u
			fyne.Do(func() { openPicker(uu) })
		}
	}()

	// 冷启动短暂等待：若未收到 URL，判定为用户主动打开 App，显示设置窗口。
	go func() {
		time.Sleep(500 * time.Millisecond)
		fyne.Do(showSettingsInMaster)
	}()

	a.Run()
}
