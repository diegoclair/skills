# confluence-docs Makefile
#
# Common targets:
#   make build       Build native binary into bin/confluence-docs
#   make build-all   Cross-compile for darwin/linux/windows × amd64/arm64
#   make test        Run unit tests
#   make install     Build and install to the Claude skill directory.
#                    Default: $HOME/.claude/skills/confluence-docs/bin
#                    Override with: make install INSTALL_DIR=/custom/path
#   make clean       Remove bin/ and dist/
#   make fmt         go fmt ./...
#   make lint        go vet ./...

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PKG     := .
BIN     := confluence-docs

# Where the Claude skill loader expects to find the binary.
# Override at invocation time if your Claude config lives elsewhere.
INSTALL_DIR ?= $(HOME)/.claude/skills/confluence-docs/bin

.PHONY: all build build-all test install clean fmt lint deps release

all: build

deps:
	go mod tidy

build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BIN) $(PKG)

build-all: clean-dist
	@mkdir -p dist
	GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BIN)-darwin-amd64    $(PKG)
	GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BIN)-darwin-arm64    $(PKG)
	GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BIN)-linux-amd64     $(PKG)
	GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BIN)-linux-arm64     $(PKG)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BIN)-windows-amd64.exe $(PKG)
	@echo "built dist/ artifacts for version $(VERSION)"

test:
	go test -v ./...

install: build
	@mkdir -p $(INSTALL_DIR)
	cp bin/$(BIN) $(INSTALL_DIR)/$(BIN)
	@echo "installed to $(INSTALL_DIR)/$(BIN)"

clean: clean-dist
	rm -rf bin

clean-dist:
	rm -rf dist

fmt:
	go fmt ./...

lint:
	go vet ./...

# release: build cross-platform binaries, package them into archives, and
# generate a SHASUMS256.txt.  Requires VERSION to be set explicitly:
#   make release VERSION=v0.3.0
#
# Outputs (all inside dist/):
#   confluence-docs-darwin-amd64.tar.gz
#   confluence-docs-darwin-arm64.tar.gz
#   confluence-docs-linux-amd64.tar.gz
#   confluence-docs-linux-arm64.tar.gz
#   confluence-docs-windows-amd64.zip
#   SHASUMS256.txt
release: build-all
ifndef VERSION
	$(error VERSION is not set — run: make release VERSION=v0.3.0)
endif
	@echo "==> Packaging release $(VERSION)"
	@# -- tarballs for Unix platforms --
	@for platform in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do \
		binary="dist/$(BIN)-$$platform"; \
		archive="dist/$(BIN)-$$platform.tar.gz"; \
		extra=""; \
		if [ -f README.md ];  then extra="$$extra README.md";  fi; \
		if [ -f SETUP.md ];   then extra="$$extra SETUP.md";   fi; \
		if [ -f LICENSE ];    then extra="$$extra LICENSE";    fi; \
		echo "  -> $$archive"; \
		tar -czf "$$archive" -C dist "$(BIN)-$$platform" \
			$$(for f in $$extra; do echo "-C $(CURDIR) $$f"; done) 2>/dev/null || \
		tar -czf "$$archive" "$$binary" $$extra; \
	done
	@# -- zip for Windows --
	@binary="dist/$(BIN)-windows-amd64.exe"; \
	archive="dist/$(BIN)-windows-amd64.zip"; \
	extra=""; \
	if [ -f README.md ]; then extra="$$extra README.md"; fi; \
	if [ -f SETUP.md ];  then extra="$$extra SETUP.md";  fi; \
	if [ -f LICENSE ];   then extra="$$extra LICENSE";   fi; \
	echo "  -> $$archive"; \
	zip -j "$$archive" "$$binary" $$extra
	@# -- SHASUMS256.txt --
	@echo "  -> dist/SHASUMS256.txt"
	@cd dist && sha256sum \
		$(BIN)-darwin-amd64.tar.gz \
		$(BIN)-darwin-arm64.tar.gz \
		$(BIN)-linux-amd64.tar.gz \
		$(BIN)-linux-arm64.tar.gz \
		$(BIN)-windows-amd64.zip \
		> SHASUMS256.txt
	@echo ""
	@echo "Release $(VERSION) ready in dist/. To publish:"
	@echo "  gh release create $(VERSION) dist/*.tar.gz dist/*.zip dist/SHASUMS256.txt \\"
	@echo "    --title '$(VERSION)' --notes 'See CHANGELOG'"
