package installplan

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Action is a non-mutating install command suggestion.
type Action struct {
	Tool          string `json:"tool"`
	Platform      string `json:"platform"`
	Manager       string `json:"manager"`
	Package       string `json:"package"`
	Command       string `json:"command,omitempty"`
	ManualSteps   string `json:"manual_steps,omitempty"`
	Risk          string `json:"risk"`
	RequiresAdmin bool   `json:"requires_admin"`
	SafeToRun     bool   `json:"safe_to_run"`
	Status        string `json:"status"`
}

// Plan is the install advisor output.
type Plan struct {
	Tool    string `json:"tool"`
	Action  Action `json:"action"`
	Summary string `json:"summary"`
}

type managerDef struct {
	Name     string
	Command  string
	Platform string
}

var linuxManagers = []managerDef{
	{Name: "apt", Command: "apt-get", Platform: "linux"},
	{Name: "dnf", Command: "dnf", Platform: "linux"},
	{Name: "yum", Command: "yum", Platform: "linux"},
	{Name: "pacman", Command: "pacman", Platform: "linux"},
	{Name: "zypper", Command: "zypper", Platform: "linux"},
}

// Generate returns a plan-only installation recommendation for one tool.
func Generate(tool string) *Plan {
	tool = normalizeTool(tool)
	action := Action{
		Tool:          tool,
		Platform:      runtime.GOOS,
		Risk:          "medium",
		RequiresAdmin: true,
		SafeToRun:     false,
		Status:        "plan-only",
	}

	manager := detectPackageManager()
	if manager.Name == "" {
		action.Status = "not supported"
		action.ManualSteps = "No supported package manager was detected. Install the tool manually using your platform documentation."
		return &Plan{Tool: tool, Action: action, Summary: "No supported package manager detected"}
	}

	action.Manager = manager.Name
	action.Package = packageName(tool, manager.Name)
	if action.Package == "" {
		action.Status = "unknown tool"
		action.ManualSteps = fmt.Sprintf("No package mapping is defined for %q on %s.", tool, manager.Name)
		return &Plan{Tool: tool, Action: action, Summary: "Tool is not mapped for this package manager"}
	}

	action.Command = installCommand(manager.Name, action.Package)
	action.ManualSteps = "Review the command before running it. envdoctor does not execute install commands in this phase."
	action.RequiresAdmin = manager.Name != "brew"

	return &Plan{
		Tool:    tool,
		Action:  action,
		Summary: fmt.Sprintf("Generated install plan for %s using %s", tool, manager.Name),
	}
}

func detectPackageManager() managerDef {
	switch runtime.GOOS {
	case "darwin":
		if commandExists("brew") {
			return managerDef{Name: "brew", Command: "brew", Platform: runtime.GOOS}
		}
	case "linux":
		for _, manager := range linuxManagers {
			if commandExists(manager.Command) {
				return manager
			}
		}
	case "windows":
		if commandExists("winget") {
			return managerDef{Name: "winget", Command: "winget", Platform: runtime.GOOS}
		}
		if commandExists("choco") {
			return managerDef{Name: "choco", Command: "choco", Platform: runtime.GOOS}
		}
	}
	return managerDef{}
}

func installCommand(manager, pkg string) string {
	switch manager {
	case "apt":
		return fmt.Sprintf("sudo apt-get update && sudo apt-get install -y %s", pkg)
	case "dnf":
		return fmt.Sprintf("sudo dnf install -y %s", pkg)
	case "yum":
		return fmt.Sprintf("sudo yum install -y %s", pkg)
	case "pacman":
		return fmt.Sprintf("sudo pacman -S --needed %s", pkg)
	case "zypper":
		return fmt.Sprintf("sudo zypper install -y %s", pkg)
	case "brew":
		return fmt.Sprintf("brew install %s", pkg)
	case "winget":
		return fmt.Sprintf("winget install --id %s", pkg)
	case "choco":
		return fmt.Sprintf("choco install %s -y", pkg)
	default:
		return ""
	}
}

