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

## CLI Commands

### Phase 1 — MVP (Complete)

#### Diagnose Entire Environment
```sh
envdoctor diagnose
```
Runs a complete environment scan including system, toolchain, and PATH analysis.

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

### Phase 2 (Complete)

#### Scan Dependencies
```sh
envdoctor scan dependencies
```
Analyzes project dependencies from multiple formats and validates their presence in the environment.

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
```

#### Compare Environments
```sh
envdoctor compare snapshot-a.json snapshot-b.json
```

---

### Phase 3 (In Progress)

#### Analyze Logs
```sh
envdoctor explain <logfile>
```
Reads logs and performs root-cause analysis with suggested fixes.

#### Generate Dockerfile
```sh
envdoctor dockerize [directory]

# Save to Dockerfile
envdoctor dockerize --save
```
Auto-generates a Dockerfile based on the project type (Python, Node.js, Go, Rust).

#### Recommendations
```sh
envdoctor recommend
```
Analyzes the environment and generates actionable fix suggestions based on detected issues.

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
    └── recommendation          Recommendation engine
```

---

## Roadmap

| Phase | Status | Features |
|-------|--------|----------|
| **Phase 1** | ✅ Complete | CLI Framework, System Discovery, Toolchain Scanner, PATH Inspector |
| **Phase 2** | ✅ Complete | Dependency Doctor, Container Doctor, Snapshot System |
| **Phase 3** | 🔄 In Progress | AI Troubleshooting, Recommendation Engine, Auto-Fix Actions |
| **Phase 4** | 🚧 Planned | VSCode Extension, JetBrains Plugin, CI/CD Integration |
| **Phase 5** | 🚧 Planned | Multi-Agent Diagnostics, Remote Environment Analysis, Cloud Infrastructure Validation |

---

## License

MIT License
