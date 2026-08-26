#!/usr/bin/env python3
# make-icns.py —— 从画稿生成符合 macOS 模板的应用图标
#
# 变换：把满幅方形画稿缩进 Apple 标准安全区（1024 画布中 824 本体、四周 100px 边距），
# 套标准圆角（r≈185），再用 iconutil 打包成 assets/BrowserSwitch.icns。
# 这样图标在 Dock / Launchpad 里的尺寸与圆角才能与 Chrome 等系统应用一致。
#
# 依赖：Pillow（pip3 install Pillow）+ macOS 自带 iconutil。仅 dev 期手动运行，
# 不参与 make build。
#
# 源图应为满幅方形实心原图（无圆角、无边距）——圆角与安全区边距由本脚本统一套用。
# 默认按 logo.png > 1024x1024.png > 512x512.png 的顺序取分辨率最高的候选。
#
# 用法：
#   python3 scripts/make-icns.py                 # 默认取 assets/ 下最优候选源
#   python3 scripts/make-icns.py assets/foo.png  # 指定源图
#
import os
import subprocess
import sys
import tempfile

try:
    from PIL import Image, ImageChops, ImageDraw
except ImportError:
    sys.exit("需要 Pillow：pip3 install Pillow")

# 仓库根目录（脚本位于 scripts/ 下）
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASSETS = os.path.join(ROOT, "assets")

# Apple macOS 图标模板（1024 画布）
CANVAS = 1024
BODY = 824                       # 本体尺寸（占画布 80.5%）
MARGIN = (CANVAS - BODY) // 2    # 四周边距 100px
RADIUS = 185                     # 圆角半径（≈185.4）

# icns 标准 10 档：(边长, 文件名)
ICONSET = [
    (16, "icon_16x16.png"), (32, "icon_16x16@2x.png"),
    (32, "icon_32x32.png"), (64, "icon_32x32@2x.png"),
    (128, "icon_128x128.png"), (256, "icon_128x128@2x.png"),
    (256, "icon_256x256.png"), (512, "icon_256x256@2x.png"),
    (512, "icon_512x512.png"), (1024, "icon_512x512@2x.png"),
]


def pick_source():
    """选源图：命令行参数优先；否则在 assets/ 里挑分辨率最高的候选。"""
    if len(sys.argv) > 1:
        return sys.argv[1]
    for name in ("logo.png", "1024x1024.png", "512x512.png"):
        p = os.path.join(ASSETS, name)
        if os.path.exists(p):
            return p
    sys.exit("未找到源图：请在 assets/ 放 logo.png / 1024x1024.png / 512x512.png，或用参数指定路径")


def build_master(src_path):
    """把满幅画稿转成合规母图（居中本体 + 圆角），返回 RGBA 图像。"""
    src = Image.open(src_path).convert("RGBA")
    if src.width < BODY:
        print(f"⚠️  源图仅 {src.width}px，放大到 {BODY}px 本体会损失清晰度；建议提供 ≥{CANVAS}px 画稿")
    body = src.resize((BODY, BODY), Image.LANCZOS)

    # 圆角遮罩叠加到本体 alpha 上（覆盖画稿原有的细小圆角）
    mask = Image.new("L", (BODY, BODY), 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, BODY - 1, BODY - 1], radius=RADIUS, fill=255)
    _, _, _, a = body.split()
    body.putalpha(ImageChops.multiply(a, mask))

    # 贴到透明画布，居中留边距
    canvas = Image.new("RGBA", (CANVAS, CANVAS), (0, 0, 0, 0))
    canvas.paste(body, (MARGIN, MARGIN), body)
    return canvas


def make_icns(master):
    """由母图生成 iconset 并用 iconutil 打包成 icns。"""
    icns_path = os.path.join(ASSETS, "BrowserSwitch.icns")
    with tempfile.TemporaryDirectory() as tmp:
        # iconutil 只认以 .iconset 结尾的目录
        iconset = os.path.join(tmp, "AppIcon.iconset")
        os.makedirs(iconset)
        for size, name in ICONSET:
            master.resize((size, size), Image.LANCZOS).save(os.path.join(iconset, name))
        subprocess.run(["iconutil", "-c", "icns", iconset, "-o", icns_path], check=True)
    return icns_path


def main():
    src = pick_source()
    print(f"源图：{os.path.relpath(src, ROOT)}")
    master = build_master(src)
    master_path = os.path.join(ASSETS, "1024.png")
    master.save(master_path)
    print(f"合规母图：{os.path.relpath(master_path, ROOT)}  (本体 {BODY}/{CANVAS}, 圆角 {RADIUS})")
    icns = make_icns(master)
    print(f"图标：{os.path.relpath(icns, ROOT)}  ✅")


if __name__ == "__main__":
    main()
