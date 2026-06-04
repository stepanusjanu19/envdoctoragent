package analyzer

import (
	"fmt"
	"os"
	"strings"
)

// LogIssue represents a detected issue with possible causes and fixes.
type LogIssue struct {
	Keyword          string   `json:"keyword"`
	Category         string   `json:"category"`
	Severity         string   `json:"severity"`
	Confidence       float64  `json:"confidence"`
	Description      string   `json:"description"`
	MatchedLineCount int      `json:"matched_line_count"`
	LineNumbers      []int    `json:"line_numbers"`
	PossibleCauses   []string `json:"possible_causes"`
	SuggestedFixes   []string `json:"suggested_fixes"`
}

// LogAnalysisResult holds the analysis of a log file.
type LogAnalysisResult struct {
	FilePath string     `json:"file_path"`
	Issues   []LogIssue `json:"issues"`
}

type logPattern struct {
	Keyword        string
	Category       string
	Severity       string
	Keywords       []string
	Description    string
	PossibleCauses []string
	SuggestedFixes []string
}

var logPatterns = []logPattern{
	{
		Keyword:     "docker-daemon-unavailable",
		Category:    "Container",
		Severity:    "critical",
		Keywords:    []string{"cannot connect to docker daemon", "is the docker daemon running", "docker daemon"},
		Description: "Docker daemon is not accessible",
		PossibleCauses: []string{
			"Docker daemon is not running",
			"User cannot access the Docker socket",
			"Docker socket path or context is misconfigured",
		},
		SuggestedFixes: []string{
			"Start Docker Desktop or the Docker service",
			"Check docker context with docker context ls",
			"On Linux, verify docker group membership and socket permissions",
		},
	},
	{
		Keyword:     "permission-denied",
		Category:    "Permissions",
		Severity:    "critical",
		Keywords:    []string{"permission denied", "access denied", "operation not permitted"},
		Description: "Insufficient permissions to access a resource",
		PossibleCauses: []string{
			"File or directory permissions are restrictive",
			"User lacks required privileges",
			"Security policy is blocking access",
		},
		SuggestedFixes: []string{
			"Inspect ownership and permissions with ls -la",
			"Use a least-privilege permission change with chmod or chown",
			"Check platform security policy such as SELinux, AppArmor, or macOS privacy settings",
		},
	},
	{
		Keyword:     "connection-refused",
		Category:    "Network",
		Severity:    "warning",
		Keywords:    []string{"connection refused", "connect: refused", "econnrefused"},
		Description: "A local or remote service refused the connection",
		PossibleCauses: []string{
			"Service is not running",
			"Wrong host or port is configured",
			"Firewall or proxy is blocking access",
		},
		SuggestedFixes: []string{
			"Confirm service status and listening port",
			"Verify host and port configuration",
			"Check firewall, proxy, or security group rules",
		},
	},
	{
		Keyword:     "missing-file",
		Category:    "Filesystem",
		Severity:    "warning",
		Keywords:    []string{"no such file or directory", "file not found", "cannot find the file"},
		Description: "A required file or directory is missing",
		PossibleCauses: []string{
			"Path is incorrect or misspelled",
			"Working directory is not what the command expects",
			"Required generated file has not been created",
		},
		SuggestedFixes: []string{
			"Check the path with pwd and ls",
			"Run the command from the expected project directory",
			"Regenerate or restore the missing file",
		},
	},
	{
		Keyword:     "python-module-missing",
		Category:    "Python",
		Severity:    "warning",
		Keywords:    []string{"modulenotfounderror", "no module named", "importerror"},
		Description: "Python module is missing or cannot be imported",
		PossibleCauses: []string{
			"Package is not installed",
			"Virtual environment is not activated",
			"Python interpreter differs from the one used to install dependencies",
		},
		SuggestedFixes: []string{
			"Activate the expected virtual environment",
			"Install dependencies with pip install -r requirements.txt",
			"Check interpreter path with python -c 'import sys; print(sys.executable)'",
		},
	},
	{
		Keyword:     "python-environment",
		Category:    "Python",
		Severity:    "info",
		Keywords:    []string{"virtualenv", "venv", "pythonpath", "site-packages"},
		Description: "Python environment configuration may be involved",
		PossibleCauses: []string{
			"Virtual environment mismatch",
			"PYTHONPATH points at an unexpected location",
			"Global and project packages are mixed",
		},
		SuggestedFixes: []string{
			"Inspect active virtual environment variables",
			"Verify python and pip resolve to the same environment",
			"Recreate the virtual environment if package state is inconsistent",
		},
	},
	{
		Keyword:     "npm-install",
		Category:    "Node.js",
		Severity:    "warning",
		Keywords:    []string{"npm err!", "eresolve", "enoent", "package-lock.json", "node_modules"},
		Description: "NPM package operation failed",
		PossibleCauses: []string{
			"Dependency resolution conflict",
			"Corrupted node_modules or lockfile",
			"Package registry or network issue",
		},
		SuggestedFixes: []string{
			"Review npm error details above the failure",
			"Run npm install after verifying package.json and lockfile",
			"Check npm registry and proxy settings",
		},
	},
	{
		Keyword:     "go-module",
		Category:    "Go",
		Severity:    "warning",
		Keywords:    []string{"go: cannot find main module", "go.mod file not found", "module lookup disabled"},
		Description: "Go module configuration is missing or invalid",
		PossibleCauses: []string{
			"go.mod is missing",
			"Command is running outside the module directory",
			"Module cache or proxy configuration is invalid",
		},
		SuggestedFixes: []string{
			"Run the command from the module root",
			"Initialize a module with go mod init when appropriate",
			"Check GOPROXY and module cache configuration",
		},
	},
	{
		Keyword:     "rust-cargo",
		Category:    "Rust",
		Severity:    "warning",
		Keywords:    []string{"cargo build failed", "could not compile", "failed to select a version", "rustc"},
		Description: "Rust build or dependency resolution failed",
		PossibleCauses: []string{
			"Missing system dependency",
			"Compiler error in source code",
			"Incompatible crate versions",
		},
		SuggestedFixes: []string{
			"Read the first compiler error before later cascading errors",
			"Install required system libraries",
			"Run cargo update only after confirming version constraints",
		},
	},
	{
		Keyword:     "kubernetes-connectivity",
		Category:    "Kubernetes",
		Severity:    "warning",
		Keywords:    []string{"kubectl", "unable to connect", "the connection to the server", "kubeconfig"},
		Description: "kubectl cannot reach or authenticate to the cluster",
		PossibleCauses: []string{
			"Cluster is unavailable",
			"kubeconfig is missing or points at the wrong context",
			"Network or authentication configuration is invalid",
		},
		SuggestedFixes: []string{
			"Check current context with kubectl config current-context",
			"Verify cluster endpoint with kubectl cluster-info",
			"Refresh kubeconfig or credentials",
		},
	},
	{
		Keyword:     "ssh-connectivity",
		Category:    "SSH",
		Severity:    "warning",
		Keywords:    []string{"ssh: connect to host", "connection timed out", "permission denied (publickey)", "known_hosts"},
		Description: "SSH connection or authentication failed",
		PossibleCauses: []string{
			"Host is unreachable",
			"SSH service is not listening",
			"Key or known_hosts entry is invalid",
		},
		SuggestedFixes: []string{
			"Verify host reachability and port 22",
			"Check the selected private key and user",
			"Inspect known_hosts only after confirming the target host identity",
		},
	},
}

