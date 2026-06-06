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

### Phase 4B — Service Agent (Read-Only + Safe Apply Preview)

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

#### Service Plan/Apply
```sh
envdoctor service plan <start|stop|restart> <name>
envdoctor service plan restart docker --json

envdoctor service apply <start|stop|restart> <name> --dry-run
envdoctor service apply restart docker --yes --profile development
envdoctor service apply restart docker --yes --profile production
```

Service status commands are read-only. Service apply uses the shared executor policy: dry-run by default, `--yes` required for mutation, structured command args only, audit log, timeout, and production profile blocking. Linux supports `systemctl start|stop|restart`; Windows supports `sc.exe start|stop` and restart as stop/start. macOS service mutation remains manual/blocked until a safe launchctl domain can be selected.

---

### Phase 4C — Version Manager Engine (Read-Only + Safe Apply Preview)

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

Generates suggested version-manager install/switch commands. `version apply` can dry-run or execute allowlisted version-manager actions through the shared executor, but `version fix` and autonomous runtime switching remain disabled.

---

### Phase 4D — Installation Advisor (Plan + Safe Apply Preview)

```sh
envdoctor install plan <tool>

# JSON output
envdoctor install plan <tool> --json
```

Suggests package-manager commands for Homebrew, apt, dnf, yum, pacman, zypper, winget, or Chocolatey when detected. `install apply` uses structured command args and executor policy; no arbitrary install shell is accepted.

Common runtime and package-manager mappings include Python, Node.js, Go, Rust, PHP, Composer, Java, Maven, Gradle, .NET SDK, Ruby, Dart, Swift, Elixir, Lua, R, Julia, Haskell, Perl, Conan, and vcpkg.

---

### Phase 4E — Safe Fix Plan + Bootstrap Plan (Plan + Safe Apply Preview)

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

Generates a project setup plan from runtime requirements, dependency manifests, and container/service hints. `fix apply` and `bootstrap apply` remain approval-gated executor previews. `diagnose --fix` and bare `bootstrap` remain disabled.

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

### Phase 5C — Safe Execution Preview

The first execution phase is approval-gated and deterministic. Existing `plan` commands remain read-only; new `apply` commands default to dry-run and only execute allowlisted actions when `--yes` is passed.

```sh
envdoctor fix apply [directory]
envdoctor install apply <tool>
envdoctor version apply [directory]
envdoctor bootstrap apply [directory]

# JSON dry-run with explicit audit log
envdoctor fix apply --dry-run --json --audit-log .cache/fix-audit.jsonl

# Mutating apply requires explicit approval
envdoctor bootstrap apply --yes <project-dir>
```

Safety behavior:
- `--dry-run` is the default and never runs suggested commands.
- `--yes` is required before any allowlisted command can execute.
- Every dry-run/apply writes an audit JSONL log; dry-run defaults to a temp audit path unless `--audit-log` is provided.
- `--yes` creates a pre-apply snapshot under `.envdoctor/snapshots/`.
- Shell operators and expansions such as `&&`, `;`, `|`, redirection, `$(`, and shell variables are blocked.
- Bare `service restart`/`service fix`, `diagnose --fix`, AI auto-fix, remote execution, and cloud mutation remain future work.

---

### Phase 5D — Project Lifecycle & Dependency Operations

Envdoctor can now plan and dry-run project initialization plus dependency lifecycle operations for new and existing projects. This layer uses the Phase 5C executor: dry-run is still the default, `--yes` is required for mutation, and every apply writes an audit log.

```sh
envdoctor project scan [directory]

envdoctor project init plan <template> [directory]
envdoctor project init apply <template> [directory] --dry-run

envdoctor project deps plan <sync|install|update|remove> [package] [directory]
envdoctor project deps apply <sync|install|update|remove> [package] [directory] --dry-run
```

Useful flags:

```sh
--json
--audit-log <file>
--timeout 2m
--ecosystem node
--manager pnpm
--dev
--version 1.2.3
--all
--allow-non-empty
```

Project initialization templates are official-only. Active starter templates use official ecosystem/framework generators; templates without a safe official generator are hidden from `project templates` and rejected if requested directly.

Dependency operations cover Node.js package managers, Python pip, Go modules, Cargo, Composer, Maven/Gradle sync, .NET packages, Dart pub, Bundler, SwiftPM sync, and Mix sync. Ecosystems without a safe command mapping return blocked or metadata-only actions instead of executing arbitrary shell commands.

Safety behavior:
- `project init apply` is blocked for non-empty directories unless `--allow-non-empty` is passed.
- `project deps remove` requires an explicit package name.
- Project actions use structured `command` + `args`; shell strings are not generated for new lifecycle actions.
- `--yes` creates a project snapshot under `.envdoctor/project-snapshots/` before running allowlisted commands.
- Project snapshots include manifest and lockfile checksums plus copies of known config files.

