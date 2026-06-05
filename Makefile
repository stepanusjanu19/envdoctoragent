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
DEPENDENCY_SMOKE_DIR ?= $(SMOKE_DIR)/dependencies

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
	@mkdir -p "$(SMOKE_DIR)/node" "$(SMOKE_DIR)/go" "$(SMOKE_DIR)/version" "$(SMOKE_DIR)/bootstrap"
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
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) system >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan toolchain >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan path >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan container >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) snapshot >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/node" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(SMOKE_DIR)/go" >/dev/null
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) scan dependencies "$(DEPENDENCY_SMOKE_DIR)" >/dev/null
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
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) version scan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/version-scan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) version plan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/version-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) install plan python --json > "$(SMOKE_DIR)/install-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) fix plan --json "$(SMOKE_DIR)/version" > "$(SMOKE_DIR)/fix-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) bootstrap plan --json "$(SMOKE_DIR)/bootstrap" > "$(SMOKE_DIR)/bootstrap-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run "$(SMOKE_DIR)/jsoncheck.go" "$(SMOKE_DIR)/system.json" "$(SMOKE_DIR)/toolchain.json" "$(SMOKE_DIR)/path.json" "$(SMOKE_DIR)/container.json" "$(SMOKE_DIR)/dependencies.json" "$(SMOKE_DIR)/snapshot.json" "$(SMOKE_DIR)/diagnose.json" "$(SMOKE_DIR)/explain.json" "$(SMOKE_DIR)/recommend.json" "$(SMOKE_DIR)/service.json" "$(SMOKE_DIR)/version-scan.json" "$(SMOKE_DIR)/version-plan.json" "$(SMOKE_DIR)/install-plan.json" "$(SMOKE_DIR)/fix-plan.json" "$(SMOKE_DIR)/bootstrap-plan.json"
	@GOCACHE="$(GOCACHE)" GOMODCACHE="$(GOMODCACHE)" $(GO) run $(PKG) ui --script "diagnose,version:$(SMOKE_DIR)/version,fix:$(SMOKE_DIR)/version,bootstrap:$(SMOKE_DIR)/bootstrap,exit" > "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Envdoctor Dashboard' "$(SMOKE_DIR)/ui.txt"
	@grep -q 'Menu' "$(SMOKE_DIR)/ui.txt"
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
