# Makefile for vexo SSH desktop application
VERSION ?= v1.0.0
MODE ?= release
APP_NAME ?= Vexo

# 获取 Git 信息（完整 hash）
GIT_INFO := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
# RFC3339-ish time; avoid GNU-only %:z so Windows/macOS date works
BUILD_TIME := $(shell date -Iseconds 2>/dev/null || date "+%Y-%m-%dT%H:%M:%S%z" 2>/dev/null || echo "unknown")

CONFIG_FILE := internal/buildinfo/buildinfo.go
CONFIG_BACKUP := internal/buildinfo/buildinfo.go.bak

LINUX_ARCHIVE := bin/$(APP_NAME)_$(VERSION)_linux_amd64.tar.gz
WINDOWS_ARCHIVE := bin/$(APP_NAME)_$(VERSION)_windows_amd64.zip
SHA256SUMS_FILE := bin/SHA256SUMS

.PHONY: help all build-windows build-linux clean build-mac build-mac-intel replace-config restore-config update-build-assets pack-release

.DEFAULT_GOAL := help

help:
	@echo "Vexo SSH desktop application — Makefile usage"
	@echo ""
	@echo "Usage: make [target] [VAR=value ...]"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION   Release version (default: $(VERSION))"
	@echo "  MODE      Build mode: debug | release (default: $(MODE))"
	@echo ""
	@echo "Targets:"
	@echo "  help            Show this help (default)"
	@echo "  all             Build all platforms (windows, darwin amd64/arm64, linux)"
	@echo "  build-windows   Build for Windows $(VERSION)"
	@echo "  build-mac       Build for macOS (arm64) $(VERSION)"
	@echo "  build-mac-intel Build for macOS (amd64) $(VERSION)"
	@echo "  build-linux     Build for Linux $(VERSION)"
	@echo "  pack-release    Build linux/windows, archive, create draft GitHub release"
	@echo "  clean           Remove bin/ and config backup"
	@echo "  update-build-assets Update build directory assets with the current Wails CLI"
	@echo "  replace-config  Inject VERSION/MODE/GIT_INFO/BUILD_TIME into config (internal)"
	@echo "  restore-config  Restore buildinfo.go from backup (internal)"

# Build all platforms
all: build-windows build-mac build-mac-intel build-linux

# 替换配置文件中的变量
replace-config:
	@echo "Replacing build variables in $(CONFIG_FILE)..."
	@cp $(CONFIG_FILE) $(CONFIG_BACKUP)
	@sed -i 's/Mode      = "debug"/Mode      = "$(MODE)"/' $(CONFIG_FILE)
	@sed -i 's/Version   = "v1.0.0"/Version   = "$(VERSION)"/' $(CONFIG_FILE)
	@sed -i 's|GitInfo   = ".*"|GitInfo   = "$(GIT_INFO)"|' $(CONFIG_FILE)
	@sed -i 's|BuildTime = ".*"|BuildTime = "$(BUILD_TIME)"|' $(CONFIG_FILE)

# 恢复配置文件
restore-config:
	@echo "Restoring $(CONFIG_FILE)..."
	@if [ -f $(CONFIG_BACKUP) ]; then \
		mv $(CONFIG_BACKUP) $(CONFIG_FILE); \
	fi

# Update generated build assets with the installed Wails CLI
update-build-assets:
	@echo "Updating build assets for $(APP_NAME)..."
	@cd build && wails3 update build-assets -name "$(APP_NAME)" -binaryname "$(APP_NAME)" -config config.yml -dir .

# Build for Windows
build-windows: replace-config
	@echo "Building for Windows $(VERSION)..."
	@wails3 build GOOS=windows VERSION=$(VERSION) MODE=$(MODE); \
		status=$$?; $(MAKE) restore-config; exit $$status

# Build for macOS ARM64 (Darwin)
build-mac: replace-config
	@echo "Building for macOS (Darwin) $(VERSION)..."
	@wails3 build GOOS=darwin ARCH=arm64 VERSION=$(VERSION) MODE=$(MODE); \
		status=$$?; $(MAKE) restore-config; exit $$status

# Build for macOS Intel (Darwin)
build-mac-intel: replace-config
	@echo "Building for macOS Intel (Darwin) $(VERSION)..."
	@wails3 build GOOS=darwin ARCH=amd64 VERSION=$(VERSION) MODE=$(MODE); \
		status=$$?; $(MAKE) restore-config; exit $$status