---

### Phase 5E — Project Scaffolding Agent

Project initialization now uses an official-only scaffold registry. Envdoctor plans official ecosystem/framework generators as structured `command` + `args` actions and does not expose internal/manual starter templates in the command surface.

```sh
envdoctor project templates
envdoctor project templates --json

envdoctor project init plan react-vite --json --create-dir <project-dir>
envdoctor project init plan laravel --json --create-dir <project-dir>
envdoctor project init apply go --dry-run --json --create-dir <project-dir>
envdoctor project init apply go --yes --json --create-dir <new-project-dir>
```

Scaffold flags:

```sh
--name my-app
--module github.com/acme/my-app
--package com.acme.app
--source auto|official
--create-dir
--force
--allow-non-empty
```

Active official scaffold templates include:

```text
node, react-vite, vue-vite, next, sveltekit, nestjs,
go, go-module,
rust, rust-cli, rust-lib,
laravel,
dotnet, dotnet-console, dotnet-webapi,
dart, dart-console, flutter, flutter-app,
swift, elixir
```

Safety behavior:
- `--source auto` selects the official generator. `--source internal` and `--source manual` are rejected.
- Envdoctor does not auto-install missing generators; apply output is blocked and includes an `envdoctor install plan <tool>` hint.
- `--create-dir` uses a safe project-local `mkdir` action when the target directory does not exist.
- Path traversal, absolute paths, symlink project roots/components, delete commands, and shell operators are blocked.
- Known generated manifest files are guarded and are not overwritten unless `--force` is passed.
- `--create-dir` is required when the target directory does not exist.
- Templates without a stable official generator, such as `go-web`, are hidden from `project templates` and rejected if requested directly.
- AI-generated project synthesis remains future work.

---

### Phase 5F — Completion Hardening and Policy Profiles

Apply commands now share an explicit policy layer for developer and production use:

```sh
envdoctor fix apply --dry-run --json --profile development
envdoctor fix apply --dry-run --json --profile production
envdoctor install apply python --dry-run --json --profile production
```

Policy flags:

```sh
--profile development|production
--max-risk low|medium|high
--policy-file policy.json
```

Behavior:
- `development` keeps the Phase 5C behavior: dry-run by default, `--yes` required for allowlisted mutation.
- `production` blocks mutating apply actions by default, even when `--yes` is passed.
- Policy decisions are included in JSON output and audit logs with profile, max risk, decision, and blocked reason.
- Install apply uses structured command plus args for package managers; shell strings with operators are not used for Linux install actions.
- `--policy-file` can set profile, max risk, and future policy switches, while CLI flags remain the explicit override.

Example policy file:

```json
{
  "profile": "production",
  "max_risk": "medium",
  "allow_production_mutation": false
}
```

---

### Phase 6A — Autonomous Agent Preview

The first agent phase is a local deterministic orchestrator over existing engines. It does not use an LLM and does not bypass the Phase 5F policy layer.

```sh
envdoctor agent plan [directory]
envdoctor agent run [directory] --dry-run
```

Agent goals:

```sh
--goal diagnose
--goal onboard
--goal repair
--goal scaffold --template react-vite --create-dir
--goal bootstrap
```

Examples:

```sh
envdoctor agent plan --json --goal diagnose --profile development
envdoctor agent plan --json --goal onboard <project-dir>
envdoctor agent run --dry-run --json --goal repair <project-dir>
envdoctor agent run --yes --json --goal scaffold --template go --create-dir <new-dir>
envdoctor agent run --yes --json --profile production --goal scaffold --template go --create-dir <new-dir>
```

Behavior:
- `agent plan` never executes actions.
- `agent run` defaults to dry-run.
- `agent run --yes` executes only actions that pass executor allowlist and policy decisions.
- Production profile remains read-only/plan-only in v1; mutating production automation is future work.
- AI troubleshooting, bare `service restart`/`service fix`, `diagnose --fix`, remote/cloud execution, and autonomous production mutation remain future phases.

---

### Phase 5 Integration — IDE CLI Wrappers

The IDE integration preview keeps Envdoctor as the single engine and wraps CLI JSON output:

```text
integrations/vscode
integrations/jetbrains
```

