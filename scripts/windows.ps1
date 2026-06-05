param(
    [ValidateSet("help", "fmt", "fmt-check", "vet", "test", "check", "build", "dev", "prod", "release", "smoke", "clean")]
    [string]$Target = "help",
    [string]$Args = "help",
    [string]$Binary = "envdoctor",
    [string]$BinDir = "bin",
    [string]$DistDir = "dist",
    [string]$CacheDir = ".cache"
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Pkg = "./cmd/envdoctor"
$GoCache = Join-Path $Root (Join-Path $CacheDir "go-build")
$GoModCache = Join-Path $Root (Join-Path $CacheDir "go-mod")
$SmokeDir = Join-Path $Root (Join-Path $CacheDir "smoke")

function Invoke-Go {
    param([string[]]$GoArgs)

    $previousGoCache = $env:GOCACHE
    $previousGoModCache = $env:GOMODCACHE
    $env:GOCACHE = $GoCache
    $env:GOMODCACHE = $GoModCache
    try {
        & go @GoArgs
        if ($LASTEXITCODE -ne 0) {
            throw "go $($GoArgs -join ' ') failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:GOCACHE = $previousGoCache
        $env:GOMODCACHE = $previousGoModCache
    }
}

function Invoke-GoOutput {
    param([string[]]$GoArgs)

    $previousGoCache = $env:GOCACHE
    $previousGoModCache = $env:GOMODCACHE
    $env:GOCACHE = $GoCache
    $env:GOMODCACHE = $GoModCache
    try {
        $output = & go @GoArgs
        if ($LASTEXITCODE -ne 0) {
            throw "go $($GoArgs -join ' ') failed with exit code $LASTEXITCODE"
        }
        return $output
    } finally {
        $env:GOCACHE = $previousGoCache
        $env:GOMODCACHE = $previousGoModCache
    }
}

function Get-GoFiles {
    Get-ChildItem -Path (Join-Path $Root "cmd"), (Join-Path $Root "internal") -Recurse -Filter "*.go" |
        ForEach-Object { $_.FullName }
}

function Invoke-Fmt {
    $files = @(Get-GoFiles)
    if ($files.Count -gt 0) {
        & gofmt -w @files
    }
}

function Invoke-FmtCheck {
    $files = @(Get-GoFiles)
    if ($files.Count -eq 0) {
        return
    }
    $unformatted = @(& gofmt -l @files)
    if ($unformatted.Count -gt 0) {
        Write-Host "Go files need formatting:"
        $unformatted | ForEach-Object { Write-Host $_ }
        throw "gofmt check failed"
    }
}

function Invoke-Test {
    Invoke-Go @("test", "./...")
}

function Invoke-Vet {
    Invoke-Go @("vet", "./...")
}

function Invoke-Check {
    Invoke-FmtCheck
    Invoke-Test
    Invoke-Vet
}

function Get-LocalBinaryPath {
    $goos = (& go env GOOS).Trim()
    $suffix = if ($goos -eq "windows") { ".exe" } else { "" }
    Join-Path $Root (Join-Path $BinDir "$Binary$suffix")
}

function Invoke-Build {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $BinDir) | Out-Null
    $out = Get-LocalBinaryPath
    Invoke-Go @("build", "-o", $out, $Pkg)
    Write-Host "Built $out"
}

function Invoke-Prod {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $BinDir) | Out-Null
    $out = Get-LocalBinaryPath
    Invoke-Go @("build", "-trimpath", "-ldflags=-s -w", "-o", $out, $Pkg)
    Write-Host "Built production binary $out"
}

