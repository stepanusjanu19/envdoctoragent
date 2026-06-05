# Environment Doctor Agent

> Intelligent cross-platform environment diagnostics, troubleshooting, and automation for developers.

[![Build](https://img.shields.io/badge/build-passing-success)](./)
[![Go Version](https://img.shields.io/badge/go-%3E%3D%201.20-blue)](./)
[![License](https://img.shields.io/badge/license-MIT-green)](./)

Environment Doctor Agent is an automation and diagnostic platform that detects, analyzes, and resolves development environment issues across multiple operating systems and programming ecosystems.

---

## Supported Platforms

- Linux
- Windows
- macOS

## Supported Ecosystems

- Python
- Node.js / npm / yarn / pnpm / bun
- Go
- Rust
- PHP / Composer
- Java / Kotlin / Maven / Gradle
- .NET / NuGet
- Ruby / Bundler
- Dart / Flutter
- Swift Package Manager
- Elixir / Mix
- Lua / LuaRocks
- R / CRAN DESCRIPTION
- Julia
- Haskell / Cabal / Stack
- Perl / cpanfile
- C/C++ / Conan / vcpkg
- Docker / Podman / Kubernetes
- OS Services (read-only)
- Version Managers (read-only)
- System Toolchains

---

## Installation

```sh
# Build from source
go build -o envdoctor ./cmd/envdoctor
```

---

## Developer Workflow

Linux, macOS, and Windows via Git Bash or WSL:

```sh
make help
make check
make dev ARGS="diagnose --json"
make smoke
```

Native Windows PowerShell:

```powershell
pwsh ./scripts/windows.ps1 check
pwsh ./scripts/windows.ps1 build
pwsh ./scripts/windows.ps1 smoke
```

Common Makefile variables:

```sh
make dev ARGS="scan path"
make build BINARY=envdoctor
make release DIST_DIR=dist
make check GOCACHE=.cache/go-build GOMODCACHE=.cache/go-mod
```

---

## Production Build

```sh
# Current platform optimized binary
make prod

# Cross-platform raw binaries
make release-binaries

# Validate release packaging config
make release-check

# Cross-platform archives, checksums, Linux packages, and package-manager manifests
make release

# Publish GitHub Release assets from a v* tag in CI
make release-publish

# Remove generated artifacts only
make clean
```

Release packaging requires Go and GoReleaser OSS. `make release` installs the configured GoReleaser version into `.cache/tools` when it is missing.

Release builds are written to `dist/` for Linux, macOS, and Windows on `amd64` and `arm64`. Local development and production binaries are written to `bin/`.

Generated release artifacts include:
- `.tar.gz` archives for Linux and macOS.
- `.zip` archives for Windows.
- `checksums.txt`.
- Linux native packages: `.deb`, `.rpm`, `.apk`, and Arch package.
- Package-manager metadata under `dist/package-managers/` for Homebrew, Scoop, Winget, and Chocolatey.

External package-manager publishing is intentionally not automated yet. Generated metadata can be published manually after repository/token decisions are made.

GitHub Actions runs release snapshot packaging on normal push and pull request events. Tags matching `v*` can publish GoReleaser GitHub Release assets with `GITHUB_TOKEN`.

---

## CLI Commands

### Phase 1 — MVP (Hardened / Verified)

#### Diagnose Entire Environment
```sh
envdoctor diagnose
```
Runs a complete environment scan including system, toolchain, PATH, container checks, and rule-based recommendations.

```sh
# JSON output
envdoctor diagnose --json
```

#### System Information
```sh
envdoctor system

# JSON output
envdoctor system --json
```

#### Scan Toolchain
```sh
envdoctor scan toolchain

# JSON output
envdoctor scan toolchain --json
```
Detects installed development tools and versions (20+ tools supported — Git, Go, Rust, Python, Node.js, Docker, etc.).

#### Scan PATH
```sh
envdoctor scan path

# JSON output
envdoctor scan path --json
```
Analyzes environment variables and path integrity. Detects missing directories, invalid entries, duplicate entries, and broken symbolic links.

---

### Phase 2 (Hardened / Verified)

#### Scan Dependencies
```sh
envdoctor scan dependencies

# JSON output
envdoctor scan dependencies --json
```
Analyzes project dependencies from multiple formats and validates their presence in the environment when the relevant language tooling is available.

Supported formats:
- `requirements.txt` / `pyproject.toml` (Python — pip)
- `package.json` (Node.js — npm, yarn, pnpm, bun; detects React, Vue, Angular, Next, Nuxt, Svelte, Vite, NestJS, Express)
- `go.mod` (Go)
- `Cargo.toml` (Rust)
- `composer.json` (PHP)
- `pom.xml` (Java/Kotlin — Maven)
- `build.gradle` / `build.gradle.kts` (Java/Kotlin — Gradle)
- `.csproj`, `packages.config`, `Directory.Packages.props` (.NET — NuGet)
- `Gemfile` (Ruby — Bundler)
- `pubspec.yaml` / `pubspec.yml` (Dart/Flutter)
- `Package.swift` (Swift Package Manager)
- `mix.exs` (Elixir)
- `.rockspec` (LuaRocks)
- `DESCRIPTION` (R)
- `Project.toml` (Julia)
- `.cabal` / `stack.yaml` (Haskell)
- `cpanfile` (Perl)
- `vcpkg.json`, `conanfile.txt`, `conanfile.py` (C/C++)

Installed-version validation is available for Python, Node.js, Go, Rust, and PHP when the relevant local tooling exists. Other ecosystems are reported as `validation_status: "metadata-only"` instead of failing the scan.

#### Scan Containers
```sh
envdoctor scan container

# JSON output
envdoctor scan container --json
```
Validates container environments (Docker, Podman, Kubernetes).

#### Create Environment Snapshot
```sh
envdoctor snapshot

# JSON output
envdoctor snapshot --json

# Save to file
envdoctor snapshot --save

# Save to a specific file
envdoctor snapshot --output snapshot-local.json
```

#### Compare Environments
```sh
envdoctor compare snapshot-a.json snapshot-b.json
```

---

### Phase 3 (Rule-Based Hardened)

#### Analyze Logs
```sh
envdoctor explain <logfile>

# JSON output
envdoctor explain --json <logfile>
```
Reads logs and performs rule-based root-cause analysis with severity, category, confidence, matched line counts, and suggested fixes.

#### Generate Dockerfile
```sh
envdoctor dockerize [directory]

# Save to Dockerfile
envdoctor dockerize --save

# Save to a specific file
envdoctor dockerize --output Dockerfile.generated

# Overwrite an existing Dockerfile or output file
envdoctor dockerize --save --force
```
Auto-generates a safer Dockerfile based on the project type (Python, Node.js, Go, Rust), with dependency cache layers, non-root runtime users where feasible, and entrypoint suggestions.

#### Recommendations
```sh
envdoctor recommend

# JSON output
envdoctor recommend --json
```
Analyzes Phase 1/2 scan results and generates actionable rule-based recommendations with source, risk, confidence, and safe-to-run metadata.

Phase 3 uses deterministic rules. LLM-powered troubleshooting and automatic repair actions remain planned future work.

---

### Phase 4B — Service Agent (Read-Only Preview)

#### List Services
```sh
envdoctor service list

# JSON output
envdoctor service list --json
```

#### Service Status
```sh
envdoctor service status <name>

# JSON output
envdoctor service status --json <name>
```

#### Service Diagnose
```sh
envdoctor service diagnose <name>

# JSON output
envdoctor service diagnose --json <name>
```

Service commands inspect native service managers only. They do not restart, enable, disable, or modify services.

---

### Phase 4C — Version Manager Engine (Read-Only / Plan-Only Preview)

#### Scan Runtime Versions
```sh
envdoctor version scan [directory]

# JSON output
envdoctor version scan --json [directory]
```

Detects version managers (`nvm`, `fnm`, `pyenv`, `rustup`, `asdf`, `sdkman`, `goenv`), installed runtimes, and project runtime requirements from files such as `.nvmrc`, `.python-version`, `rust-toolchain.toml`, `go.mod`, `package.json`, and `pyproject.toml`.

#### Version Plan
```sh
envdoctor version plan [directory]

# JSON output
envdoctor version plan --json [directory]
```

Generates suggested version-manager install/switch commands. Envdoctor does not run auto-switch commands.

---

### Phase 4D — Installation Advisor (Plan-Only Preview)

```sh
envdoctor install plan <tool>

# JSON output
envdoctor install plan <tool> --json
```

Suggests package-manager commands for Homebrew, apt, dnf, yum, pacman, zypper, winget, or Chocolatey when detected. Envdoctor does not install tools in this phase.

Common runtime and package-manager mappings include Python, Node.js, Go, Rust, PHP, Composer, Java, Maven, Gradle, .NET SDK, Ruby, Dart, Swift, Elixir, Lua, R, Julia, Haskell, Perl, Conan, and vcpkg.

---

### Phase 4E — Safe Fix Plan + Bootstrap Plan (Plan-Only Preview)

#### Safe Fix Plan
```sh
envdoctor fix plan

# JSON output
envdoctor fix plan --json
```

Combines recommendations, PATH/container findings, version mismatches, and service manager availability into a non-mutating fix plan.

#### Bootstrap Plan
```sh
envdoctor bootstrap plan [directory]

# JSON output
envdoctor bootstrap plan --json [directory]
```

Generates a project setup plan from runtime requirements, dependency manifests, and container/service hints. Envdoctor does not install dependencies, start services, or modify project files.

---

### Phase 4F — CLI UX Stabilization

The command surface now has stable JSON output for both legacy and preview workflows:

```sh
envdoctor system --json
envdoctor scan toolchain --json
envdoctor scan path --json
envdoctor scan container --json
envdoctor scan dependencies --json [directory]
envdoctor snapshot --json
```

Status wording is kept explicit in docs and UI: implemented, read-only, plan-only, and future mutating.

---

### Phase 4G — Interactive CLI UI (Non-Mutating Preview)

```sh
envdoctor ui

# Deterministic smoke/script mode
envdoctor ui --script "diagnose,version,fix,bootstrap,exit"
```

The UI is a terminal menu for Diagnose, Service, Version, Install Plan, Fix Plan, Bootstrap Plan, Snapshot, and Logs. It only runs read-only or plan-only workflows and never executes install, restart, fix, runtime switch, or bootstrap actions.

---

### Phase 4H — Dependency Registry Expansion

`scan dependencies` now uses an extensible manifest registry. It scans supported manifests in the target directory and limited subdirectories while ignoring common generated directories such as `.git`, `node_modules`, `vendor`, `target`, `dist`, `build`, `.venv`, `venv`, and `.cache`.

Dependency JSON includes registry metadata for automation and future UI work:

```json
{
  "ecosystem": "node",
  "package_manager": "pnpm",
  "manifest_type": "package.json",
  "scope": "mixed",
  "validation_status": "validated",
  "status": "detected"
}
```

`bootstrap plan` consumes the same dependency metadata to produce per-ecosystem setup suggestions. `fix plan` includes dependency review actions for metadata-only ecosystems and missing validated dependencies. All actions remain plan-only.

---

### Phase 4I — Terminal UI Polish

`envdoctor ui` uses a lightweight stdlib-only terminal interface with ANSI styling, a compact dashboard, status labels, aligned summary rows, and deterministic script mode:

```sh
envdoctor ui --script "diagnose,version,fix,bootstrap,exit"
```

Set `NO_COLOR=1` to disable ANSI styling. The UI remains non-mutating and does not run install, restart, runtime switch, fix, or bootstrap changes.

---

## Project Architecture

```
envdoctor
├── cmd/envdoctor               CLI entrypoint
└── internal
    ├── scanner
    │   ├── toolchain             Detect dev tools
    │   └── path                  PATH analysis
    ├── system                    OS / kernel / shell discovery
    ├── dependencies              Dependency parsing & validation
    ├── container                 Docker / Podman / Kubernetes checks
    ├── snapshot                  Environment snapshot & comparison
    ├── diagnose                  Full diagnosis orchestration
    ├── analyzer                  Log analysis (root cause)
    ├── dockerize                 Dockerfile generator
    ├── recommendation           Recommendation engine
    ├── service                  Read-only service discovery
    ├── version                  Version manager and runtime planning
    ├── installplan              Package manager install advisor
    ├── fixplan                  Safe fix planning
    ├── bootstrap                Project bootstrap planning
    └── cliui                    Interactive non-mutating CLI UI
```

---

## Verification

```sh
make check
make smoke
make release-check
make release
```

With `_test.go` files intentionally removed, `go test ./...` is used as a package compile check inside `make check`. Manual smoke checks remain available for ad-hoc verification:

```sh
go run ./cmd/envdoctor system --json
go run ./cmd/envdoctor scan toolchain --json
go run ./cmd/envdoctor scan path --json
go run ./cmd/envdoctor diagnose --json
go run ./cmd/envdoctor scan container --json
go run ./cmd/envdoctor snapshot --json
go run ./cmd/envdoctor scan dependencies --json <fixture-dir>
go run ./cmd/envdoctor explain --json <logfile>
go run ./cmd/envdoctor recommend --json
go run ./cmd/envdoctor dockerize --output <output-file> <fixture-dir>
go run ./cmd/envdoctor service list
go run ./cmd/envdoctor service status <service-name>
go run ./cmd/envdoctor version scan --json <fixture-dir>
go run ./cmd/envdoctor version plan --json <fixture-dir>
go run ./cmd/envdoctor install plan python --json
go run ./cmd/envdoctor fix plan --json
go run ./cmd/envdoctor bootstrap plan --json <fixture-dir>
go run ./cmd/envdoctor ui --script "diagnose,version,fix,bootstrap,exit"
```

---

## Roadmap

| Phase | Status | Features |
|-------|--------|----------|
| **Phase 1** | ✅ Hardened / Verified | CLI Framework, OS-aware System Discovery, Toolchain Scanner, PATH Inspector |
| **Phase 2** | ✅ Hardened / Verified | Multi-manifest Dependency Doctor, Container Doctor, Snapshot System |
| **Phase 3** | ✅ Rule-Based Hardened | Log Analyzer, Recommendation Engine, Dockerfile Generator; AI and Auto-Fix planned |
| **Phase 4A** | ✅ Implemented | Makefile, Windows PowerShell helper, dev/prod/release/smoke workflows |
| **Phase 4B** | ✅ Read-Only Preview | Service list/status/diagnose without restart or fix actions |
| **Phase 4C** | ✅ Read-Only / Plan-Only Preview | Version manager detection, runtime requirement scan, version plan |
| **Phase 4D** | ✅ Plan-Only Preview | Installation advisor with package-manager command suggestions |
| **Phase 4E** | ✅ Plan-Only Preview | Safe fix plan and bootstrap plan without applying changes |
| **Phase 4F** | ✅ CLI UX Stabilized | JSON output for legacy scan commands and explicit status wording |
| **Phase 4G** | ✅ Non-Mutating UI Preview | Interactive CLI menu for read-only and plan-only workflows |
| **Phase 4H** | ✅ Metadata Registry Implemented | Broad dependency manifest registry with metadata-only coverage |
| **Phase 4I** | ✅ UI Polish Implemented | ANSI dashboard and stable stdlib terminal UI |
| **Phase 5** | ✅ CI Added / 🚧 IDE Planned | GitHub Actions check/smoke/release workflow; VSCode and JetBrains wrappers planned |
| **Phase 5B** | ✅ Release Packaging Implemented | GoReleaser archives, checksums, Linux packages, and package-manager metadata |
| **Phase 6** | 🚧 Future Mutating / AI | Approval-gated auto-fix, AI troubleshooting, multi-agent diagnostics, remote/cloud validation |

---

## License

MIT License