// AnalyzeLog reads a log file and returns analysis.
func AnalyzeLog(filePath string) (*LogAnalysisResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	result := &LogAnalysisResult{
		FilePath: filePath,
		Issues:   []LogIssue{},
	}

	for _, pattern := range logPatterns {
		lineNumbers := matchingLines(lines, pattern.Keywords)
		if len(lineNumbers) == 0 {
			continue
		}

		result.Issues = append(result.Issues, LogIssue{
			Keyword:          pattern.Keyword,
			Category:         pattern.Category,
			Severity:         pattern.Severity,
			Confidence:       confidenceForMatches(len(lineNumbers), len(pattern.Keywords)),
			Description:      pattern.Description,
			MatchedLineCount: len(lineNumbers),
			LineNumbers:      lineNumbers,
			PossibleCauses:   pattern.PossibleCauses,
			SuggestedFixes:   pattern.SuggestedFixes,
		})
	}

	return result, nil
}

func matchingLines(lines []string, keywords []string) []int {
	var lineNumbers []int
	seen := make(map[int]bool)
	for index, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, strings.ToLower(keyword)) {
				lineNumber := index + 1
				if !seen[lineNumber] {
					lineNumbers = append(lineNumbers, lineNumber)
					seen[lineNumber] = true
				}
			}
		}
	}
	return lineNumbers
}

func confidenceForMatches(matchCount, keywordCount int) float64 {
	if matchCount <= 0 {
		return 0
	}
	confidence := 0.60 + float64(matchCount)*0.10
	if keywordCount > 1 && matchCount > 1 {
		confidence += 0.10
	}
	if confidence > 0.95 {
		return 0.95
	}
	return confidence
}

// PrintAnalysis prints the log analysis result.
func PrintAnalysis(result *LogAnalysisResult) {
	fmt.Printf("Log Analysis for: %s\n", result.FilePath)
	if len(result.Issues) == 0 {
		fmt.Println("No known issues found in the log.")
		return
	}

	fmt.Printf("Issues found (%d):\n\n", len(result.Issues))
	for i, issue := range result.Issues {
		fmt.Printf("--- Issue %d ---\n", i+1)
		fmt.Printf("[%s] [%s] confidence %.2f\n", issue.Severity, issue.Category, issue.Confidence)
		fmt.Printf("Description: %s\n", issue.Description)
		fmt.Printf("Matched lines: %d (%s)\n", issue.MatchedLineCount, formatLineNumbers(issue.LineNumbers))
		fmt.Println("Possible Causes:")
		for _, cause := range issue.PossibleCauses {
			fmt.Printf("  - %s\n", cause)
		}
		fmt.Println("Suggested Fixes:")
		for _, fix := range issue.SuggestedFixes {
			fmt.Printf("  - %s\n", fix)
		}
		fmt.Println()
	}
}

func formatLineNumbers(lineNumbers []int) string {
	if len(lineNumbers) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(lineNumbers))
	limit := len(lineNumbers)
	if limit > 6 {
		limit = 6
	}
	for _, lineNumber := range lineNumbers[:limit] {
		parts = append(parts, fmt.Sprintf("%d", lineNumber))
	}
	if len(lineNumbers) > limit {
		parts = append(parts, "...")
	}
	return strings.Join(parts, ", ")
}