- VSCode wrapper commands: Diagnose, Fix Plan, and Agent Plan.
- VSCode settings: `envdoctor.path`, `envdoctor.profile`, and `envdoctor.goal`.
- JetBrains integration uses External Tools templates for diagnose, fix plan, and agent plan.
- IDE wrappers do not call `apply`, install tools, restart services, or mutate project files.

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
    ├── executor                 Approval-gated dry-run/apply engine
    ├── projectops               Project init and dependency lifecycle operations
    ├── scaffold                 Official-only project scaffold registry and safe generator actions
    ├── agent                    Deterministic local autonomous orchestration preview
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
go run ./cmd/envdoctor service plan restart <service-name> --json
go run ./cmd/envdoctor service apply restart <service-name> --dry-run --json
go run ./cmd/envdoctor service apply restart <service-name> --yes --json --profile production
go run ./cmd/envdoctor version scan --json <fixture-dir>
go run ./cmd/envdoctor version plan --json <fixture-dir>
go run ./cmd/envdoctor install plan python --json
go run ./cmd/envdoctor fix plan --json
go run ./cmd/envdoctor bootstrap plan --json <fixture-dir>
go run ./cmd/envdoctor fix apply --dry-run --json
go run ./cmd/envdoctor install apply python --dry-run --json
go run ./cmd/envdoctor version apply --dry-run --json <fixture-dir>
go run ./cmd/envdoctor bootstrap apply --dry-run --json <fixture-dir>
go run ./cmd/envdoctor fix apply --dry-run --json --profile development
go run ./cmd/envdoctor fix apply --dry-run --json --profile production
go run ./cmd/envdoctor install apply python --dry-run --json --profile production
go run ./cmd/envdoctor project scan --json <fixture-dir>
go run ./cmd/envdoctor project templates --json
go run ./cmd/envdoctor project init plan node --json <empty-dir>
go run ./cmd/envdoctor project init apply go --dry-run --json <empty-dir>
go run ./cmd/envdoctor project init plan react-vite --json <empty-dir>
go run ./cmd/envdoctor project init plan laravel --json --create-dir <empty-dir>
go run ./cmd/envdoctor project init apply react-vite --dry-run --json --create-dir <empty-dir>
go run ./cmd/envdoctor project deps plan sync --json <fixture-dir>
go run ./cmd/envdoctor project deps plan sync --ecosystem go --json <fixture-dir>
go run ./cmd/envdoctor project deps plan install lodash --ecosystem node --json <fixture-dir>
go run ./cmd/envdoctor project deps apply sync --dry-run --json <fixture-dir>
go run ./cmd/envdoctor agent plan --json --goal diagnose --profile development
go run ./cmd/envdoctor agent plan --json --goal onboard <fixture-dir>
go run ./cmd/envdoctor agent plan --json --goal scaffold --template react-vite --create-dir <new-dir>
go run ./cmd/envdoctor agent run --dry-run --json --goal repair <fixture-dir>
go run ./cmd/envdoctor agent run --yes --json --profile production --goal scaffold --template go --create-dir <new-dir>
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
| **Phase 4B** | ✅ Safe Execution Preview | Service list/status/diagnose plus policy-gated service plan/apply |
| **Phase 4C** | ✅ Safe Execution Preview | Version manager detection, runtime requirement scan, version plan/apply |
| **Phase 4D** | ✅ Safe Execution Preview | Installation advisor with structured package-manager plan/apply |
| **Phase 4E** | ✅ Safe Execution Preview | Safe fix/bootstrap plan and approval-gated apply |
| **Phase 4F** | ✅ CLI UX Stabilized | JSON output for legacy scan commands and explicit status wording |
| **Phase 4G** | ✅ Non-Mutating UI Preview | Interactive CLI menu for read-only and plan-only workflows |
| **Phase 4H** | ✅ Metadata Registry Implemented | Broad dependency manifest registry with metadata-only coverage |
| **Phase 4I** | ✅ UI Polish Implemented | ANSI dashboard and stable stdlib terminal UI |
| **Phase 5** | ✅ CI + IDE Wrapper Preview | GitHub Actions check/smoke/release workflow; VSCode and JetBrains CLI wrappers |
| **Phase 5B** | ✅ Release Packaging Implemented | GoReleaser archives, checksums, Linux packages, and package-manager metadata |
| **Phase 5C** | ✅ Safe Execution Preview | Approval-gated `apply` commands, dry-run default, audit log, pre-apply snapshot |
| **Phase 5D** | ✅ Project Lifecycle Preview | Project scan/init/deps plan and approval-gated dependency operations |
| **Phase 5E** | ✅ Official Scaffolding Preview | Official-only starter registry with structured generator actions and guarded manifests |
| **Phase 5F** | ✅ Policy Hardening Preview | Development/production profiles, max-risk policy, policy audit metadata, structured install actions |
| **Phase 6A** | ✅ Autonomous Agent Preview | Local deterministic agent plan/run over diagnose/onboard/repair/scaffold/bootstrap goals |
| **Phase 6B+** | 🚧 Future AI / Remote | AI troubleshooting, autonomous auto-fix, service fix automation, multi-agent diagnostics, remote/cloud validation |

---

## License

MIT License
