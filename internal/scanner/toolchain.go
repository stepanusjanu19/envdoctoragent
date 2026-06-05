package scanner

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/stepanusjanu19/envdoctoragent/internal/command"
	"github.com/stepanusjanu19/envdoctoragent/internal/common"
)

// toolDef defines a tool and the CLI arguments used to fetch its version.
type toolDef struct {
	Name string
	Cmd  string
	Args []string
	// Some tools (python, go, rustc) write version to stderr.
	// We will use CombinedOutput to capture both stdout and stderr.
}

var tools = []toolDef{
	{Name: "Git", Cmd: "git", Args: []string{"--version"}},
	{Name: "Go", Cmd: "go", Args: []string{"version"}},
	{Name: "Node.js", Cmd: "node", Args: []string{"--version"}},
	{Name: "NPM", Cmd: "npm", Args: []string{"--version"}},
	{Name: "Yarn", Cmd: "yarn", Args: []string{"--version"}},
	{Name: "PNPM", Cmd: "pnpm", Args: []string{"--version"}},
	{Name: "Python", Cmd: "python", Args: []string{"--version"}},
	{Name: "Python3", Cmd: "python3", Args: []string{"--version"}},
	{Name: "Rust (rustc)", Cmd: "rustc", Args: []string{"--version"}},
	{Name: "Rust (cargo)", Cmd: "cargo", Args: []string{"--version"}},
	{Name: "Docker", Cmd: "docker", Args: []string{"--version"}},
	{Name: "Podman", Cmd: "podman", Args: []string{"--version"}},
	{Name: "kubectl", Cmd: "kubectl", Args: []string{"version", "--client"}},
	{Name: "GCC", Cmd: "gcc", Args: []string{"--version"}},
	{Name: "Clang", Cmd: "clang", Args: []string{"--version"}},
	{Name: "Make", Cmd: "make", Args: []string{"--version"}},
	{Name: "CMake", Cmd: "cmake", Args: []string{"--version"}},
	{Name: "UV", Cmd: "uv", Args: []string{"--version"}},
	{Name: "Pip", Cmd: "pip", Args: []string{"--version"}},
	{Name: "Pip3", Cmd: "pip3", Args: []string{"--version"}},
}

// ScanToolchain detects installed development tools.
func ScanToolchain() ([]common.ToolInfo, error) {
	var results []common.ToolInfo
	for _, t := range tools {
		results = append(results, detectTool(t))
	}
	return results, nil
}

func detectTool(t toolDef) common.ToolInfo {
	info := common.ToolInfo{
		Name:  t.Name,
		Found: false,
	}

	cmdPath, err := exec.LookPath(t.Cmd)
	if err != nil {
		return info
	}
	info.Found = true
	info.Path = cmdPath

	// Use CombinedOutput to capture both stdout and stderr.
	out, err := command.CombinedOutput(t.Cmd, t.Args...)
	if err != nil {
		// Even if the exit code is non-zero, we still have output we can parse.
		if len(out) == 0 {
			return info
		}
	}

	info.Version = cleanVersion(string(out))
	return info
}

func cleanVersion(s string) string {
	lines := strings.Split(s, "\n")
	// Find the first line that contains a number.
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if containsDigit(line) {
			return line
		}
	}
	return strings.TrimSpace(lines[0])
}

func containsDigit(s string) bool {
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			return true
		}
	}
	return false
}

// PrintToolchain prints detected tools in a table-like format
func PrintToolchain(tools []common.ToolInfo) {
	fmt.Printf("%-20s %-12s %s\n", "Tool", "Installed", "Version")
	fmt.Println(strings.Repeat("-", 60))
	for _, t := range tools {
		status := "no"
		ver := "-"
		if t.Found {
			status = "yes"
			ver = t.Version
		}
		fmt.Printf("%-20s %-12s %s\n", t.Name, status, ver)
	}
}
