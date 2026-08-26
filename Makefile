.PHONY: build test vet clean app dmg

BINARY=browser-switch
GOFLAGS=-ldflags="-s -w"
VERSION?=1.0.0

# 构建 macOS 可执行文件（必须开启 CGO：依赖 CoreServices / CoreFoundation / Carbon）
build:
	CGO_ENABLED=1 go build $(GOFLAGS) -o $(BINARY) .

# 运行纯函数测试套件
test:
	CGO_ENABLED=1 go test ./...

# 静态检查
vet:
	CGO_ENABLED=1 go vet ./...

# macOS：构建可分发的 .app bundle（dist/Browser Switch.app）
app:
	./scripts/build-app.sh

# macOS：打包成拖拽安装的 DMG（dist/BrowserSwitch-$(VERSION).dmg）
dmg:
	VERSION=$(VERSION) ./scripts/build-dmg.sh

# 清理构建产物
clean:
	rm -rf dist $(BINARY)
