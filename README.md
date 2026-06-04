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
- Node.js
- Go
- Rust
- Docker / Podman / Kubernetes
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

# Cross-platform release binaries
make release

# Remove generated artifacts only
make clean
```

Release builds are written to `dist/` for Linux, macOS, and Windows on `amd64` and `arm64`. Local development and production binaries are written to `bin/`.

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
```

#### Scan Toolchain
```sh
envdoctor scan toolchain
```
Detects installed development tools and versions (20+ tools supported — Git, Go, Rust, Python, Node.js, Docker, etc.).

#### Scan PATH
```sh
envdoctor scan path
```
Analyzes environment variables and path integrity. Detects missing directories, invalid entries, duplicate entries, and broken symbolic links.

---

### Phase 2 (Hardened / Verified)

#### Scan Dependencies
```sh
envdoctor scan dependencies
```
Analyzes project dependencies from multiple formats and validates their presence in the environment when the relevant language tooling is available.

Supported formats:
- `requirements.txt` / `pyproject.toml` (Python — pip)
- `package.json` (Node.js — npm)
- `go.mod` (Go)
- `Cargo.toml` (Rust)
- `composer.json` (PHP)

#### Scan Containers
```sh
envdoctor scan container
```
Validates container environments (Docker, Podman, Kubernetes).

#### Create Environment Snapshot
```sh
envdoctor snapshot

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
    └── recommendation           Recommendation engine
```

---

## Verification

```sh
make check
make smoke
```

With `_test.go` files intentionally removed, `go test ./...` is used as a package compile check inside `make check`. Manual smoke checks remain available for ad-hoc verification:

```sh
go run ./cmd/envdoctor system
go run ./cmd/envdoctor scan toolchain
go run ./cmd/envdoctor scan path
go run ./cmd/envdoctor diagnose --json
go run ./cmd/envdoctor scan container
go run ./cmd/envdoctor snapshot
go run ./cmd/envdoctor scan dependencies <fixture-dir>
go run ./cmd/envdoctor explain --json <logfile>
go run ./cmd/envdoctor recommend --json
go run ./cmd/envdoctor dockerize --output <output-file> <fixture-dir>
```

---

## Roadmap

| Phase | Status | Features |
|-------|--------|----------|
| **Phase 1** | ✅ Hardened / Verified | CLI Framework, OS-aware System Discovery, Toolchain Scanner, PATH Inspector |
| **Phase 2** | ✅ Hardened / Verified | Multi-manifest Dependency Doctor, Container Doctor, Snapshot System |
| **Phase 3** | ✅ Rule-Based Hardened | Log Analyzer, Recommendation Engine, Dockerfile Generator; AI and Auto-Fix planned |
| **Phase 4A** | ✅ Tooling Added | Makefile, Windows PowerShell helper, dev/prod/release/smoke workflows |
| **Phase 4** | 🚧 Planned | VSCode Extension, JetBrains Plugin, CI/CD Integration |
| **Phase 5** | 🚧 Planned | Multi-Agent Diagnostics, Remote Environment Analysis, Cloud Infrastructure Validation |

---

## License

MIT License
