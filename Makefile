SHELL := /bin/sh

GO ?= go
BINARY ?= envdoctor
BIN_DIR ?= bin
DIST_DIR ?= dist
CACHE_DIR ?= .cache
GOCACHE ?= $(CURDIR)/$(CACHE_DIR)/go-build
GOMODCACHE ?= $(CURDIR)/$(CACHE_DIR)/go-mod
TOOLS_DIR ?= $(CURDIR)/$(CACHE_DIR)/tools
PKG ?= ./cmd/envdoctor
PACKAGES ?= ./...
ARGS ?= help
GIT_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+' || true)
VERSION ?= $(if $(GIT_TAG),$(GIT_TAG),0.0.0-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
RELEASE_REPOSITORY ?= stepanusjanu19/envdoctoragent
GORELEASER_VERSION ?= v2.16.0
GORELEASER ?= $(TOOLS_DIR)/goreleaser
BUILD_LDFLAGS ?= -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
PROD_LDFLAGS ?= -s -w $(BUILD_LDFLAGS)
RELEASE_TARGETS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
SMOKE_DIR ?= $(CURDIR)/$(CACHE_DIR)/smoke
DEPENDENCY_SMOKE_DIR ?= $(SMOKE_DIR)/dependencies

GO_FILES := $(shell find cmd internal -type f -name '*.go' 2>/dev/null)
HOST_GOOS := $(shell $(GO) env GOOS)
EXE := $(if $(filter windows,$(HOST_GOOS)),.exe,)
LOCAL_BIN := $(BIN_DIR)/$(BINARY)$(EXE)

.PHONY: help fmt fmt-check vet test check build dev prod release-binaries release-check release release-publish tools-goreleaser smoke clean

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
		'  make release-binaries            Cross-compile raw Linux/macOS/Windows binaries' \
		'  make release-check               Validate GoReleaser configuration' \
		'  make release                     Build packaged snapshot artifacts into dist/' \
		'  make smoke                       Run non-mutating smoke checks' \
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
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) build -ldflags="$(BUILD_LDFLAGS)" -o "$(LOCAL_BIN)" $(PKG)
	@printf 'Built %s\n' "$(LOCAL_BIN)"

dev:
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) $(ARGS)

prod:
	@mkdir -p "$(BIN_DIR)"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) build -trimpath -ldflags="$(PROD_LDFLAGS)" -o "$(LOCAL_BIN)" $(PKG)
	@printf 'Built production binary %s\n' "$(LOCAL_BIN)"

release-binaries:
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

tools-goreleaser:
	@if ! command -v "$(GORELEASER)" >/dev/null 2>&1; then \
		if [ "$(GORELEASER)" != "$(TOOLS_DIR)/goreleaser" ]; then \
			printf 'GoReleaser not found: %s\n' "$(GORELEASER)"; \
			exit 127; \
		fi; \
		printf 'Installing GoReleaser %s into %s\n' "$(GORELEASER_VERSION)" "$(TOOLS_DIR)"; \
		mkdir -p "$(TOOLS_DIR)"; \
		GOBIN="$(TOOLS_DIR)" GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" \
			$(GO) install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION); \
	fi

release-check: tools-goreleaser
	@$(GORELEASER) check

release: tools-goreleaser
	@VERSION="$(VERSION)" GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GORELEASER) release --snapshot --clean
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run ./cmd/releasemanifests \
		--dist "$(DIST_DIR)" \
		--version "$(VERSION)" \
		--repository "$(RELEASE_REPOSITORY)"
	@printf 'Release artifacts written to %s\n' "$(DIST_DIR)"

release-publish: tools-goreleaser
	@VERSION="$(VERSION)" GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GORELEASER) release --clean