function Invoke-Release {
    $targets = @(
        @{ GOOS = "linux"; GOARCH = "amd64" },
        @{ GOOS = "linux"; GOARCH = "arm64" },
        @{ GOOS = "darwin"; GOARCH = "amd64" },
        @{ GOOS = "darwin"; GOARCH = "arm64" },
        @{ GOOS = "windows"; GOARCH = "amd64" },
        @{ GOOS = "windows"; GOARCH = "arm64" }
    )

    $distPath = Join-Path $Root $DistDir
    New-Item -ItemType Directory -Force -Path $distPath | Out-Null

    $previousGoos = $env:GOOS
    $previousGoarch = $env:GOARCH
    $previousCgo = $env:CGO_ENABLED
    try {
        foreach ($target in $targets) {
            $env:GOOS = $target.GOOS
            $env:GOARCH = $target.GOARCH
            $env:CGO_ENABLED = "0"
            $suffix = if ($target.GOOS -eq "windows") { ".exe" } else { "" }
            $out = Join-Path $distPath "$Binary-$($target.GOOS)-$($target.GOARCH)$suffix"
            Write-Host "Building $out"
            Invoke-Go @("build", "-trimpath", "-ldflags=-s -w", "-o", $out, $Pkg)
        }
    } finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
        $env:CGO_ENABLED = $previousCgo
    }
}

function Invoke-Dev {
    Invoke-Go (@("run", $Pkg) + ($Args -split " "))
}