func packageName(tool, manager string) string {
	packages := map[string]map[string]string{
		"python": {
			"apt": "python3 python3-pip", "dnf": "python3 python3-pip", "yum": "python3 python3-pip",
			"pacman": "python python-pip", "zypper": "python3 python3-pip", "brew": "python",
			"winget": "Python.Python.3.12", "choco": "python",
		},
		"node": {
			"apt": "nodejs npm", "dnf": "nodejs npm", "yum": "nodejs npm", "pacman": "nodejs npm",
			"zypper": "nodejs npm", "brew": "node", "winget": "OpenJS.NodeJS.LTS", "choco": "nodejs-lts",
		},
		"docker": {
			"apt": "docker.io", "dnf": "docker", "yum": "docker", "pacman": "docker",
			"zypper": "docker", "brew": "--cask docker", "winget": "Docker.DockerDesktop", "choco": "docker-desktop",
		},
		"go": {
			"apt": "golang-go", "dnf": "golang", "yum": "golang", "pacman": "go",
			"zypper": "go", "brew": "go", "winget": "GoLang.Go", "choco": "golang",
		},
		"rust": {
			"apt": "rustc cargo", "dnf": "rust cargo", "yum": "rust cargo", "pacman": "rust",
			"zypper": "rust cargo", "brew": "rust", "winget": "Rustlang.Rustup", "choco": "rustup.install",
		},
		"git": {
			"apt": "git", "dnf": "git", "yum": "git", "pacman": "git", "zypper": "git",
			"brew": "git", "winget": "Git.Git", "choco": "git",
		},
		"make": {
			"apt": "make", "dnf": "make", "yum": "make", "pacman": "make", "zypper": "make",
			"brew": "make", "winget": "GnuWin32.Make", "choco": "make",
		},
		"kubectl": {
			"apt": "kubectl", "dnf": "kubectl", "yum": "kubectl", "pacman": "kubectl", "zypper": "kubectl",
			"brew": "kubectl", "winget": "Kubernetes.kubectl", "choco": "kubernetes-cli",
		},
		"php": {
			"apt": "php-cli", "dnf": "php-cli", "yum": "php-cli", "pacman": "php",
			"zypper": "php8", "brew": "php", "winget": "PHP.PHP", "choco": "php",
		},
		"composer": {
			"apt": "composer", "dnf": "composer", "yum": "composer", "pacman": "composer",
			"zypper": "composer", "brew": "composer", "winget": "Composer.Composer", "choco": "composer",
		},
		"java": {
			"apt": "default-jdk", "dnf": "java-latest-openjdk-devel", "yum": "java-17-openjdk-devel",
			"pacman": "jdk-openjdk", "zypper": "java-17-openjdk-devel", "brew": "openjdk",
			"winget": "EclipseAdoptium.Temurin.21.JDK", "choco": "temurin",
		},
		"maven": {
			"apt": "maven", "dnf": "maven", "yum": "maven", "pacman": "maven", "zypper": "maven",
			"brew": "maven", "winget": "Apache.Maven", "choco": "maven",
		},
		"gradle": {
			"apt": "gradle", "dnf": "gradle", "yum": "gradle", "pacman": "gradle", "zypper": "gradle",
			"brew": "gradle", "winget": "Gradle.Gradle", "choco": "gradle",
		},
		"dotnet": {
			"apt": "dotnet-sdk-8.0", "dnf": "dotnet-sdk-8.0", "yum": "dotnet-sdk-8.0", "pacman": "dotnet-sdk",
			"zypper": "dotnet-sdk-8.0", "brew": "dotnet-sdk", "winget": "Microsoft.DotNet.SDK.8", "choco": "dotnet-8.0-sdk",
		},
		"ruby": {
			"apt": "ruby ruby-bundler", "dnf": "ruby rubygem-bundler", "yum": "ruby rubygem-bundler",
			"pacman": "ruby", "zypper": "ruby ruby-devel", "brew": "ruby", "winget": "RubyInstallerTeam.Ruby.3.3", "choco": "ruby",
		},
		"dart": {
			"apt": "dart", "dnf": "dart", "yum": "dart", "pacman": "dart", "zypper": "dart",
			"brew": "dart-sdk", "winget": "Dart.Dart", "choco": "dart-sdk",
		},
		"swift": {
			"apt": "swiftlang", "dnf": "swift-lang", "yum": "swift-lang", "pacman": "swift-language",
			"zypper": "swift-lang", "brew": "swift", "winget": "Swift.Toolchain", "choco": "swift",
		},
		"elixir": {
			"apt": "elixir", "dnf": "elixir", "yum": "elixir", "pacman": "elixir", "zypper": "elixir",
			"brew": "elixir", "winget": "Elixir.Elixir", "choco": "elixir",
		},
		"lua": {
			"apt": "lua5.4 luarocks", "dnf": "lua luarocks", "yum": "lua luarocks", "pacman": "lua luarocks",
			"zypper": "lua54 luarocks", "brew": "lua luarocks", "winget": "DEVCOM.Lua", "choco": "lua",
		},
		"r": {
			"apt": "r-base", "dnf": "R", "yum": "R", "pacman": "r", "zypper": "R-base",
			"brew": "r", "winget": "RProject.R", "choco": "r.project",
		},
		"julia": {
			"apt": "julia", "dnf": "julia", "yum": "julia", "pacman": "julia", "zypper": "julia",
			"brew": "julia", "winget": "Julialang.Julia", "choco": "julia",
		},
		"haskell": {
			"apt": "ghc cabal-install", "dnf": "ghc cabal-install", "yum": "ghc cabal-install",
			"pacman": "ghc cabal-install", "zypper": "ghc cabal-install", "brew": "ghc cabal-install",
			"winget": "Haskell.Haskell", "choco": "haskell-dev",
		},
		"perl": {
			"apt": "perl cpanminus", "dnf": "perl-App-cpanminus", "yum": "perl-App-cpanminus",
			"pacman": "perl perl-app-cpanminus", "zypper": "perl-App-cpanminus", "brew": "perl",
			"winget": "StrawberryPerl.StrawberryPerl", "choco": "strawberryperl",
		},
		"conan": {
			"apt": "conan", "dnf": "conan", "yum": "conan", "pacman": "conan", "zypper": "conan",
			"brew": "conan", "winget": "Conan.Conan", "choco": "conan",
		},
		"vcpkg": {
			"apt": "vcpkg", "dnf": "vcpkg", "yum": "vcpkg", "pacman": "vcpkg", "zypper": "vcpkg",
			"brew": "vcpkg", "winget": "Microsoft.Vcpkg", "choco": "vcpkg",
		},
	}

	if byManager, ok := packages[tool]; ok {
		return byManager[manager]
	}
	return ""
}

func normalizeTool(tool string) string {
	tool = strings.ToLower(strings.TrimSpace(tool))
	switch tool {
	case "nodejs", "node.js":
		return "node"
	case "python3":
		return "python"
	case "golang":
		return "go"
	case "jdk", "jre", "openjdk":
		return "java"
	case "mvn":
		return "maven"
	case "gradlew":
		return "gradle"
	case "dotnet-sdk", ".net", "nuget":
		return "dotnet"
	case "bundler", "gem":
		return "ruby"
	case "flutter", "pub":
		return "dart"
	case "swiftpm":
		return "swift"
	case "mix":
		return "elixir"
	case "luarocks":
		return "lua"
	case "cran":
		return "r"
	case "ghc", "cabal", "stack":
		return "haskell"
	case "cpan", "cpanm":
		return "perl"
	}
	return tool
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
