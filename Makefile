#!make
.DEFAULT_GOAL := clean install build

# --- OS Environment Setup --- #
ifeq ($(OS),Windows_NT)
	export PATH:=$(PATH);$(GOPATH)\bin;C:\Program Files\Go\bin;C:\Windows\System32;C:\Program Files\nodejs;C:\WINDOWS\System32\WindowsPowerShell\v1.0;
	SHELL := powershell.exe
	.SHELLFLAGS := -NoProfile -Command
	VERSION := $(shell powershell -ExecutionPolicy Bypass -File resources\windows\get_version.ps1)
	DETECTED_OS=Windows_NT
	HELPER=src\core\host\bin\helper\win\YourPlaceHelper.exe
	PACKAGER=resources\windows\windows_packager.ps1
	NPM=npm.cmd
	NPX=npx.cmd
	export GOTMPDIR=C:\Users\$(USERNAME)\AppData\Local\Temp\go-yourplace-build
else
	DETECTED_OS=$(shell uname -s)
	GO=$(shell which go)
	NODE=$(shell which node)
	NPM=$(shell which npm)
	NPX="$(shell which npx)"
endif

# --- Update Commands --- #
npm_update:
	npm install -g npm-check-updates
	ncu -u
	npm install
go_update:
	go get -u ./...

# --- Setup Commands --- #
clean:
ifeq ($(DETECTED_OS),Windows_NT)
	-powershell -Command "Get-Process -Name 'YourPlace' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue | Out-Null"
	-powershell -Command "Get-Process -Name 'YourPlace-$(VERSION)' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue | Out-Null"
	-powershell -Command "Get-Process -Name 'YourPlaceHelper' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue | Out-Null"
	-powershell -Command "Get-Process -Name 'YourPlaceIpfs' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue | Out-Null"
	-powershell -Command "Get-Process -Name 'YourPlaceFfmpeg' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue | Out-Null"
	-powershell -Command "if (Test-Path 'target') { Remove-Item -Recurse -Force 'target' | Out-Null }"
	-powershell -Command "if (Test-Path '$(HELPER)') { Remove-Item -Force '$(HELPER)' | Out-Null }"
	-powershell -Command "if (Test-Path 'src\core\host\bin\helper\win\helper.version') { Remove-Item -Force 'src\core\host\bin\helper\win\helper.version' | Out-Null }"
	-powershell -Command "if (Test-Path 'rsrc_windows_amd64.syso') { Remove-Item -Force 'rsrc_windows_amd64.syso' | Out-Null }"
	go clean
