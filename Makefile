SHELL := /bin/sh

GO ?= go
BINARY ?= envdoctor
BIN_DIR ?= bin
DIST_DIR ?= dist
CACHE_DIR ?= .cache
GOCACHE ?= $(CURDIR)/$(CACHE_DIR)/go-build
GOMODCACHE ?= $(CURDIR)/$(CACHE_DIR)/go-mod
PKG ?= ./cmd/envdoctor
PACKAGES ?= ./...
ARGS ?= help
PROD_LDFLAGS ?= -s -w
RELEASE_TARGETS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
SMOKE_DIR ?= $(CURDIR)/$(CACHE_DIR)/smoke

GO_FILES := $(shell find cmd internal -type f -name '*.go' 2>/dev/null)
HOST_GOOS := $(shell $(GO) env GOOS)
EXE := $(if $(filter windows,$(HOST_GOOS)),.exe,)
LOCAL_BIN := $(BIN_DIR)/$(BINARY)$(EXE)

.PHONY: help fmt fmt-check vet test check build dev prod release smoke clean

help:
	@printf '%s\n' \
		'Environment Doctor Agent targets:' \
		'  make fmt                         Format Go source files' \
		'  make fmt-check                   Fail if Go source files need formatting' \
		'  make vet                         Run go vet ./...' \
		'  make test                        Compile packages with go test ./...' \
		'  make check                       Run fmt-check, test, and vet' \
		'  make build                       Build local binary into bin/' \
		'  make dev ARGS="diagnose --json"  Run CLI with go run' \
		'  make prod                        Build optimized local production binary' \
		'  make release                     Cross-compile Linux/macOS/Windows binaries' \
		'  make smoke                       Run non-mutating Phase 1-3 smoke checks' \
		'  make clean                       Remove bin/, dist/, and .cache/'

fmt:
	@gofmt -w $(GO_FILES)

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then \
		printf 'Go files need formatting:\n%s\n' "$$files"; \
		exit 1; \
	fi

vet:
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) vet $(PACKAGES)

test:
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) test $(PACKAGES)

check: fmt-check test vet

build:
	@mkdir -p "$(BIN_DIR)"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) build -o "$(LOCAL_BIN)" $(PKG)
	@printf 'Built %s\n' "$(LOCAL_BIN)"

dev:
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) $(ARGS)

prod:
	@mkdir -p "$(BIN_DIR)"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) build -trimpath -ldflags="$(PROD_LDFLAGS)" -o "$(LOCAL_BIN)" $(PKG)
	@printf 'Built production binary %s\n' "$(LOCAL_BIN)"

release:
	@mkdir -p "$(DIST_DIR)"
	@set -e; \
	for target in $(RELEASE_TARGETS); do \
		os="$${target%/*}"; \
		arch="$${target#*/}"; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="$(DIST_DIR)/$(BINARY)-$$os-$$arch$$ext"; \
		printf 'Building %s\n' "$$out"; \
		GOOS="$$os" GOARCH="$$arch" CGO_ENABLED=0 GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" \
			$(GO) build -trimpath -ldflags="$(PROD_LDFLAGS)" -o "$$out" $(PKG); \
	done

smoke:
	@mkdir -p "$(SMOKE_DIR)/node" "$(SMOKE_DIR)/go"
	@printf '%s\n' \
		'ERROR: Cannot connect to Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' \
		'npm ERR! code ERESOLVE' \
		"ModuleNotFoundError: No module named 'requests'" \
		'ssh: connect to host example.com port 22: Connection refused' \
		> "$(SMOKE_DIR)/envdoctor.log"
	@printf '%s\n' '{"dependencies":{"express":"^5.0.0"},"scripts":{"start":"node index.js"}}' > "$(SMOKE_DIR)/node/package.json"
	@printf '%s\n' 'module example.com/envdoctor-smoke' '' 'go 1.22' > "$(SMOKE_DIR)/go/go.mod"
	@printf '%s\n' 'package main' 'import ("encoding/json"; "os")' 'func main() { for _, p := range os.Args[1:] { b, err := os.ReadFile(p); if err != nil { panic(err) }; var v any; if err := json.Unmarshal(b, &v); err != nil { panic(err) } } }' > "$(SMOKE_DIR)/jsoncheck.go"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) system >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan toolchain >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan path >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan container >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) snapshot >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/node" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/go" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) diagnose --json > "$(SMOKE_DIR)/diagnose.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) explain --json "$(SMOKE_DIR)/envdoctor.log" > "$(SMOKE_DIR)/explain.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) recommend --json > "$(SMOKE_DIR)/recommend.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/jsoncheck.go" "$(SMOKE_DIR)/diagnose.json" "$(SMOKE_DIR)/explain.json" "$(SMOKE_DIR)/recommend.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) dockerize "$(SMOKE_DIR)/node" > "$(SMOKE_DIR)/Dockerfile.preview"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) dockerize --force --output "$(SMOKE_DIR)/Dockerfile.generated" "$(SMOKE_DIR)/go" >/dev/null
	@printf 'Smoke checks passed\n'

clean:
	@if [ "$(BIN_DIR)" = "." ] || [ "$(DIST_DIR)" = "." ] || [ "$(CACHE_DIR)" = "." ]; then \
		printf 'Refusing to clean with unsafe directory settings.\n'; \
		exit 1; \
	fi
	@rm -rf -- "$(BIN_DIR)" "$(DIST_DIR)" "$(CACHE_DIR)"
	@printf 'Removed %s, %s, and %s\n' "$(BIN_DIR)" "$(DIST_DIR)" "$(CACHE_DIR)"