# Build for Linux
build-linux: replace-config
	@echo "Building for Linux $(VERSION)..."
	@wails3 build GOOS=linux VERSION=$(VERSION) MODE=$(MODE); \
		status=$$?; $(MAKE) restore-config; exit $$status

# Build linux/windows, create archives, publish draft GitHub release
pack-release:
	@$(MAKE) build-windows VERSION=$(VERSION) MODE=release
	@$(MAKE) build-linux VERSION=$(VERSION) MODE=release
	@echo "Packaging $(VERSION)..."
	@mkdir -p bin
	@test -f bin/$(APP_NAME) || (echo "missing bin/$(APP_NAME) (linux build failed or GOOS not applied)"; exit 1)
	@test -f bin/$(APP_NAME).exe || (echo "missing bin/$(APP_NAME).exe"; exit 1)
	@# Ensure linux artifact is not a Windows PE mistakenly named Vexo
	@if command -v file >/dev/null 2>&1; then \
		file bin/$(APP_NAME) | grep -qiE 'PE32|MS Windows' && \
			(echo "bin/$(APP_NAME) looks like a Windows binary; cross-build GOOS=linux failed"; exit 1) || true; \
	fi
	@tar -C bin -czf $(LINUX_ARCHIVE) $(APP_NAME)
	@rm -f $(WINDOWS_ARCHIVE)
	@if command -v zip >/dev/null 2>&1; then \
		cd bin && zip -q $(APP_NAME)_$(VERSION)_windows_amd64.zip $(APP_NAME).exe; \
	else \
		powershell -NoProfile -Command "Compress-Archive -Path 'bin/$(APP_NAME).exe' -DestinationPath '$(WINDOWS_ARCHIVE)' -Force"; \
	fi
	@test -f $(LINUX_ARCHIVE) || (echo "missing $(LINUX_ARCHIVE)"; exit 1)
	@test -f $(WINDOWS_ARCHIVE) || (echo "missing $(WINDOWS_ARCHIVE)"; exit 1)
	@echo "Created $(LINUX_ARCHIVE)"
	@echo "Created $(WINDOWS_ARCHIVE)"
	@echo "Generating $(SHA256SUMS_FILE)..."
	@rm -f $(SHA256SUMS_FILE)
	@if command -v sha256sum >/dev/null 2>&1; then \
		(cd bin && sha256sum $(APP_NAME)_$(VERSION)_linux_amd64.tar.gz $(APP_NAME)_$(VERSION)_windows_amd64.zip > SHA256SUMS); \
	elif command -v shasum >/dev/null 2>&1; then \
		(cd bin && shasum -a 256 $(APP_NAME)_$(VERSION)_linux_amd64.tar.gz $(APP_NAME)_$(VERSION)_windows_amd64.zip > SHA256SUMS); \
	else \
		powershell -NoProfile -Command \
			"$$out='$(SHA256SUMS_FILE)'; if (Test-Path $$out) { Remove-Item $$out }; \
			foreach ($$f in @('$(APP_NAME)_$(VERSION)_linux_amd64.tar.gz','$(APP_NAME)_$(VERSION)_windows_amd64.zip')) { \
				$$h=(Get-FileHash -Algorithm SHA256 -Path (Join-Path 'bin' $$f)).Hash.ToLower(); \
				Add-Content -Path $$out -Value ($$h + '  ' + $$f) -Encoding ascii \
			}"; \
	fi
	@test -f $(SHA256SUMS_FILE) || (echo "missing $(SHA256SUMS_FILE)"; exit 1)
	@echo "Created $(SHA256SUMS_FILE)"
	@command -v gh >/dev/null 2>&1 || (echo "gh CLI is required for pack-release"; exit 1)
	@if gh release view $(VERSION) >/dev/null 2>&1; then \
		echo "Uploading assets to existing release $(VERSION)..."; \
		gh release upload $(VERSION) $(LINUX_ARCHIVE) $(WINDOWS_ARCHIVE) $(SHA256SUMS_FILE) --clobber; \
	else \
		echo "Creating draft release $(VERSION)..."; \
		gh release create $(VERSION) \
			--draft \
			--title "$(VERSION)" \
			--generate-notes \
			$(LINUX_ARCHIVE) \
			$(WINDOWS_ARCHIVE) \
			$(SHA256SUMS_FILE); \
	fi
	@echo "Draft release $(VERSION) ready."

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@if [ -f $(CONFIG_BACKUP) ]; then \
		rm $(CONFIG_BACKUP); \
	fi