function Invoke-Smoke {
    $nodeDir = Join-Path $SmokeDir "node"
    $goDir = Join-Path $SmokeDir "go"
    $versionDir = Join-Path $SmokeDir "version"
    $bootstrapDir = Join-Path $SmokeDir "bootstrap"
    $dependencyDir = Join-Path $SmokeDir "dependencies"
    New-Item -ItemType Directory -Force -Path $nodeDir, $goDir, $versionDir, $bootstrapDir | Out-Null
    @(
        "python", "node", "go", "rust", "php",
        "maven", "gradle", "dotnet", "nuget-config", "nuget-props",
        "ruby", "dart", "swift", "elixir", "lua",
        "r", "julia", "cabal", "stack", "perl",
        "vcpkg", "conan-txt", "conan-py"
    ) | ForEach-Object {
        New-Item -ItemType Directory -Force -Path (Join-Path $dependencyDir $_) | Out-Null
    }

    @(
        "ERROR: Cannot connect to Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
        "npm ERR! code ERESOLVE",
        "ModuleNotFoundError: No module named 'requests'",
        "ssh: connect to host example.com port 22: Connection refused"
    ) | Set-Content -Path (Join-Path $SmokeDir "envdoctor.log") -Encoding UTF8

    '{"dependencies":{"express":"^5.0.0"},"scripts":{"start":"node index.js"}}' |
        Set-Content -Path (Join-Path $nodeDir "package.json") -Encoding UTF8

    @("module example.com/envdoctor-smoke", "", "go 1.22") |
        Set-Content -Path (Join-Path $goDir "go.mod") -Encoding UTF8

    "18.19.0" | Set-Content -Path (Join-Path $versionDir ".nvmrc") -Encoding UTF8
    "3.12.0" | Set-Content -Path (Join-Path $versionDir ".python-version") -Encoding UTF8
    @("[toolchain]", "channel = `"stable`"") |
        Set-Content -Path (Join-Path $versionDir "rust-toolchain.toml") -Encoding UTF8
    @("module example.com/envdoctor-version", "", "go 1.22") |
        Set-Content -Path (Join-Path $versionDir "go.mod") -Encoding UTF8
    '{"engines":{"node":">=18"}}' |
        Set-Content -Path (Join-Path $versionDir "package.json") -Encoding UTF8
    @("[project]", "requires-python = `">=3.11`"") |
        Set-Content -Path (Join-Path $versionDir "pyproject.toml") -Encoding UTF8
    '{"dependencies":{"express":"^5.0.0"}}' |
        Set-Content -Path (Join-Path $bootstrapDir "package.json") -Encoding UTF8
    "FROM node:22-alpine" |
        Set-Content -Path (Join-Path $bootstrapDir "Dockerfile") -Encoding UTF8

    @("requests>=2.31", "-r common.txt", "--extra-index-url https://example.invalid/simple") |
        Set-Content -Path (Join-Path $dependencyDir "python/requirements.txt") -Encoding UTF8
    @("[project]", "dependencies = [`"requests>=2.31`"]", "[project.optional-dependencies]", "dev = [`"pytest>=8`"]") |
        Set-Content -Path (Join-Path $dependencyDir "python/pyproject.toml") -Encoding UTF8
    '{"packageManager":"pnpm@9.0.0","dependencies":{"react":"^18.2.0","next":"^14.0.0","express":"^5.0.0"},"devDependencies":{"vite":"^5.0.0"},"peerDependencies":{"vue":"^3.0.0"},"optionalDependencies":{"svelte":"^4.0.0"}}' |
        Set-Content -Path (Join-Path $dependencyDir "node/package.json") -Encoding UTF8
    @("module example.com/dependencies-smoke", "", "go 1.22") |
        Set-Content -Path (Join-Path $dependencyDir "go/go.mod") -Encoding UTF8
    @("[package]", "name = `"dependencies_smoke`"", "version = `"0.1.0`"", "edition = `"2021`"", "[dependencies]") |
        Set-Content -Path (Join-Path $dependencyDir "rust/Cargo.toml") -Encoding UTF8
    '{"require":{"monolog/monolog":"^3.0"},"require-dev":{"phpunit/phpunit":"^10.0"}}' |
        Set-Content -Path (Join-Path $dependencyDir "php/composer.json") -Encoding UTF8
    '<project><dependencies><dependency><groupId>org.slf4j</groupId><artifactId>slf4j-api</artifactId><version>2.0.13</version></dependency></dependencies></project>' |
        Set-Content -Path (Join-Path $dependencyDir "maven/pom.xml") -Encoding UTF8
    @('plugins { id "java" }', 'dependencies {', '  implementation "com.google.guava:guava:33.0.0-jre"', '  testImplementation "junit:junit:4.13.2"', '}') |
        Set-Content -Path (Join-Path $dependencyDir "gradle/build.gradle") -Encoding UTF8
    '<Project Sdk="Microsoft.NET.Sdk"><ItemGroup><PackageReference Include="Newtonsoft.Json" Version="13.0.3" /></ItemGroup></Project>' |
        Set-Content -Path (Join-Path $dependencyDir "dotnet/App.csproj") -Encoding UTF8
    '<packages><package id="NUnit" version="3.14.0" /></packages>' |
        Set-Content -Path (Join-Path $dependencyDir "nuget-config/packages.config") -Encoding UTF8
    '<Project><ItemGroup><PackageVersion Include="Serilog" Version="3.1.1" /></ItemGroup></Project>' |
        Set-Content -Path (Join-Path $dependencyDir "nuget-props/Directory.Packages.props") -Encoding UTF8
    @("gem 'rails', '~> 7.1'", "gem 'rspec', group: :test") |
        Set-Content -Path (Join-Path $dependencyDir "ruby/Gemfile") -Encoding UTF8
    @("name: dependencies_smoke", "dependencies:", "  http: ^1.2.0", "dev_dependencies:", "  test: ^1.25.0", "flutter:", "  uses-material-design: true") |
        Set-Content -Path (Join-Path $dependencyDir "dart/pubspec.yaml") -Encoding UTF8
    @("let package = Package(", '  dependencies: [.package(url: "https://github.com/apple/swift-nio.git", from: "2.0.0")]', ")") |
        Set-Content -Path (Join-Path $dependencyDir "swift/Package.swift") -Encoding UTF8
    @("defp deps do", "  [", '    {:plug, "~> 1.0"}', "  ]", "end") |
        Set-Content -Path (Join-Path $dependencyDir "elixir/mix.exs") -Encoding UTF8
    @('package = "dependencies_smoke"', 'version = "1.0-1"', "dependencies = {", '  "lua >= 5.4",', '  "luasocket >= 3.0"', "}") |
        Set-Content -Path (Join-Path $dependencyDir "lua/dependencies_smoke-1.0-1.rockspec") -Encoding UTF8
    @("Package: dependenciesSmoke", "Imports: jsonlite (>= 1.8), httr", "Suggests: testthat") |
        Set-Content -Path (Join-Path $dependencyDir "r/DESCRIPTION") -Encoding UTF8
    @("[deps]", 'JSON = "682c06a0-de6a-54ab-a142-c8b1cf79cde6"', "[compat]", 'JSON = "0.21"') |
        Set-Content -Path (Join-Path $dependencyDir "julia/Project.toml") -Encoding UTF8
    @("name: dependencies-smoke", "version: 0.1.0.0", "build-depends: base >=4.14, text") |
        Set-Content -Path (Join-Path $dependencyDir "cabal/dependencies-smoke.cabal") -Encoding UTF8
    @("resolver: lts-22.0", "extra-deps:", "  - text-2.0.2") |
        Set-Content -Path (Join-Path $dependencyDir "stack/stack.yaml") -Encoding UTF8
    @("requires 'Mojolicious', '9.0';", "on 'test' => sub { requires 'Test::More', '1.0'; };") |
        Set-Content -Path (Join-Path $dependencyDir "perl/cpanfile") -Encoding UTF8
    '{"name":"dependencies-smoke","version-string":"0.1.0","dependencies":["fmt",{"name":"zlib","version>=":"1.2.13"}]}' |
        Set-Content -Path (Join-Path $dependencyDir "vcpkg/vcpkg.json") -Encoding UTF8
    @("[requires]", "zlib/1.2.13") |
        Set-Content -Path (Join-Path $dependencyDir "conan-txt/conanfile.txt") -Encoding UTF8
    'requires = "zlib/1.2.13"' |
        Set-Content -Path (Join-Path $dependencyDir "conan-py/conanfile.py") -Encoding UTF8

    Invoke-Go @("run", $Pkg, "system") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "toolchain") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "path") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "container") | Out-Null
    Invoke-Go @("run", $Pkg, "snapshot") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "dependencies", $nodeDir) | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "dependencies", $goDir) | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "dependencies", $dependencyDir) | Out-Null

    $system = Invoke-GoOutput @("run", $Pkg, "system", "--json")
    $toolchain = Invoke-GoOutput @("run", $Pkg, "scan", "toolchain", "--json")
    $path = Invoke-GoOutput @("run", $Pkg, "scan", "path", "--json")
    $container = Invoke-GoOutput @("run", $Pkg, "scan", "container", "--json")
    $dependencies = Invoke-GoOutput @("run", $Pkg, "scan", "dependencies", "--json", $dependencyDir)
    $snapshot = Invoke-GoOutput @("run", $Pkg, "snapshot", "--json")
    $diagnose = Invoke-GoOutput @("run", $Pkg, "diagnose", "--json")
    $explain = Invoke-GoOutput @("run", $Pkg, "explain", "--json", (Join-Path $SmokeDir "envdoctor.log"))
    $recommend = Invoke-GoOutput @("run", $Pkg, "recommend", "--json")
    Invoke-Go @("run", $Pkg, "service", "list") | Out-Null
    Invoke-Go @("run", $Pkg, "service", "status", "envdoctor-smoke-missing") | Out-Null
    $service = Invoke-GoOutput @("run", $Pkg, "service", "diagnose", "--json", "envdoctor-smoke-missing")
    $versionScan = Invoke-GoOutput @("run", $Pkg, "version", "scan", "--json", $versionDir)
    $versionPlan = Invoke-GoOutput @("run", $Pkg, "version", "plan", "--json", $versionDir)
    $installPlan = Invoke-GoOutput @("run", $Pkg, "install", "plan", "python", "--json")
    $fixPlan = Invoke-GoOutput @("run", $Pkg, "fix", "plan", "--json", $versionDir)
    $bootstrapPlan = Invoke-GoOutput @("run", $Pkg, "bootstrap", "plan", "--json", $bootstrapDir)
    Invoke-GoOutput @("run", $Pkg, "ui", "--script", "diagnose,version:$versionDir,fix:$versionDir,bootstrap:$bootstrapDir,exit") |
        Set-Content -Path (Join-Path $SmokeDir "ui.txt") -Encoding UTF8
    $uiOutput = Get-Content -Raw -Path (Join-Path $SmokeDir "ui.txt")
    if ($uiOutput -notmatch "Envdoctor Dashboard" -or $uiOutput -notmatch "Menu") {
        throw "UI smoke output is missing dashboard or menu sections"
    }
    if ($uiOutput -match "(?i)executed|installed|restarted|fixed") {
        throw "UI smoke output contains mutating action wording"
    }
    $system | ConvertFrom-Json | Out-Null
    $toolchain | ConvertFrom-Json | Out-Null
    $path | ConvertFrom-Json | Out-Null
    $container | ConvertFrom-Json | Out-Null
    $dependencies | ConvertFrom-Json | Out-Null
    $snapshot | ConvertFrom-Json | Out-Null
    $diagnose | ConvertFrom-Json | Out-Null
    $explain | ConvertFrom-Json | Out-Null
    $recommend | ConvertFrom-Json | Out-Null
    $service | ConvertFrom-Json | Out-Null
    $versionScan | ConvertFrom-Json | Out-Null
    $versionPlan | ConvertFrom-Json | Out-Null
    $installPlan | ConvertFrom-Json | Out-Null
    $fixPlan | ConvertFrom-Json | Out-Null
    $bootstrapPlan | ConvertFrom-Json | Out-Null

    Invoke-GoOutput @("run", $Pkg, "dockerize", $nodeDir) |
        Set-Content -Path (Join-Path $SmokeDir "Dockerfile.preview") -Encoding UTF8
    Invoke-Go @("run", $Pkg, "dockerize", "--force", "--output", (Join-Path $SmokeDir "Dockerfile.generated"), $goDir) | Out-Null

    Write-Host "Smoke checks passed"
}

