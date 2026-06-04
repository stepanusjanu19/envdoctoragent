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
    New-Item -ItemType Directory -Force -Path $nodeDir, $goDir | Out-Null

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

    Invoke-Go @("run", $Pkg, "system") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "toolchain") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "path") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "container") | Out-Null
    Invoke-Go @("run", $Pkg, "snapshot") | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "dependencies", $nodeDir) | Out-Null
    Invoke-Go @("run", $Pkg, "scan", "dependencies", $goDir) | Out-Null

    $diagnose = Invoke-GoOutput @("run", $Pkg, "diagnose", "--json")
    $explain = Invoke-GoOutput @("run", $Pkg, "explain", "--json", (Join-Path $SmokeDir "envdoctor.log"))
    $recommend = Invoke-GoOutput @("run", $Pkg, "recommend", "--json")
    $diagnose | ConvertFrom-Json | Out-Null
    $explain | ConvertFrom-Json | Out-Null
    $recommend | ConvertFrom-Json | Out-Null

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
