.PHONY: build test vet clean app dmg all app-x64 app-arm64 app-universal dmg-x64 dmg-arm64 dmg-universal

BINARY=browser-switch
GOFLAGS=-ldflags="-s -w"
VERSION?=1.0.1

# 构建 macOS 可执行文件（必须开启 CGO：依赖 CoreServices / CoreFoundation / Carbon）
build:
	CGO_ENABLED=1 go build $(GOFLAGS) -o $(BINARY) .

# 运行纯函数测试套件
test:
	CGO_ENABLED=1 go test ./...

# 静态检查
vet:
	CGO_ENABLED=1 go vet ./...

# macOS：构建可分发的 .app bundle（dist/<arch>/Browser Switch.app，默认当前主机架构）
app:
	./scripts/build-app.sh

# macOS：打包成拖拽安装的 DMG（dist/BrowserSwitch-$(VERSION)-<arch>.dmg，默认当前主机架构）
dmg:
	VERSION=$(VERSION) ./scripts/build-dmg.sh

# 指定架构打包（amd64 = Intel Mac x64，arm64 = Apple Silicon，universal = 通用包）
app-x64:
	./scripts/build-app.sh amd64
app-arm64:
	./scripts/build-app.sh arm64
app-universal:
	./scripts/build-app.sh universal

dmg-x64:
	VERSION=$(VERSION) ./scripts/build-dmg.sh amd64
dmg-arm64:
	VERSION=$(VERSION) ./scripts/build-dmg.sh arm64
dmg-universal:
	VERSION=$(VERSION) ./scripts/build-dmg.sh universal

# 一键编译全部版本安装包（amd64 + arm64 + universal 三个 DMG）
all:
	VERSION=$(VERSION) ./scripts/build-dmg.sh amd64
	VERSION=$(VERSION) ./scripts/build-dmg.sh arm64
	VERSION=$(VERSION) ./scripts/build-dmg.sh universal

# 清理构建产物
clean:
	rm -rf dist $(BINARY)