smoke:
	@rm -rf -- "$(SMOKE_DIR)/project-empty" "$(SMOKE_DIR)/project-node" "$(SMOKE_DIR)/project-yes" "$(SMOKE_DIR)/scaffold-react" "$(SMOKE_DIR)/scaffold-go" "$(SMOKE_DIR)/scaffold-python" "$(SMOKE_DIR)/scaffold-conflict" "$(SMOKE_DIR)/agent-scaffold-plan" "$(SMOKE_DIR)/agent-scaffold-yes" "$(SMOKE_DIR)/agent-prod-block" "$(SMOKE_DIR)/automation-scaffold-plan" "$(SMOKE_DIR)/automation-prod-block"
	@mkdir -p "$(SMOKE_DIR)/node" "$(SMOKE_DIR)/go" "$(SMOKE_DIR)/version" "$(SMOKE_DIR)/bootstrap" "$(SMOKE_DIR)/project-empty" "$(SMOKE_DIR)/project-node" "$(SMOKE_DIR)/project-yes"
	@mkdir -p "$(SMOKE_DIR)/scaffold-react" "$(SMOKE_DIR)/scaffold-go" "$(SMOKE_DIR)/scaffold-conflict"
	@mkdir -p "$(DEPENDENCY_SMOKE_DIR)/python" "$(DEPENDENCY_SMOKE_DIR)/node" "$(DEPENDENCY_SMOKE_DIR)/go" "$(DEPENDENCY_SMOKE_DIR)/rust" "$(DEPENDENCY_SMOKE_DIR)/php"
	@mkdir -p "$(DEPENDENCY_SMOKE_DIR)/maven" "$(DEPENDENCY_SMOKE_DIR)/gradle" "$(DEPENDENCY_SMOKE_DIR)/dotnet" "$(DEPENDENCY_SMOKE_DIR)/nuget-config" "$(DEPENDENCY_SMOKE_DIR)/nuget-props"
	@mkdir -p "$(DEPENDENCY_SMOKE_DIR)/ruby" "$(DEPENDENCY_SMOKE_DIR)/dart" "$(DEPENDENCY_SMOKE_DIR)/swift" "$(DEPENDENCY_SMOKE_DIR)/elixir" "$(DEPENDENCY_SMOKE_DIR)/lua"
	@mkdir -p "$(DEPENDENCY_SMOKE_DIR)/r" "$(DEPENDENCY_SMOKE_DIR)/julia" "$(DEPENDENCY_SMOKE_DIR)/cabal" "$(DEPENDENCY_SMOKE_DIR)/stack" "$(DEPENDENCY_SMOKE_DIR)/perl"
	@mkdir -p "$(DEPENDENCY_SMOKE_DIR)/vcpkg" "$(DEPENDENCY_SMOKE_DIR)/conan-txt" "$(DEPENDENCY_SMOKE_DIR)/conan-py"
	@printf '%s\n' \
		'ERROR: Cannot connect to Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' \
		'npm ERR! code ERESOLVE' \
		"ModuleNotFoundError: No module named 'requests'" \
		'ssh: connect to host example.com port 22: Connection refused' \
		> "$(SMOKE_DIR)/envdoctor.log"
	@printf '%s\n' '{"dependencies":{"express":"^5.0.0"},"scripts":{"start":"node index.js"}}' > "$(SMOKE_DIR)/node/package.json"
	@printf '%s\n' 'module example.com/envdoctor-smoke' '' 'go 1.22' > "$(SMOKE_DIR)/go/go.mod"
	@printf '%s\n' '18.19.0' > "$(SMOKE_DIR)/version/.nvmrc"
	@printf '%s\n' '3.12.0' > "$(SMOKE_DIR)/version/.python-version"
	@printf '%s\n' '[toolchain]' 'channel = "stable"' > "$(SMOKE_DIR)/version/rust-toolchain.toml"
	@printf '%s\n' 'module example.com/envdoctor-version' '' 'go 1.22' > "$(SMOKE_DIR)/version/go.mod"
	@printf '%s\n' '{"engines":{"node":">=18"}}' > "$(SMOKE_DIR)/version/package.json"
	@printf '%s\n' '[project]' 'requires-python = ">=3.11"' > "$(SMOKE_DIR)/version/pyproject.toml"
	@printf '%s\n' '{"dependencies":{"express":"^5.0.0"}}' > "$(SMOKE_DIR)/bootstrap/package.json"
	@printf '%s\n' '{"dependencies":{"express":"^5.0.0"}}' > "$(SMOKE_DIR)/project-node/package.json"
	@printf '%s\n' 'FROM node:22-alpine' > "$(SMOKE_DIR)/bootstrap/Dockerfile"
	@printf '%s\n' 'requests>=2.31' '-r common.txt' '--extra-index-url https://example.invalid/simple' > "$(DEPENDENCY_SMOKE_DIR)/python/requirements.txt"
	@printf '%s\n' '[project]' 'dependencies = ["requests>=2.31"]' '[project.optional-dependencies]' 'dev = ["pytest>=8"]' > "$(DEPENDENCY_SMOKE_DIR)/python/pyproject.toml"
	@printf '%s\n' '{"packageManager":"pnpm@9.0.0","dependencies":{"react":"^18.2.0","next":"^14.0.0","express":"^5.0.0"},"devDependencies":{"vite":"^5.0.0"},"peerDependencies":{"vue":"^3.0.0"},"optionalDependencies":{"svelte":"^4.0.0"}}' > "$(DEPENDENCY_SMOKE_DIR)/node/package.json"
	@printf '%s\n' 'module example.com/dependencies-smoke' '' 'go 1.22' > "$(DEPENDENCY_SMOKE_DIR)/go/go.mod"
	@printf '%s\n' '[package]' 'name = "dependencies_smoke"' 'version = "0.1.0"' 'edition = "2021"' '[dependencies]' > "$(DEPENDENCY_SMOKE_DIR)/rust/Cargo.toml"
	@printf '%s\n' '{"require":{"monolog/monolog":"^3.0"},"require-dev":{"phpunit/phpunit":"^10.0"}}' > "$(DEPENDENCY_SMOKE_DIR)/php/composer.json"
	@printf '%s\n' '<project><dependencies><dependency><groupId>org.slf4j</groupId><artifactId>slf4j-api</artifactId><version>2.0.13</version></dependency></dependencies></project>' > "$(DEPENDENCY_SMOKE_DIR)/maven/pom.xml"
	@printf '%s\n' 'plugins { id "java" }' 'dependencies {' '  implementation "com.google.guava:guava:33.0.0-jre"' '  testImplementation "junit:junit:4.13.2"' '}' > "$(DEPENDENCY_SMOKE_DIR)/gradle/build.gradle"
	@printf '%s\n' '<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="Newtonsoft.Json" Version="13.0.3" /></ItemGroup></Project>' > "$(DEPENDENCY_SMOKE_DIR)/dotnet/App.csproj"
	@printf '%s\n' '<packages><package id="NUnit" version="3.14.0" /></packages>' > "$(DEPENDENCY_SMOKE_DIR)/nuget-config/packages.config"
	@printf '%s\n' '<Project><ItemGroup><PackageVersion Include="Serilog" Version="3.1.1" /></ItemGroup></Project>' > "$(DEPENDENCY_SMOKE_DIR)/nuget-props/Directory.Packages.props"
	@printf '%s\n' "gem 'rails', '~> 7.1'" "gem 'rspec', group: :test" > "$(DEPENDENCY_SMOKE_DIR)/ruby/Gemfile"
	@printf '%s\n' 'name: dependencies_smoke' 'dependencies:' '  http: ^1.2.0' 'dev_dependencies:' '  test: ^1.25.0' 'flutter:' '  uses-material-design: true' > "$(DEPENDENCY_SMOKE_DIR)/dart/pubspec.yaml"
	@printf '%s\n' 'let package = Package(' '  dependencies: [.package(url: "https://github.com/apple/swift-nio.git", from: "2.0.0")]' ')' > "$(DEPENDENCY_SMOKE_DIR)/swift/Package.swift"
	@printf '%s\n' 'defp deps do' '  [' '    {:plug, "~> 1.0"}' '  ]' 'end' > "$(DEPENDENCY_SMOKE_DIR)/elixir/mix.exs"
	@printf '%s\n' 'package = "dependencies_smoke"' 'version = "1.0-1"' 'dependencies = {' '  "lua >= 5.4",' '  "luasocket >= 3.0"' '}' > "$(DEPENDENCY_SMOKE_DIR)/lua/dependencies_smoke-1.0-1.rockspec"
	@printf '%s\n' 'Package: dependenciesSmoke' 'Imports: jsonlite (>= 1.8), httr' 'Suggests: testthat' > "$(DEPENDENCY_SMOKE_DIR)/r/DESCRIPTION"
	@printf '%s\n' '[deps]' 'JSON = "682c06a0-de6a-54ab-a142-c8b1cf79cde6"' '[compat]' 'JSON = "0.21"' > "$(DEPENDENCY_SMOKE_DIR)/julia/Project.toml"
	@printf '%s\n' 'name: dependencies-smoke' 'version: 0.1.0.0' 'build-depends: base >=4.14, text' > "$(DEPENDENCY_SMOKE_DIR)/cabal/dependencies-smoke.cabal"
	@printf '%s\n' 'resolver: lts-22.0' 'extra-deps:' '  - text-2.0.2' > "$(DEPENDENCY_SMOKE_DIR)/stack/stack.yaml"
	@printf '%s\n' "requires 'Mojolicious', '9.0';" "on 'test' => sub { requires 'Test::More', '1.0'; };" > "$(DEPENDENCY_SMOKE_DIR)/perl/cpanfile"
	@printf '%s\n' '{"name":"dependencies-smoke","version-string":"0.1.0","dependencies":["fmt",{"name":"zlib","version>=":"1.2.13"}]}' > "$(DEPENDENCY_SMOKE_DIR)/vcpkg/vcpkg.json"
	@printf '%s\n' '[requires]' 'zlib/1.2.13' > "$(DEPENDENCY_SMOKE_DIR)/conan-txt/conanfile.txt"
	@printf '%s\n' 'requires = "zlib/1.2.13"' > "$(DEPENDENCY_SMOKE_DIR)/conan-py/conanfile.py"
	@printf '%s\n' 'package main' 'import ("encoding/json"; "os")' 'func main() { for _, p := range os.Args[1:] { b, err := os.ReadFile(p); if err != nil { panic(err) }; var v any; if err := json.Unmarshal(b, &v); err != nil { panic(err) } } }' > "$(SMOKE_DIR)/jsoncheck.go"
	@printf '%s\n' 'package main' 'import ("encoding/json"; "fmt"; "os"; "path/filepath")' 'type plan struct { Directory string `json:"directory"`; ExecutionBaseDir string `json:"execution_base_dir"`; Template string `json:"template"`; Source string `json:"source"`; Actions []action `json:"actions"` }' 'type action struct { Type string `json:"type"`; Command string `json:"command"`; Args []string `json:"args"`; WorkingDir string `json:"working_dir"` }' 'func main() { if len(os.Args) != 3 { panic("usage: scaffoldcheck <path|target> <plan.json>") }; mode, path := os.Args[1], os.Args[2]; b, err := os.ReadFile(path); if err != nil { panic(err) }; var p plan; if err := json.Unmarshal(b, &p); err != nil { panic(err) }; if p.Source != "official" { panic(fmt.Sprintf("%s source is %s", p.Template, p.Source)) }; if p.ExecutionBaseDir == "" { p.ExecutionBaseDir = p.Directory }; commandCount, hasMkdir := 0, false; for _, a := range p.Actions { if a.Type == "write_file" || a.Type == "manual" { panic(fmt.Sprintf("%s exposed %s action", p.Template, a.Type)) }; if a.Type == "mkdir" { hasMkdir = true }; if a.Type != "command" { continue }; commandCount++; if a.WorkingDir == "" { panic(fmt.Sprintf("%s command has empty working_dir", p.Template)) }; if mode == "path" && filepath.Clean(a.WorkingDir) != filepath.Clean(filepath.Dir(p.Directory)) { panic(fmt.Sprintf("%s command working_dir = %s, want parent", p.Template, a.WorkingDir)) }; if mode == "target" && filepath.Clean(a.WorkingDir) != filepath.Clean(p.Directory) { panic(fmt.Sprintf("%s command working_dir = %s, want target", p.Template, a.WorkingDir)) }; for _, arg := range a.Args { if mode == "path" && arg == "." { panic(fmt.Sprintf("%s path-arg generator used . destination", p.Template)) } } }; if commandCount == 0 { panic(fmt.Sprintf("%s has no command action", p.Template)) }; if mode == "path" { if hasMkdir { panic(fmt.Sprintf("%s path-arg generator should not pre-create target", p.Template)) }; if filepath.Clean(p.ExecutionBaseDir) != filepath.Clean(filepath.Dir(p.Directory)) { panic(fmt.Sprintf("%s execution_base_dir = %s, want parent", p.Template, p.ExecutionBaseDir)) } }; if mode == "target" && filepath.Clean(p.ExecutionBaseDir) != filepath.Clean(p.Directory) { panic(fmt.Sprintf("%s execution_base_dir = %s, want target", p.Template, p.ExecutionBaseDir)) }; if p.Template == "laravel" { validateLaravel(p) } }' 'func validateLaravel(p plan) { target := filepath.Base(p.Directory); for _, a := range p.Actions { if a.Type != "command" { continue }; if a.Command != "composer" { panic("laravel scaffold command is not composer") }; if len(a.Args) != 3 || a.Args[0] != "create-project" || a.Args[1] != "laravel/laravel" || a.Args[2] != target { panic(fmt.Sprintf("laravel scaffold args = %#v, want create-project laravel/laravel %s", a.Args, target)) }; return }; panic("laravel scaffold has no command action") }' > "$(SMOKE_DIR)/scaffoldcheck.go"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) system >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan toolchain >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan path >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan container >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) snapshot >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/node" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/go" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(DEPENDENCY_SMOKE_DIR)" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) about > "$(SMOKE_DIR)/about.txt"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) diagnose > "$(SMOKE_DIR)/diagnose.txt"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent plan --goal diagnose > "$(SMOKE_DIR)/agent-plan.txt"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation plan --goal diagnose > "$(SMOKE_DIR)/automation-plan.txt"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project templates > "$(SMOKE_DIR)/project-templates.txt"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) --plain diagnose > "$(SMOKE_DIR)/diagnose-plain.txt"
	@grep -q 'Envdoctor' "$(SMOKE_DIR)/about.txt"
	@grep -q 'Progress:' "$(SMOKE_DIR)/diagnose.txt"
	@grep -q 'Next steps' "$(SMOKE_DIR)/agent-plan.txt"
	@grep -q 'Policy Summary' "$(SMOKE_DIR)/automation-plan.txt"
	@grep -q 'Project Templates' "$(SMOKE_DIR)/project-templates.txt"
	@if grep -q 'Progress:' "$(SMOKE_DIR)/diagnose-plain.txt"; then \
		printf 'Plain output unexpectedly contains progress bars.\n'; \
		exit 1; \
	fi
	@if LC_ALL=C grep "$$(printf '\033')" "$(SMOKE_DIR)/diagnose-plain.txt" >/dev/null; then \
		printf 'Plain output unexpectedly contains ANSI escape codes.\n'; \
		exit 1; \
	fi
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) system --json > "$(SMOKE_DIR)/system.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan toolchain --json > "$(SMOKE_DIR)/toolchain.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan path --json > "$(SMOKE_DIR)/path.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan container --json > "$(SMOKE_DIR)/container.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies --json "$(DEPENDENCY_SMOKE_DIR)" > "$(SMOKE_DIR)/dependencies.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) snapshot --json > "$(SMOKE_DIR)/snapshot.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) diagnose --json > "$(SMOKE_DIR)/diagnose.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) explain --json "$(SMOKE_DIR)/envdoctor.log" > "$(SMOKE_DIR)/explain.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) recommend --json > "$(SMOKE_DIR)/recommend.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service list >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service status envdoctor-smoke-missing >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service diagnose --json envdoctor-smoke-missing > "$(SMOKE_DIR)/service.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service plan restart envdoctor-smoke-missing --json > "$(SMOKE_DIR)/service-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service apply restart envdoctor-smoke-missing --dry-run --json --audit-log "$(SMOKE_DIR)/service-apply-audit.jsonl" > "$(SMOKE_DIR)/service-apply.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) service apply restart envdoctor-smoke-missing --yes --json --profile production --audit-log "$(SMOKE_DIR)/service-apply-prod-audit.jsonl" > "$(SMOKE_DIR)/service-apply-prod.json"
	@grep -q '"profile": "production"' "$(SMOKE_DIR)/service-apply-prod.json"
	@grep -q '"allowed": false' "$(SMOKE_DIR)/service-apply-prod.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) version scan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/version-scan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) version plan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/version-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) install plan python --json > "$(SMOKE_DIR)/install-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) fix plan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/fix-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) bootstrap plan --json "$(SMOKE_DIR)/bootstrap" > "$(SMOKE_DIR)/bootstrap-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project scan --json "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-scan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project templates --json > "$(SMOKE_DIR)/project-templates.json"
	@if grep -Eq '"source": "(internal|manual)"' "$(SMOKE_DIR)/project-templates.json"; then \
		printf 'Project templates exposed internal/manual scaffold source.\n'; \
		exit 1; \
	fi
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan node --json "$(SMOKE_DIR)/project-empty" > "$(SMOKE_DIR)/project-init-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan react-vite --json --create-dir "$(SMOKE_DIR)/scaffold-react-official" > "$(SMOKE_DIR)/scaffold-react-plan.json"
	@grep -q '"source": "official"' "$(SMOKE_DIR)/scaffold-react-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan laravel --json --create-dir "$(SMOKE_DIR)/scaffold-laravel" > "$(SMOKE_DIR)/scaffold-laravel-plan.json"
	@grep -q '"source": "official"' "$(SMOKE_DIR)/scaffold-laravel-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/scaffoldcheck.go" path "$(SMOKE_DIR)/scaffold-laravel-plan.json"
	@before="$$(find "$(SMOKE_DIR)/scaffold-laravel-apply" -mindepth 1 2>/dev/null | wc -l | tr -d ' ')"; \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init apply laravel --dry-run --json --create-dir "$(SMOKE_DIR)/scaffold-laravel-apply" > "$(SMOKE_DIR)/scaffold-laravel-apply.json"; \
		after="$$(find "$(SMOKE_DIR)/scaffold-laravel-apply" -mindepth 1 2>/dev/null | wc -l | tr -d ' ')"; \
		if [ "$$before" != "$$after" ]; then \
			printf 'Laravel scaffold dry-run created files.\n'; \
			exit 1; \
		fi
	@for template in react-vite vue-vite next sveltekit nestjs laravel dotnet dotnet-console dotnet-webapi dart dart-console flutter flutter-app elixir; do \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan "$$template" --json --create-dir "$(SMOKE_DIR)/scaffold-matrix-$$template" > "$(SMOKE_DIR)/scaffold-matrix-$$template.json"; \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/scaffoldcheck.go" path "$(SMOKE_DIR)/scaffold-matrix-$$template.json"; \
	done
	@for template in node go go-module rust rust-cli rust-lib swift; do \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan "$$template" --json --create-dir "$(SMOKE_DIR)/scaffold-matrix-$$template" > "$(SMOKE_DIR)/scaffold-matrix-$$template.json"; \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/scaffoldcheck.go" target "$(SMOKE_DIR)/scaffold-matrix-$$template.json"; \
	done
	@if GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan go-web --json --create-dir "$(SMOKE_DIR)/scaffold-go-web" >/dev/null 2>&1; then \
		printf 'Project scaffold unexpectedly exposed go-web without a safe official generator.\n'; \
		exit 1; \
	fi
	@before="$$(find "$(SMOKE_DIR)/scaffold-go" -mindepth 1 | wc -l | tr -d ' ')"; \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init apply react-vite --dry-run --json --audit-log "$(SMOKE_DIR)/project-init-apply-audit.jsonl" "$(SMOKE_DIR)/scaffold-go" > "$(SMOKE_DIR)/project-init-apply.json"; \
		after="$$(find "$(SMOKE_DIR)/scaffold-go" -mindepth 1 | wc -l | tr -d ' ')"; \
		if [ "$$before" != "$$after" ]; then \
			printf 'Project scaffold dry-run created files.\n'; \
			exit 1; \
		fi
	@printf '%s\n' 'module example.com/conflict' > "$(SMOKE_DIR)/scaffold-conflict/go.mod"
	@if GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan go --allow-non-empty "$(SMOKE_DIR)/scaffold-conflict" >/dev/null 2>&1; then \
		printf 'Project scaffold unexpectedly allowed Go init over an existing module.\n'; \
		exit 1; \
	fi
	@if GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init plan ../x --json "$(SMOKE_DIR)/scaffold-react" >/dev/null 2>&1; then \
		printf 'Project scaffold unexpectedly allowed path traversal template id.\n'; \
		exit 1; \
	fi
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps plan sync --json "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-deps-sync.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps plan sync --ecosystem go --json "$(SMOKE_DIR)/go" > "$(SMOKE_DIR)/project-deps-sync-go.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps plan install lodash --ecosystem node --json "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-deps-install.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps plan update lodash --ecosystem node --json "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-deps-update.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps plan remove lodash --ecosystem node --json "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-deps-remove.json"
	@before="$$(cksum "$(SMOKE_DIR)/project-node/package.json")"; \
		GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps apply sync --dry-run --json --audit-log "$(SMOKE_DIR)/project-deps-apply-audit.jsonl" "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/project-deps-apply.json"; \
		after="$$(cksum "$(SMOKE_DIR)/project-node/package.json")"; \
		if [ "$$before" != "$$after" ]; then \
			printf 'Project deps dry-run mutated package.json.\n'; \
			exit 1; \
		fi
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init apply go --yes --json --audit-log "$(SMOKE_DIR)/project-init-yes-audit.jsonl" "$(SMOKE_DIR)/project-yes" > "$(SMOKE_DIR)/project-init-yes.json"
	@test -f "$(SMOKE_DIR)/project-yes/go.mod"
	@grep -q '"project_snapshot_file"' "$(SMOKE_DIR)/project-init-yes.json"
	@if GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project init apply go --dry-run "$(SMOKE_DIR)/project-node" >/dev/null 2>&1; then \
		printf 'Project init apply unexpectedly allowed non-empty directory.\n'; \
		exit 1; \
	fi
	@if GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) project deps apply remove --dry-run >/dev/null 2>&1; then \
		printf 'Project deps remove unexpectedly allowed missing package.\n'; \
		exit 1; \
	fi
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) fix apply --dry-run --json --audit-log "$(SMOKE_DIR)/fix-apply-audit.jsonl" "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/fix-apply.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) fix apply --dry-run --json --profile development --audit-log "$(SMOKE_DIR)/fix-apply-dev-audit.jsonl" "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/fix-apply-dev.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) fix apply --dry-run --json --profile production --audit-log "$(SMOKE_DIR)/fix-apply-prod-audit.jsonl" "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/fix-apply-prod.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) install apply python --dry-run --json --audit-log "$(SMOKE_DIR)/install-apply-audit.jsonl" > "$(SMOKE_DIR)/install-apply.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) install apply python --dry-run --json --profile production --audit-log "$(SMOKE_DIR)/install-apply-prod-audit.jsonl" > "$(SMOKE_DIR)/install-apply-prod.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) version apply --dry-run --json --audit-log "$(SMOKE_DIR)/version-apply-audit.jsonl" "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/version-apply.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) bootstrap apply --dry-run --json --audit-log "$(SMOKE_DIR)/bootstrap-apply-audit.jsonl" "$(SMOKE_DIR)/bootstrap" > "$(SMOKE_DIR)/bootstrap-apply.json"
	@grep -q '"profile": "production"' "$(SMOKE_DIR)/fix-apply-prod.json"
	@grep -q '"profile": "production"' "$(SMOKE_DIR)/install-apply-prod.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent plan --json --goal diagnose --profile development > "$(SMOKE_DIR)/agent-plan-diagnose.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent plan --json --goal onboard "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/agent-plan-onboard.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent plan --json --goal scaffold --template react-vite --create-dir "$(SMOKE_DIR)/agent-scaffold-plan" > "$(SMOKE_DIR)/agent-plan-scaffold.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent run --dry-run --json --goal repair "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/agent-run-repair.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent run --yes --json --goal scaffold --template go --create-dir --audit-log "$(SMOKE_DIR)/agent-scaffold-yes-audit.jsonl" "$(SMOKE_DIR)/agent-scaffold-yes" > "$(SMOKE_DIR)/agent-run-scaffold.json"
	@test -f "$(SMOKE_DIR)/agent-scaffold-yes/go.mod"
	@grep -q '"project_snapshot_file"' "$(SMOKE_DIR)/agent-run-scaffold.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) agent run --yes --json --profile production --goal scaffold --template go --create-dir --audit-log "$(SMOKE_DIR)/agent-prod-block-audit.jsonl" "$(SMOKE_DIR)/agent-prod-block" > "$(SMOKE_DIR)/agent-run-prod-block.json"
	@if [ -e "$(SMOKE_DIR)/agent-prod-block" ]; then \
		printf 'Production agent scaffold created a project directory despite policy block.\n'; \
		exit 1; \
	fi
	@grep -q '"allowed": false' "$(SMOKE_DIR)/agent-run-prod-block.json"
	@grep -q 'production profile blocks mutating apply actions' "$(SMOKE_DIR)/agent-run-prod-block.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation plan --json --goal diagnose --profile development > "$(SMOKE_DIR)/automation-plan-diagnose.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation plan --json --goal maintain "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/automation-plan-maintain.json"
	@grep -q '"maintain"' "$(SMOKE_DIR)/automation-plan-maintain.json"
	@grep -q '"repair"' "$(SMOKE_DIR)/automation-plan-maintain.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation plan --json --goal scaffold --template react-vite --create-dir "$(SMOKE_DIR)/automation-scaffold-plan" > "$(SMOKE_DIR)/automation-plan-scaffold.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation run --dry-run --json --goal repair "$(SMOKE_DIR)/project-node" > "$(SMOKE_DIR)/automation-run-repair.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) automation run --yes --json --profile production --goal scaffold --template go --create-dir --audit-log "$(SMOKE_DIR)/automation-prod-block-audit.jsonl" "$(SMOKE_DIR)/automation-prod-block" > "$(SMOKE_DIR)/automation-run-prod-block.json"
	@if [ -e "$(SMOKE_DIR)/automation-prod-block" ]; then \
		printf 'Production automation scaffold created a project directory despite policy block.\n'; \
		exit 1; \
	fi
	@grep -q '"allowed": false' "$(SMOKE_DIR)/automation-run-prod-block.json"
	@grep -q '"production_mutation_blocked": true' "$(SMOKE_DIR)/automation-run-prod-block.json"
	@test -s "$(SMOKE_DIR)/fix-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/fix-apply-dev-audit.jsonl"
	@test -s "$(SMOKE_DIR)/fix-apply-prod-audit.jsonl"
	@test -s "$(SMOKE_DIR)/install-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/install-apply-prod-audit.jsonl"
	@test -s "$(SMOKE_DIR)/service-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/service-apply-prod-audit.jsonl"
	@test -s "$(SMOKE_DIR)/version-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/bootstrap-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/project-init-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/project-deps-apply-audit.jsonl"
	@test -s "$(SMOKE_DIR)/project-init-yes-audit.jsonl"
	@test -s "$(SMOKE_DIR)/agent-scaffold-yes-audit.jsonl"
	@test -s "$(SMOKE_DIR)/agent-prod-block-audit.jsonl"
	@test -s "$(SMOKE_DIR)/automation-prod-block-audit.jsonl"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/jsoncheck.go" "$(SMOKE_DIR)/system.json" "$(SMOKE_DIR)/toolchain.json" "$(SMOKE_DIR)/path.json" "$(SMOKE_DIR)/container.json" "$(SMOKE_DIR)/dependencies.json" "$(SMOKE_DIR)/snapshot.json" "$(SMOKE_DIR)/diagnose.json" "$(SMOKE_DIR)/explain.json" "$(SMOKE_DIR)/recommend.json" "$(SMOKE_DIR)/service.json" "$(SMOKE_DIR)/service-plan.json" "$(SMOKE_DIR)/service-apply.json" "$(SMOKE_DIR)/service-apply-prod.json" "$(SMOKE_DIR)/version-scan.json" "$(SMOKE_DIR)/version-plan.json" "$(SMOKE_DIR)/install-plan.json" "$(SMOKE_DIR)/fix-plan.json" "$(SMOKE_DIR)/bootstrap-plan.json" "$(SMOKE_DIR)/project-scan.json" "$(SMOKE_DIR)/project-templates.json" "$(SMOKE_DIR)/project-init-plan.json" "$(SMOKE_DIR)/scaffold-react-plan.json" "$(SMOKE_DIR)/scaffold-laravel-plan.json" "$(SMOKE_DIR)/scaffold-laravel-apply.json" "$(SMOKE_DIR)/project-init-apply.json" "$(SMOKE_DIR)/project-deps-sync.json" "$(SMOKE_DIR)/project-deps-sync-go.json" "$(SMOKE_DIR)/project-deps-install.json" "$(SMOKE_DIR)/project-deps-update.json" "$(SMOKE_DIR)/project-deps-remove.json" "$(SMOKE_DIR)/project-deps-apply.json" "$(SMOKE_DIR)/project-init-yes.json" "$(SMOKE_DIR)/fix-apply.json" "$(SMOKE_DIR)/fix-apply-dev.json" "$(SMOKE_DIR)/fix-apply-prod.json" "$(SMOKE_DIR)/install-apply.json" "$(SMOKE_DIR)/install-apply-prod.json" "$(SMOKE_DIR)/version-apply.json" "$(SMOKE_DIR)/bootstrap-apply.json" "$(SMOKE_DIR)/agent-plan-diagnose.json" "$(SMOKE_DIR)/agent-plan-onboard.json" "$(SMOKE_DIR)/agent-plan-scaffold.json" "$(SMOKE_DIR)/agent-run-repair.json" "$(SMOKE_DIR)/agent-run-scaffold.json" "$(SMOKE_DIR)/agent-run-prod-block.json" "$(SMOKE_DIR)/automation-plan-diagnose.json" "$(SMOKE_DIR)/automation-plan-maintain.json" "$(SMOKE_DIR)/automation-plan-scaffold.json" "$(SMOKE_DIR)/automation-run-repair.json" "$(SMOKE_DIR)/automation-run-prod-block.json" "integrations/vscode/package.json"
	@test -f "integrations/vscode/extension.js"
	@test -f "integrations/jetbrains/README.md"
	@test -f "integrations/jetbrains/external-tools.xml"
	@grep -q 'Envdoctor Agent Plan JSON' "integrations/jetbrains/external-tools.xml"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) ui --script "diagnose,version:$(SMOKE_DIR)/version,fix:$(SMOKE_DIR)/version,bootstrap:$(SMOKE_DIR)/bootstrap,exit" > "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Envdoctor Dashboard' "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Menu' "$(SMOKE_DIR)/ui.txt"
	@grep -q 'About' "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Progress:' "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Next steps' "$(SMOKE_DIR)/ui.txt"
	@if grep -Eiq 'executed|installed|restarted|fixed' "$(SMOKE_DIR)/ui.txt"; then \
		printf 'UI smoke output contains mutating action wording.\n'; \
		exit 1; \
	fi
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