else ifeq ($(DETECTED_OS),Darwin)
	-pkill -f YourPlace 2>/dev/null || true
	-pkill -f YourPlaceHelper 2>/dev/null || true
	-pkill -f YourPlaceIpfs 2>/dev/null || true
	-pkill -f YourPlaceFfmpeg 2>/dev/null || true
	rm -rf target 2>/dev/null || true
	rm -rf src/core/host/bin/helper/osx/* 2>/dev/null || true
	go clean
else
	-pkill -f YourPlace 2>/dev/null || true
	rm -rf target 2>/dev/null || true
	go clean
endif
install:
	$(NPM) install
test:
	$(which golangci-lint) run --enable-all
	go test

# --- Build Commands --- #
build: clean
ifeq ($(DETECTED_OS),Windows_NT)
	powershell -Command "if (-not (Test-Path 'target')) { New-Item -ItemType Directory -Path 'target' }"
	powershell -Command "if (-not (Test-Path 'src\core\host\bin\helper\win')) { New-Item -ItemType Directory -Path 'src\core\host\bin\helper\win' -Force }"
	powershell -Command "if (-not (Test-Path '$(GOTMPDIR)')) { New-Item -ItemType Directory -Path '$(GOTMPDIR)' -Force }"
	powershell -Command "New-Item -ItemType File -Path 'src\core\host\bin\helper\win\YourPlaceHelper.exe' -Force"
	powershell -ExecutionPolicy Bypass -File resources\windows\helper_build_setup.ps1
	go build -ldflags "-H=windowsgui -s -w" -o target\YourPlaceHelper.exe helper\helper_win.go
	powershell -Command "Copy-Item -Path 'target\YourPlaceHelper.exe' -Destination 'src\core\host\bin\helper\win\YourPlaceHelper.exe' -Force"
	$(NPX) webpack --config "src\typescript\webpack.prod.js"
	go build -ldflags "-H=windowsgui -s -w" -o target\YourPlace.exe main.go
	resources\windows\upx.exe -f -o target\YourPlace-$(VERSION).exe target\YourPlace.exe
else ifeq ($(DETECTED_OS),Darwin)
	$(NPX) webpack --config src/typescript/webpack.prod.js
	mkdir -p src/core/host/bin/helper/osx/
	go generate
	touch src/core/host/bin/helper/osx/YourPlaceHelper
	VERSION=$(shell grep 'version.*=.*"' helper/helper_osx.go | cut -d'"' -f2)
	@echo $(VERSION) > src/core/host/bin/helper/osx/helper.version
	go build -o target/YourPlaceHelper helper/helper_osx.go
	cp -rf target/YourPlaceHelper src/core/host/bin/helper/osx/YourPlaceHelper
	go build -o target/YourPlace main.go
	chmod +x resources/osx/askpass.sh
	chmod +x resources/osx/osx_packager.sh
	./resources/osx/osx_packager.sh
else
	mkdir -p target
	$(NPX) webpack --config src/typescript/webpack.prod.js
	@VERSION=$$(grep -oE '^\s*version\s*=\s*"[^"]*"' main.go | grep -oE '"[^"]*"' | tr -d '"') && \
	go build -ldflags "-s -w" -o target/YourPlace-$$VERSION main.go
endif
dbg_build: clean
ifeq ($(DETECTED_OS),Windows_NT)
	powershell -Command "if (-not (Test-Path 'target')) { New-Item -ItemType Directory -Path 'target' }"
	powershell -Command "if (-not (Test-Path 'src\core\host\bin\helper\win')) { New-Item -ItemType Directory -Path 'src\core\host\bin\helper\win' -Force }"
	powershell -Command "if (-not (Test-Path '$(GOTMPDIR)')) { New-Item -ItemType Directory -Path '$(GOTMPDIR)' -Force }"
	powershell -Command "New-Item -ItemType File -Path 'src\core\host\bin\helper\win\YourPlaceHelper.exe' -Force"
	powershell -ExecutionPolicy Bypass -File resources\windows\helper_build_setup.ps1
	go build -ldflags "-s -w" -o target\YourPlaceHelper.exe helper\helper_win.go
	powershell -Command "Copy-Item -Path 'target\YourPlaceHelper.exe' -Destination 'src\core\host\bin\helper\win\YourPlaceHelper.exe' -Force"
	$(NPX) webpack --config "src\typescript\webpack.prod.js"
	go build -ldflags "-s -w" -o target\YourPlace-$(VERSION).exe main.go
else ifeq ($(DETECTED_OS),Darwin)
	$(NPX) webpack --config src/typescript/webpack.prod.js
	mkdir -p src/core/host/bin/helper/osx/
	go generate
	touch src/core/host/bin/helper/osx/YourPlaceHelper
	VERSION=$(shell grep 'version.*=.*"' helper/helper_osx.go | cut -d'"' -f2)
	@echo $(VERSION) > src/core/host/bin/helper/osx/helper.version
	go build -o target/YourPlaceHelper helper/helper_osx.go
	cp -rf target/YourPlaceHelper src/core/host/bin/helper/osx/YourPlaceHelper
	go build -o target/YourPlace main.go
	chmod +x resources/osx/askpass.sh
	chmod +x resources/osx/osx_packager.sh
	./resources/osx/osx_packager.sh dev
endif
helper_build:
ifeq ($(DETECTED_OS),Windows_NT)
	$(PS) -Command "if (!(Test-Path '$(HELPER)')) { New-Item -ItemType File -Path '$(HELPER)' -Force | Out-Null }"
	$(GO) generate
	type nul > "src\core\host\bin\helper\win\YourPlaceHelper.exe"
	$(GO) build -ldflags '-s -w' -o target\YourPlaceHelper.exe helper\helper_win.go
	copy /B /Y /V target\YourPlaceHelper.exe src\core\host\bin\helper\win\YourPlaceHelper.exe
else ifeq ($(DETECTED_OS),Darwin)
	cp -rf helper/helper_osx.version src/core/host/bin/helper/osx/helper_osx.version
	go generate
	go build -o target/YourPlaceHelper helper/helper_osx.go
	cp -rf target/YourPlaceHelper src/core/host/bin/helper/osx/YourPlaceHelper
endif

# --- Run Commands --- #
run:
ifeq ($(DETECTED_OS),Windows_NT)
	target\\YourPlace-$(VERSION).exe
else ifeq ($(DETECTED_OS),Darwin)
	rm -rf ~/YourPlace/debug # Ensure debug file is removed
	@VERSION=$$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/') && \
	./target/YourPlace-$(VERSION)
endif
dbg_run:
ifeq ($(DETECTED_OS),Windows_NT)
	target\\YourPlace-$(VERSION).exe -d -du
else ifeq ($(DETECTED_OS),Darwin)
	mkdir -p ~/YourPlace && touch ~/YourPlace/debug # Add debug file flag
	@VERSION=$$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/') && \
	SUDO_ASKPASS=resources/osx/askpass.sh sudo -A installer -pkg "target/YourPlace-$$VERSION.pkg" -target /
endif
dbg_gateway_run:
ifeq ($(DETECTED_OS),Windows_NT)
	target\\YourPlace-$(VERSION).exe -d -du -g
else ifeq ($(DETECTED_OS),Darwin)
	mkdir -p ~/YourPlace && touch ~/YourPlace/debug # Add debug file flag
    @VERSION=$$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/') && \
    SUDO_ASKPASS=resources/osx/askpass.sh sudo -A installer -pkg "target/YourPlace-$$VERSION.pkg" -target /
endif
dbg_noindexer_run:
ifeq ($(DETECTED_OS),Windows_NT)
	target\\YourPlace-$(VERSION).exe -d -du -di
else ifeq ($(DETECTED_OS),Darwin)
	mkdir -p ~/YourPlace && touch ~/YourPlace/debug && touch ~/YourPlace/noindexer # Add debug file flag
	@VERSION=$$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/') && \
	SUDO_ASKPASS=resources/osx/askpass.sh installer -pkg "target/YourPlace-$$VERSION.pkg" -target /
endif
dbg_noinstall_run:
ifeq ($(DETECTED_OS),Windows_NT)
	target\\YourPlace-$(VERSION).exe -d -du
else ifeq ($(DETECTED_OS),Darwin)
	@VERSION=$$(grep 'version.*=.*".*"' main.go | sed -E 's/.*version.*=.*"(.*)".*/\1/') && \
	./target/YourPlace -d -du
endif
testing:
	go run test.go

# --- Snapshot Service Commands --- #
snapshot_docker_build:
	docker build --progress=plain --no-cache -f snapshot/Dockerfile -t yourplace-snapshot .
snapshot_build:
	mkdir -p target/
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o target/YourPlaceSnapshot snapshot/snapshot.go
snapshot_run:
	./target/YourPlaceSnapshot
snapshot_dbg_build:
	mkdir -p target/
	mkdir -p src/core/host/bin/helper/osx/
	@if [ ! -f src/core/host/bin/helper/osx/helper.version ]; then echo "1.0.0" > src/core/host/bin/helper/osx/helper.version; fi
	go build -o target/YourPlaceSnapshot snapshot/snapshot.go
snapshot_dbg_run:
	./target/YourPlaceSnapshot -d
