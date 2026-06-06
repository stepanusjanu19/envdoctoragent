param(
    [ValidateSet("help", "fmt", "fmt-check", "vet", "test", "check", "build", "dev", "prod", "release-binaries", "release-check", "release", "smoke", "clean")]
    [string]$Target = "help",
    [string]$Args = "help",
    [string]$Binary = "envdoctor",
    [string]$BinDir = "bin",
    [string]$DistDir = "dist",
    [string]$CacheDir = ".cache",
    [string]$GoReleaserVersion = "v2.16.0",
    [string]$ReleaseRepository = "stepanusjanu19/envdoctoragent"
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$Pkg = "./cmd/envdoctor"
$GoCache = Join-Path $Root (Join-Path $CacheDir "go-build")
$GoModCache = Join-Path $Root (Join-Path $CacheDir "go-mod")
$SmokeDir = Join-Path $Root (Join-Path $CacheDir "smoke")
$ToolsDir = Join-Path $Root (Join-Path $CacheDir "tools")
$GoReleaser = Join-Path $ToolsDir "goreleaser.exe"

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

function Join-OutputText {
    param($Output)

    if ($null -eq $Output) {
        return ""
    }
    if ($Output -is [array]) {
        return ($Output -join [Environment]::NewLine)
    }
    return [string]$Output
}

function Test-OutputContains {
    param(
        $Output,
        [string]$Pattern
    )

    return (Join-OutputText $Output) -match $Pattern
}

function Test-OutputNotContains {
    param(
        $Output,
        [string]$Pattern
    )

    return -not (Test-OutputContains $Output $Pattern)
}

function Normalize-PathText {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $trimChars = [char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
    return ([System.IO.Path]::GetFullPath($Path)).TrimEnd($trimChars)
}

function Assert-ScaffoldPlan {
    param(
        $Output,
        [ValidateSet("path", "target")]
        [string]$Mode
    )

    $plan = $Output | ConvertFrom-Json
    if ($plan.source -ne "official") {
        throw "$($plan.template) scaffold plan is not official"
    }
    $executionBaseDir = $plan.execution_base_dir
    if ([string]::IsNullOrWhiteSpace($executionBaseDir)) {
        $executionBaseDir = $plan.directory
    }
    $parent = Split-Path -Parent $plan.directory
    $commandCount = 0
    $hasMkdir = $false
    foreach ($action in @($plan.actions)) {
        if ($action.type -eq "write_file" -or $action.type -eq "manual") {
            throw "$($plan.template) exposed $($action.type) action"
        }
        if ($action.type -eq "mkdir") {
            $hasMkdir = $true
        }
        if ($action.type -ne "command") {
            continue
        }
        $commandCount++
        if ([string]::IsNullOrWhiteSpace($action.working_dir)) {
            throw "$($plan.template) command has empty working_dir"
        }
        if ($Mode -eq "path") {
            if ((Normalize-PathText $action.working_dir) -ne (Normalize-PathText $parent)) {
                throw "$($plan.template) command working_dir was not the target parent"
            }
            if (@($action.args) -contains ".") {
                throw "$($plan.template) path-arg generator used . destination"
            }
        } elseif ((Normalize-PathText $action.working_dir) -ne (Normalize-PathText $plan.directory)) {
            throw "$($plan.template) command working_dir was not the target directory"
        }
    }
    if ($commandCount -eq 0) {
        throw "$($plan.template) has no command action"
    }
    if ($Mode -eq "path") {
        if ($hasMkdir) {
            throw "$($plan.template) path-arg generator should not pre-create target"
        }
        if ((Normalize-PathText $executionBaseDir) -ne (Normalize-PathText $parent)) {
            throw "$($plan.template) execution_base_dir was not the target parent"
        }
    } elseif ((Normalize-PathText $executionBaseDir) -ne (Normalize-PathText $plan.directory)) {
        throw "$($plan.template) execution_base_dir was not the target directory"
    }
    if ($plan.template -eq "laravel") {
        $command = @($plan.actions | Where-Object { $_.type -eq "command" })[0]
        $target = Split-Path -Leaf $plan.directory
        $args = @($command.args)
        if ($command.command -ne "composer" -or $args.Count -ne 3 -or $args[0] -ne "create-project" -or $args[1] -ne "laravel/laravel" -or $args[2] -ne $target) {
            throw "Laravel scaffold args were not composer create-project laravel/laravel <target>"
        }
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

function Get-Version {
    $tag = (& git describe --tags --abbrev=0 2>$null)
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($tag)) {
        $version = $tag.Trim().TrimStart("v")
        if ($version -match "^\d+\.\d+\.\d+") {
            return $version
        }
    }
    return "0.0.0-dev"
}

function Get-Commit {
    $commit = (& git rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($commit)) {
        return $commit.Trim()
    }
    return "none"
}

function Get-BuildDate {
    (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
}

function Get-LdFlags {
    $version = Get-Version
    $commit = Get-Commit
    $date = Get-BuildDate
    "-s -w -X main.version=$version -X main.commit=$commit -X main.date=$date"
}

function Install-GoReleaser {
    if (Test-Path $GoReleaser) {
        return
    }
    New-Item -ItemType Directory -Force -Path $ToolsDir | Out-Null
    Write-Host "Installing GoReleaser $GoReleaserVersion into $ToolsDir"

    $previousGoBin = $env:GOBIN
    $previousGoCache = $env:GOCACHE
    $previousGoModCache = $env:GOMODCACHE
    $env:GOBIN = $ToolsDir
    $env:GOCACHE = $GoCache
    $env:GOMODCACHE = $GoModCache
    try {
        & go install "github.com/goreleaser/goreleaser/v2@$GoReleaserVersion"
        if ($LASTEXITCODE -ne 0) {
            throw "go install goreleaser failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:GOBIN = $previousGoBin
        $env:GOCACHE = $previousGoCache
        $env:GOMODCACHE = $previousGoModCache
    }
}

function Invoke-Build {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $BinDir) | Out-Null
    $out = Get-LocalBinaryPath
    Invoke-Go @("build", "-ldflags=$(Get-LdFlags)", "-o", $out, $Pkg)
    Write-Host "Built $out"
}

function Invoke-Prod {
    New-Item -ItemType Directory -Force -Path (Join-Path $Root $BinDir) | Out-Null
    $out = Get-LocalBinaryPath
    Invoke-Go @("build", "-trimpath", "-ldflags=$(Get-LdFlags)", "-o", $out, $Pkg)
    Write-Host "Built production binary $out"
}

function Invoke-ReleaseBinaries {
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
            Invoke-Go @("build", "-trimpath", "-ldflags=$(Get-LdFlags)", "-o", $out, $Pkg)
        }
    } finally {
        $env:GOOS = $previousGoos
        $env:GOARCH = $previousGoarch
        $env:CGO_ENABLED = $previousCgo
    }
}

function Invoke-ReleaseCheck {
    Install-GoReleaser
    & $GoReleaser check
    if ($LASTEXITCODE -ne 0) {
        throw "goreleaser check failed with exit code $LASTEXITCODE"
    }
}

function Invoke-Release {
    Install-GoReleaser
    $previousVersion = $env:VERSION
    $previousGoCache = $env:GOCACHE
    $previousGoModCache = $env:GOMODCACHE
    try {
        $env:VERSION = Get-Version
        $env:GOCACHE = $GoCache
        $env:GOMODCACHE = $GoModCache
        & $GoReleaser release --snapshot --clean
        if ($LASTEXITCODE -ne 0) {
            throw "goreleaser release failed with exit code $LASTEXITCODE"
        }
    } finally {
        $env:VERSION = $previousVersion
        $env:GOCACHE = $previousGoCache
        $env:GOMODCACHE = $previousGoModCache
    }
    Invoke-Go @(
        "run", "./cmd/releasemanifests",
        "--dist", (Join-Path $Root $DistDir),
        "--version", (Get-Version),
        "--repository", $ReleaseRepository
    )
    Write-Host "Release artifacts written to $DistDir"
}

function Invoke-Dev {
    Invoke-Go (@("run", $Pkg) + ($Args -split " "))
}

function Invoke-Smoke {
    $nodeDir = Join-Path $SmokeDir "node"
    $goDir = Join-Path $SmokeDir "go"
    $versionDir = Join-Path $SmokeDir "version"
    $bootstrapDir = Join-Path $SmokeDir "bootstrap"
    $projectEmptyDir = Join-Path $SmokeDir "project-empty"
    $projectNodeDir = Join-Path $SmokeDir "project-node"
    $projectYesDir = Join-Path $SmokeDir "project-yes"
    $scaffoldReactDir = Join-Path $SmokeDir "scaffold-react"
    $scaffoldLaravelDir = Join-Path $SmokeDir "scaffold-laravel"
    $scaffoldGoWebDir = Join-Path $SmokeDir "scaffold-go-web"
    $scaffoldGoDir = Join-Path $SmokeDir "scaffold-go"
    $scaffoldPythonDir = Join-Path $SmokeDir "scaffold-python"
    $scaffoldConflictDir = Join-Path $SmokeDir "scaffold-conflict"
    $agentScaffoldPlanDir = Join-Path $SmokeDir "agent-scaffold-plan"
    $agentScaffoldYesDir = Join-Path $SmokeDir "agent-scaffold-yes"
    $agentProdBlockDir = Join-Path $SmokeDir "agent-prod-block"
    $automationScaffoldPlanDir = Join-Path $SmokeDir "automation-scaffold-plan"
    $automationProdBlockDir = Join-Path $SmokeDir "automation-prod-block"
    $dependencyDir = Join-Path $SmokeDir "dependencies"
    foreach ($path in @($projectEmptyDir, $projectNodeDir, $projectYesDir, $scaffoldReactDir, $scaffoldLaravelDir, $scaffoldGoWebDir, $scaffoldGoDir, $scaffoldPythonDir, $scaffoldConflictDir, $agentScaffoldPlanDir, $agentScaffoldYesDir, $agentProdBlockDir, $automationScaffoldPlanDir, $automationProdBlockDir)) {
        if (Test-Path $path) {
            Remove-Item -Recurse -Force $path
        }
    }
    New-Item -ItemType Directory -Force -Path $nodeDir, $goDir, $versionDir, $bootstrapDir, $projectEmptyDir, $projectNodeDir, $projectYesDir, $scaffoldReactDir, $scaffoldGoDir, $scaffoldConflictDir | Out-Null
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
    '{"dependencies":{"express":"^5.0.0"}}' |
        Set-Content -Path (Join-Path $projectNodeDir "package.json") -Encoding UTF8
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
    $aboutText = Invoke-GoOutput @("run", $Pkg, "about")
    $diagnoseText = Invoke-GoOutput @("run", $Pkg, "diagnose")
    $agentPlanText = Invoke-GoOutput @("run", $Pkg, "agent", "plan", "--goal", "diagnose")
    $automationPlanText = Invoke-GoOutput @("run", $Pkg, "automation", "plan", "--goal", "diagnose")
    $projectTemplatesText = Invoke-GoOutput @("run", $Pkg, "project", "templates")
    $diagnosePlain = Invoke-GoOutput @("run", $Pkg, "--plain", "diagnose")
    if (-not (Test-OutputContains $aboutText "Envdoctor")) {
        throw "About output did not contain Envdoctor title"
    }
    if (-not (Test-OutputContains $diagnoseText "Progress:")) {
        throw "Diagnose output did not contain progress"
    }
    if (-not (Test-OutputContains $agentPlanText "Next steps")) {
        throw "Agent plan output did not contain next steps"
    }
    if (-not (Test-OutputContains $automationPlanText "Policy Summary")) {
        throw "Automation plan output did not contain policy summary"
    }
    if (-not (Test-OutputContains $projectTemplatesText "Project Templates")) {
        throw "Project templates output did not contain friendly title"
    }
    $diagnosePlainText = Join-OutputText $diagnosePlain
    if ((Test-OutputContains $diagnosePlainText "Progress:") -or $diagnosePlainText.Contains([string][char]27)) {
        throw "Plain output contained progress or ANSI escape codes"
    }

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
    $servicePlan = Invoke-GoOutput @("run", $Pkg, "service", "plan", "restart", "envdoctor-smoke-missing", "--json")
    $serviceApplyAudit = Join-Path $SmokeDir "service-apply-audit.jsonl"
    $serviceApplyProdAudit = Join-Path $SmokeDir "service-apply-prod-audit.jsonl"
    $serviceApply = Invoke-GoOutput @("run", $Pkg, "service", "apply", "restart", "envdoctor-smoke-missing", "--dry-run", "--json", "--audit-log", $serviceApplyAudit)
    $serviceApplyProd = Invoke-GoOutput @("run", $Pkg, "service", "apply", "restart", "envdoctor-smoke-missing", "--yes", "--json", "--profile", "production", "--audit-log", $serviceApplyProdAudit)
    if (($serviceApplyProd | ConvertFrom-Json).profile -ne "production") {
        throw "Production service apply did not report production profile"
    }
    if (-not (($serviceApplyProd | ConvertFrom-Json).policy_decisions | Where-Object { $_.allowed -eq $false })) {
        throw "Production service apply did not report a policy block"
    }
    $versionScan = Invoke-GoOutput @("run", $Pkg, "version", "scan", "--json", $versionDir)
    $versionPlan = Invoke-GoOutput @("run", $Pkg, "version", "plan", "--json", $versionDir)
    $installPlan = Invoke-GoOutput @("run", $Pkg, "install", "plan", "python", "--json")
    $fixPlan = Invoke-GoOutput @("run", $Pkg, "fix", "plan", "--json", $versionDir)
    $bootstrapPlan = Invoke-GoOutput @("run", $Pkg, "bootstrap", "plan", "--json", $bootstrapDir)
    $projectScan = Invoke-GoOutput @("run", $Pkg, "project", "scan", "--json", $projectNodeDir)
    $projectTemplates = Invoke-GoOutput @("run", $Pkg, "project", "templates", "--json")
    $templateSources = @($projectTemplates | ConvertFrom-Json | ForEach-Object { $_.source })
    if ($templateSources -contains "internal" -or $templateSources -contains "manual") {
        throw "Project templates exposed internal/manual scaffold source"
    }
    $projectInitPlan = Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "node", "--json", $projectEmptyDir)
    $scaffoldReactPlan = Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "react-vite", "--json", "--create-dir", $scaffoldReactDir)
    if (($scaffoldReactPlan | ConvertFrom-Json).source -ne "official") {
        throw "React Vite scaffold plan is not official"
    }
    $scaffoldLaravelPlan = Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "laravel", "--json", "--create-dir", $scaffoldLaravelDir)
    if (($scaffoldLaravelPlan | ConvertFrom-Json).source -ne "official") {
        throw "Laravel scaffold plan is not official"
    }
    Assert-ScaffoldPlan $scaffoldLaravelPlan "path"
    $scaffoldLaravelApplyDir = Join-Path $SmokeDir "scaffold-laravel-apply"
    $beforeLaravelApply = 0
    if (Test-Path $scaffoldLaravelApplyDir) {
        $beforeLaravelApply = (Get-ChildItem -Force -Path $scaffoldLaravelApplyDir | Measure-Object).Count
    }
    $scaffoldLaravelApply = Invoke-GoOutput @("run", $Pkg, "project", "init", "apply", "laravel", "--dry-run", "--json", "--create-dir", $scaffoldLaravelApplyDir)
    $afterLaravelApply = 0
    if (Test-Path $scaffoldLaravelApplyDir) {
        $afterLaravelApply = (Get-ChildItem -Force -Path $scaffoldLaravelApplyDir | Measure-Object).Count
    }
    if ($beforeLaravelApply -ne $afterLaravelApply) {
        throw "Laravel scaffold dry-run created files"
    }
    foreach ($template in @("react-vite", "vue-vite", "next", "sveltekit", "nestjs", "laravel", "dotnet", "dotnet-console", "dotnet-webapi", "dart", "dart-console", "flutter", "flutter-app", "elixir")) {
        $templateDir = Join-Path $SmokeDir "scaffold-matrix-$template"
        $templatePlan = Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", $template, "--json", "--create-dir", $templateDir)
        Assert-ScaffoldPlan $templatePlan "path"
    }
    foreach ($template in @("node", "go", "go-module", "rust", "rust-cli", "rust-lib", "swift")) {
        $templateDir = Join-Path $SmokeDir "scaffold-matrix-$template"
        $templatePlan = Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", $template, "--json", "--create-dir", $templateDir)
        Assert-ScaffoldPlan $templatePlan "target"
    }
    try {
        Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "go-web", "--json", "--create-dir", $scaffoldGoWebDir) | Out-Null
        throw "Project scaffold unexpectedly exposed go-web without a safe official generator"
    } catch {
        if ($_.Exception.Message -eq "Project scaffold unexpectedly exposed go-web without a safe official generator") {
            throw
        }
    }
    $projectInitApplyAudit = Join-Path $SmokeDir "project-init-apply-audit.jsonl"
    $projectDepsApplyAudit = Join-Path $SmokeDir "project-deps-apply-audit.jsonl"
    $projectInitYesAudit = Join-Path $SmokeDir "project-init-yes-audit.jsonl"
    $beforeScaffoldCount = (Get-ChildItem -Force -Path $scaffoldGoDir | Measure-Object).Count
    $projectInitApply = Invoke-GoOutput @("run", $Pkg, "project", "init", "apply", "react-vite", "--dry-run", "--json", "--audit-log", $projectInitApplyAudit, $scaffoldGoDir)
    $afterScaffoldCount = (Get-ChildItem -Force -Path $scaffoldGoDir | Measure-Object).Count
    if ($beforeScaffoldCount -ne $afterScaffoldCount) {
        throw "Project scaffold dry-run created files"
    }
    "module example.com/conflict" | Set-Content -Path (Join-Path $scaffoldConflictDir "go.mod") -Encoding UTF8
    try {
        Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "go", "--allow-non-empty", $scaffoldConflictDir) | Out-Null
        throw "Project scaffold unexpectedly allowed Go init over an existing module"
    } catch {
        if ($_.Exception.Message -eq "Project scaffold unexpectedly allowed Go init over an existing module") {
            throw
        }
    }
    try {
        Invoke-GoOutput @("run", $Pkg, "project", "init", "plan", "../x", "--json", $scaffoldReactDir) | Out-Null
        throw "Project scaffold unexpectedly allowed path traversal template id"
    } catch {
        if ($_.Exception.Message -eq "Project scaffold unexpectedly allowed path traversal template id") {
            throw
        }
    }
    $projectDepsSync = Invoke-GoOutput @("run", $Pkg, "project", "deps", "plan", "sync", "--json", $projectNodeDir)
    $projectDepsSyncGo = Invoke-GoOutput @("run", $Pkg, "project", "deps", "plan", "sync", "--ecosystem", "go", "--json", $goDir)
    $projectDepsInstall = Invoke-GoOutput @("run", $Pkg, "project", "deps", "plan", "install", "lodash", "--ecosystem", "node", "--json", $projectNodeDir)
    $projectDepsUpdate = Invoke-GoOutput @("run", $Pkg, "project", "deps", "plan", "update", "lodash", "--ecosystem", "node", "--json", $projectNodeDir)
    $projectDepsRemove = Invoke-GoOutput @("run", $Pkg, "project", "deps", "plan", "remove", "lodash", "--ecosystem", "node", "--json", $projectNodeDir)
    $packageJsonPath = Join-Path $projectNodeDir "package.json"
    $beforeHash = (Get-FileHash $packageJsonPath).Hash
    $projectDepsApply = Invoke-GoOutput @("run", $Pkg, "project", "deps", "apply", "sync", "--dry-run", "--json", "--audit-log", $projectDepsApplyAudit, $projectNodeDir)
    $afterHash = (Get-FileHash $packageJsonPath).Hash
    if ($beforeHash -ne $afterHash) {
        throw "Project deps dry-run mutated package.json"
    }
    $projectInitYes = Invoke-GoOutput @("run", $Pkg, "project", "init", "apply", "go", "--yes", "--json", "--audit-log", $projectInitYesAudit, $projectYesDir)
    if (-not (Test-Path (Join-Path $projectYesDir "go.mod"))) {
        throw "Go scaffold go.mod was not created"
    }
    if (-not (($projectInitYes | ConvertFrom-Json).project_snapshot_file)) {
        throw "Project init --yes did not report a project snapshot"
    }
    try {
        Invoke-GoOutput @("run", $Pkg, "project", "init", "apply", "go", "--dry-run", $projectNodeDir) | Out-Null
        throw "Project init apply unexpectedly allowed non-empty directory"
    } catch {
        if ($_.Exception.Message -eq "Project init apply unexpectedly allowed non-empty directory") {
            throw
        }
    }
    try {
        Invoke-GoOutput @("run", $Pkg, "project", "deps", "apply", "remove", "--dry-run") | Out-Null
        throw "Project deps remove unexpectedly allowed missing package"
    } catch {
        if ($_.Exception.Message -eq "Project deps remove unexpectedly allowed missing package") {
            throw
        }
    }
    $fixApplyAudit = Join-Path $SmokeDir "fix-apply-audit.jsonl"
    $fixApplyDevAudit = Join-Path $SmokeDir "fix-apply-dev-audit.jsonl"
    $fixApplyProdAudit = Join-Path $SmokeDir "fix-apply-prod-audit.jsonl"
    $installApplyAudit = Join-Path $SmokeDir "install-apply-audit.jsonl"
    $installApplyProdAudit = Join-Path $SmokeDir "install-apply-prod-audit.jsonl"
    $versionApplyAudit = Join-Path $SmokeDir "version-apply-audit.jsonl"
    $bootstrapApplyAudit = Join-Path $SmokeDir "bootstrap-apply-audit.jsonl"
    $fixApply = Invoke-GoOutput @("run", $Pkg, "fix", "apply", "--dry-run", "--json", "--audit-log", $fixApplyAudit, $versionDir)
    $fixApplyDev = Invoke-GoOutput @("run", $Pkg, "fix", "apply", "--dry-run", "--json", "--profile", "development", "--audit-log", $fixApplyDevAudit, $versionDir)
    $fixApplyProd = Invoke-GoOutput @("run", $Pkg, "fix", "apply", "--dry-run", "--json", "--profile", "production", "--audit-log", $fixApplyProdAudit, $versionDir)
    $installApply = Invoke-GoOutput @("run", $Pkg, "install", "apply", "python", "--dry-run", "--json", "--audit-log", $installApplyAudit)
    $installApplyProd = Invoke-GoOutput @("run", $Pkg, "install", "apply", "python", "--dry-run", "--json", "--profile", "production", "--audit-log", $installApplyProdAudit)
    $versionApply = Invoke-GoOutput @("run", $Pkg, "version", "apply", "--dry-run", "--json", "--audit-log", $versionApplyAudit, $versionDir)
    $bootstrapApply = Invoke-GoOutput @("run", $Pkg, "bootstrap", "apply", "--dry-run", "--json", "--audit-log", $bootstrapApplyAudit, $bootstrapDir)
    if (($fixApplyProd | ConvertFrom-Json).profile -ne "production") {
        throw "Production fix apply did not report production profile"
    }
    if (($installApplyProd | ConvertFrom-Json).profile -ne "production") {
        throw "Production install apply did not report production profile"
    }
    $agentScaffoldYesAudit = Join-Path $SmokeDir "agent-scaffold-yes-audit.jsonl"
    $agentProdBlockAudit = Join-Path $SmokeDir "agent-prod-block-audit.jsonl"
    $automationProdBlockAudit = Join-Path $SmokeDir "automation-prod-block-audit.jsonl"
    $agentPlanDiagnose = Invoke-GoOutput @("run", $Pkg, "agent", "plan", "--json", "--goal", "diagnose", "--profile", "development")
    $agentPlanOnboard = Invoke-GoOutput @("run", $Pkg, "agent", "plan", "--json", "--goal", "onboard", $projectNodeDir)
    $agentPlanScaffold = Invoke-GoOutput @("run", $Pkg, "agent", "plan", "--json", "--goal", "scaffold", "--template", "react-vite", "--create-dir", $agentScaffoldPlanDir)
    $agentRunRepair = Invoke-GoOutput @("run", $Pkg, "agent", "run", "--dry-run", "--json", "--goal", "repair", $projectNodeDir)
    $agentRunScaffold = Invoke-GoOutput @("run", $Pkg, "agent", "run", "--yes", "--json", "--goal", "scaffold", "--template", "go", "--create-dir", "--audit-log", $agentScaffoldYesAudit, $agentScaffoldYesDir)
    if (-not (Test-Path (Join-Path $agentScaffoldYesDir "go.mod"))) {
        throw "Agent scaffold go.mod was not created"
    }
    if (-not (($agentRunScaffold | ConvertFrom-Json).execution.project_snapshot_file)) {
        throw "Agent scaffold --yes did not report a project snapshot"
    }
    $agentRunProdBlock = Invoke-GoOutput @("run", $Pkg, "agent", "run", "--yes", "--json", "--profile", "production", "--goal", "scaffold", "--template", "go", "--create-dir", "--audit-log", $agentProdBlockAudit, $agentProdBlockDir)
    if (Test-Path $agentProdBlockDir) {
        throw "Production agent scaffold created a project directory despite policy block"
    }
    $agentProdReport = $agentRunProdBlock | ConvertFrom-Json
    if (-not ($agentProdReport.execution.policy_decisions | Where-Object { $_.allowed -eq $false -and $_.reason -eq "production profile blocks mutating apply actions" })) {
        throw "Production agent scaffold did not report mutating policy block"
    }
    $automationPlanDiagnose = Invoke-GoOutput @("run", $Pkg, "automation", "plan", "--json", "--goal", "diagnose", "--profile", "development")
    $automationPlanMaintain = Invoke-GoOutput @("run", $Pkg, "automation", "plan", "--json", "--goal", "maintain", $projectNodeDir)
    $automationMaintainReport = $automationPlanMaintain | ConvertFrom-Json
    if ($automationMaintainReport.goal -ne "maintain" -or -not (@($automationMaintainReport.selected_goals) -contains "repair")) {
        throw "Automation maintain plan did not include repair goal"
    }
    $automationPlanScaffold = Invoke-GoOutput @("run", $Pkg, "automation", "plan", "--json", "--goal", "scaffold", "--template", "react-vite", "--create-dir", $automationScaffoldPlanDir)
    $automationRunRepair = Invoke-GoOutput @("run", $Pkg, "automation", "run", "--dry-run", "--json", "--goal", "repair", $projectNodeDir)
    $automationRunProdBlock = Invoke-GoOutput @("run", $Pkg, "automation", "run", "--yes", "--json", "--profile", "production", "--goal", "scaffold", "--template", "go", "--create-dir", "--audit-log", $automationProdBlockAudit, $automationProdBlockDir)
    if (Test-Path $automationProdBlockDir) {
        throw "Production automation scaffold created a project directory despite policy block"
    }
    $automationProdReport = $automationRunProdBlock | ConvertFrom-Json
    if (-not $automationProdReport.policy_summary.production_mutation_blocked) {
        throw "Production automation scaffold did not report production mutation block"
    }
    if (-not ($automationProdReport.execution.policy_decisions | Where-Object { $_.allowed -eq $false })) {
        throw "Production automation scaffold did not report a policy block"
    }
    foreach ($auditPath in @($fixApplyAudit, $fixApplyDevAudit, $fixApplyProdAudit, $installApplyAudit, $installApplyProdAudit, $serviceApplyAudit, $serviceApplyProdAudit, $versionApplyAudit, $bootstrapApplyAudit)) {
        if (-not (Test-Path $auditPath) -or (Get-Item $auditPath).Length -eq 0) {
            throw "Apply audit log was not created: $auditPath"
        }
    }
    foreach ($auditPath in @($projectInitApplyAudit, $projectDepsApplyAudit, $projectInitYesAudit)) {
        if (-not (Test-Path $auditPath) -or (Get-Item $auditPath).Length -eq 0) {
            throw "Project audit log was not created: $auditPath"
        }
    }
    foreach ($auditPath in @($agentScaffoldYesAudit, $agentProdBlockAudit)) {
        if (-not (Test-Path $auditPath) -or (Get-Item $auditPath).Length -eq 0) {
            throw "Agent audit log was not created: $auditPath"
        }
    }
    if (-not (Test-Path $automationProdBlockAudit) -or (Get-Item $automationProdBlockAudit).Length -eq 0) {
        throw "Automation audit log was not created: $automationProdBlockAudit"
    }
    Invoke-GoOutput @("run", $Pkg, "ui", "--script", "diagnose,version:$versionDir,fix:$versionDir,bootstrap:$bootstrapDir,exit") |
        Set-Content -Path (Join-Path $SmokeDir "ui.txt") -Encoding UTF8
    $uiOutput = Get-Content -Raw -Path (Join-Path $SmokeDir "ui.txt")
    if ($uiOutput -notmatch "Envdoctor Dashboard" -or $uiOutput -notmatch "Menu" -or $uiOutput -notmatch "About" -or $uiOutput -notmatch "Progress:" -or $uiOutput -notmatch "Next steps") {
        throw "UI smoke output is missing dashboard, about, progress, next steps, or menu sections"
    }
    if ($uiOutput -match "(?i)executed|installed|restarted|fixed") {
        throw "UI smoke output contains mutating action wording"
    }
    Get-Content -Raw -Path (Join-Path $Root "integrations/vscode/package.json") | ConvertFrom-Json | Out-Null
    if (-not (Test-Path (Join-Path $Root "integrations/vscode/extension.js"))) {
        throw "VSCode wrapper extension.js was not found"
    }
    if (-not (Test-Path (Join-Path $Root "integrations/jetbrains/README.md"))) {
        throw "JetBrains integration README was not found"
    }
    $jetbrainsTools = Join-Path $Root "integrations/jetbrains/external-tools.xml"
    if (-not (Test-Path $jetbrainsTools) -or (Get-Content -Raw -Path $jetbrainsTools) -notmatch "Envdoctor Agent Plan JSON") {
        throw "JetBrains external tools template is missing"
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
    $servicePlan | ConvertFrom-Json | Out-Null
    $serviceApply | ConvertFrom-Json | Out-Null
    $serviceApplyProd | ConvertFrom-Json | Out-Null
    $versionScan | ConvertFrom-Json | Out-Null
    $versionPlan | ConvertFrom-Json | Out-Null
    $installPlan | ConvertFrom-Json | Out-Null
    $fixPlan | ConvertFrom-Json | Out-Null
    $bootstrapPlan | ConvertFrom-Json | Out-Null
    $projectScan | ConvertFrom-Json | Out-Null
    $projectTemplates | ConvertFrom-Json | Out-Null
    $projectInitPlan | ConvertFrom-Json | Out-Null
    $scaffoldReactPlan | ConvertFrom-Json | Out-Null
    $scaffoldLaravelPlan | ConvertFrom-Json | Out-Null
    $scaffoldLaravelApply | ConvertFrom-Json | Out-Null
    $projectInitApply | ConvertFrom-Json | Out-Null
    $projectDepsSync | ConvertFrom-Json | Out-Null
    $projectDepsSyncGo | ConvertFrom-Json | Out-Null
    $projectDepsInstall | ConvertFrom-Json | Out-Null
    $projectDepsUpdate | ConvertFrom-Json | Out-Null
    $projectDepsRemove | ConvertFrom-Json | Out-Null
    $projectDepsApply | ConvertFrom-Json | Out-Null
    $projectInitYes | ConvertFrom-Json | Out-Null
    $fixApply | ConvertFrom-Json | Out-Null
    $fixApplyDev | ConvertFrom-Json | Out-Null
    $fixApplyProd | ConvertFrom-Json | Out-Null
    $installApply | ConvertFrom-Json | Out-Null
    $installApplyProd | ConvertFrom-Json | Out-Null
    $versionApply | ConvertFrom-Json | Out-Null
    $bootstrapApply | ConvertFrom-Json | Out-Null
    $agentPlanDiagnose | ConvertFrom-Json | Out-Null
    $agentPlanOnboard | ConvertFrom-Json | Out-Null
    $agentPlanScaffold | ConvertFrom-Json | Out-Null
    $agentRunRepair | ConvertFrom-Json | Out-Null
    $agentRunScaffold | ConvertFrom-Json | Out-Null
    $agentRunProdBlock | ConvertFrom-Json | Out-Null
    $automationPlanDiagnose | ConvertFrom-Json | Out-Null
    $automationPlanMaintain | ConvertFrom-Json | Out-Null
    $automationPlanScaffold | ConvertFrom-Json | Out-Null
    $automationRunRepair | ConvertFrom-Json | Out-Null
    $automationRunProdBlock | ConvertFrom-Json | Out-Null

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
    Write-Host "  pwsh ./scripts/windows.ps1 release-binaries"
    Write-Host "  pwsh ./scripts/windows.ps1 release-check"
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
    "release-binaries" { Invoke-ReleaseBinaries }
    "release-check" { Invoke-ReleaseCheck }
    "release" { Invoke-Release }
    "smoke" { Invoke-Smoke }
    "clean" { Invoke-Clean }
    default { Show-Help }
}