function Invoke-Clean {
    foreach ($path in @($BinDir, $DistDir, $CacheDir)) {
        if ($path -eq "." -or $path -eq "") {
            throw "Refusing to clean with unsafe directory setting: '$path'"
        }
        $fullPath = Join-Path $Root $path
        if (Test-Path $fullPath) {
            Remove-Item -Recurse -Force $fullPath
        }
    }
    Write-Host "Removed $BinDir, $DistDir, and $CacheDir"
}

function Show-Help {
    Write-Host "Environment Doctor Agent Windows targets:"
    Write-Host "  pwsh ./scripts/windows.ps1 check"
    Write-Host "  pwsh ./scripts/windows.ps1 build"
    Write-Host "  pwsh ./scripts/windows.ps1 prod"
    Write-Host "  pwsh ./scripts/windows.ps1 release"
    Write-Host "  pwsh ./scripts/windows.ps1 smoke"
    Write-Host "  pwsh ./scripts/windows.ps1 clean"
    Write-Host "  pwsh ./scripts/windows.ps1 dev -Args `"diagnose --json`""
}

Set-Location $Root

switch ($Target) {
    "fmt" { Invoke-Fmt }
    "fmt-check" { Invoke-FmtCheck }
    "vet" { Invoke-Vet }
    "test" { Invoke-Test }
    "check" { Invoke-Check }
    "build" { Invoke-Build }
    "dev" { Invoke-Dev }
    "prod" { Invoke-Prod }
    "release" { Invoke-Release }
    "smoke" { Invoke-Smoke }
    "clean" { Invoke-Clean }
    default { Show-Help }
}
