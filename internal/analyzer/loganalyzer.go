package analyzer

import (
	"fmt"
	"os"
	"strings"
)

// LogIssue represents a detected issue with possible causes and fixes
type LogIssue struct {
	Keyword      string   `json:"keyword"`
	Description  string   `json:"description"`
	PossibleCauses []string `json:"possible_causes"`
	SuggestedFixes []string `json:"suggested_fixes"`
}

// LogAnalysisResult holds the analysis of a log file
type LogAnalysisResult struct {
	FilePath string     `json:"file_path"`
	Issues   []LogIssue `json:"issues"`
}

// Common log patterns mapped to issues and fixes
var logPatterns = []LogIssue{
	{
		Keyword:     "Cannot connect to Docker daemon",
		Description: "Docker daemon is not accessible",
		PossibleCauses: []string{
			"Docker daemon is not running",
			"User is not in docker group",
			"Docker socket permissions are incorrect",
		},
		SuggestedFixes: []string{
			"sudo systemctl start docker",
			"sudo usermod -aG docker $USER",
			"sudo chmod 666 /var/run/docker.sock",
		},
	},
	{
		Keyword:     "Permission denied",
		Description: "Insufficient permissions to access a resource",
		PossibleCauses: []string{
			"File or directory has restrictive permissions",
			"User does not have required privileges",
			"SELinux or AppArmor is blocking access",
		},
		SuggestedFixes: []string{
			"Check file permissions with ls -la",
			"Use sudo or adjust permissions with chmod/chown",
			"Check SELinux status with getenforce",
		},
	},
	{
		Keyword:     "Connection refused",
		Description: "Unable to establish a connection to a service",
		PossibleCauses: []string{
			"Service is not running",
			"Firewall is blocking the connection",
			"Wrong port or host configured",
		},
		SuggestedFixes: []string{
			"Check if the service is running with systemctl status <service>",
			"Check firewall rules with iptables -L or ufw status",
			"Verify the port and host settings",
		},
	},
	{
		Keyword:     "No such file or directory",
		Description: "A required file or directory is missing",
		PossibleCauses: []string{
			"File or directory does not exist",
			"Path is misspelled or incorrect",
			"Working directory is not what you expect",
		},
		SuggestedFixes: []string{
			"Verify the path with ls or pwd",
			"Check for typos in the file or directory name",
			"Ensure you are in the correct working directory",
		},
	},
	{
		Keyword:     "ModuleNotFoundError",
		Description: "Python module is not found",
		PossibleCauses: []string{
			"Module is not installed",
			"Virtual environment is not activated",
			"Python version is incorrect",
		},
		SuggestedFixes: []string{
			"Install the module with pip install <module>",
			"Activate the virtual environment",
			"Check Python version with python --version",
		},
	},
	{
		Keyword:     "npm ERR!",
		Description: "NPM error occurred during package operation",
		PossibleCauses: []string{
			"Package is not found in registry",
			"Network connectivity issue",
			"Corrupted node_modules or package-lock.json",
		},
		SuggestedFixes: []string{
			"Clear npm cache with npm cache clean --force",
			"Delete node_modules and package-lock.json, then run npm install",
			"Check network connectivity and registry settings",
		},
	},
	{
		Keyword:     "cargo build failed",
		Description: "Rust build failed",
		PossibleCauses: []string{
			"Missing system dependencies",
			"Compiler errors in source code",
			"Incompatible dependency versions",
		},
		SuggestedFixes: []string{
			"Install required system dependencies",
			"Fix compiler errors shown in the output",
			"Update dependencies with cargo update",
		},
	},
	{
		Keyword:     "go: cannot find main module",
		Description: "Go module is not initialized or missing",
		PossibleCauses: []string{
			"go.mod file is missing",
			"Working directory is outside the module",
			"Module path is incorrect",
		},
		SuggestedFixes: []string{
			"Initialize module with go mod init <module-path>",
			"Ensure you are inside the module directory",
			"Check the module path in go.mod",
		},
	},
	{
		Keyword:     "kubectl unable to connect",
		Description: "kubectl cannot connect to the Kubernetes cluster",
		PossibleCauses: []string{
			"Kubernetes cluster is not running",
			"kubeconfig is misconfigured",
			"Network issues between kubectl and cluster",
		},
		SuggestedFixes: []string{
			"Verify cluster status with kubectl cluster-info",
			"Check kubeconfig with kubectl config view",
			"Ensure network connectivity to the cluster",
		},
	},
	{
		Keyword:     "ssh: connect to host",
		Description: "SSH connection failed",
		PossibleCauses: []string{
			"Host is unreachable or down",
			"SSH service is not running on the host",
			"Firewall or security group is blocking port 22",
		},
		SuggestedFixes: []string{
			"Verify the host is reachable with ping or nc",
			"Check if SSH service is running on the host",
			"Ensure port 22 is open in firewall/security group",
		},
	},
}

// AnalyzeLog reads a log file and returns analysis
func AnalyzeLog(filePath string) (*LogAnalysisResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	logText := string(content)
	result := &LogAnalysisResult{
		FilePath: filePath,
		Issues:   []LogIssue{},
	}

	for _, pattern := range logPatterns {
		if strings.Contains(logText, pattern.Keyword) {
			result.Issues = append(result.Issues, pattern)
		}
	}

	return result, nil
}

// PrintAnalysis prints the log analysis result
func PrintAnalysis(result *LogAnalysisResult) {
	fmt.Printf("Log Analysis for: %s\n", result.FilePath)
	if len(result.Issues) == 0 {
		fmt.Println("No known issues found in the log.")
		return
	}

	fmt.Printf("Issues found (%d):\n\n", len(result.Issues))
	for i, issue := range result.Issues {
		fmt.Printf("--- Issue %d ---\n", i+1)
		fmt.Printf("Description: %s\n", issue.Description)
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
